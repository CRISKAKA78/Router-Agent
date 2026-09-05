#include "rmp/file_manager.h"
#include "rmp/sha256.h"
#include <algorithm>
#include <cerrno>
#include <cstring>
#include <ctime>
#include <fcntl.h>
#include <iostream>
#include <stdexcept>
#include <sys/stat.h>
#include <unistd.h>

namespace rmp {
namespace {
std::string Text(const Frame &f) { return std::string(f.payload.begin(), f.payload.end()); }
void Need(bool b, const char *message) {
    if (!b)
        throw std::runtime_error(message);
}
struct LocalFile {
    int fd;
    std::string temporary;
    LocalFile() : fd(-1) {}
    ~LocalFile() {
        if (fd >= 0)
            close(fd);
        if (!temporary.empty())
            unlink(temporary.c_str());
    }
    void Source(const std::string &p) {
        std::lock_guard<std::mutex> lock(ExecForkMutex());
        fd = open(p.c_str(), O_RDONLY | O_NONBLOCK);
        Need(fd >= 0, "cannot open source");
        Need(fcntl(fd, F_SETFD, FD_CLOEXEC) == 0, "source cloexec failed");
        struct stat st;
        Need(fstat(fd, &st) == 0 && S_ISREG(st.st_mode), "source must be regular file");
    }
    void Target(const FileParams &p) {
        struct stat st;
        if (!p.overwrite) {
            int rc = lstat(p.remote_path.c_str(), &st);
            Need(rc != 0 && errno == ENOENT, "target exists or cannot be inspected");
        }
        std::string pattern =
            p.remote_path.substr(0, p.remote_path.find_last_of('/') + 1) + ".rmp-transfer-XXXXXX";
        std::vector<char> name(pattern.begin(), pattern.end());
        name.push_back(0);
        std::lock_guard<std::mutex> lock(ExecForkMutex());
        fd = mkstemp(&name[0]);
        Need(fd >= 0, "cannot create temporary file");
        temporary = &name[0];
        Need(fcntl(fd, F_SETFD, FD_CLOEXEC) == 0, "target cloexec failed");
    }
    void Write(const std::uint8_t *p, std::size_t n) {
        while (n) {
            ssize_t k = write(fd, p, n);
            if (k < 0 && errno == EINTR)
                continue;
            Need(k > 0, "file write failed");
            p += k;
            n -= static_cast<std::size_t>(k);
        }
    }
    void Publish(const FileParams &p) {
        mode_t mode = 0;
        for (std::size_t i = 0; i < p.mode.size(); i++)
            mode = static_cast<mode_t>(mode * 8 + p.mode[i] - '0');
        Need(fchmod(fd, mode) == 0, "chmod failed");
        int old = fd;
        fd = -1;
        Need(close(old) == 0, "file close failed");
        int rc = p.overwrite ? rename(temporary.c_str(), p.remote_path.c_str())
                             : link(temporary.c_str(), p.remote_path.c_str());
        Need(rc == 0, "file publication failed");
        // Publication succeeded; never roll back a complete target on unlink failure.
        if (unlink(temporary.c_str()) == 0 || p.overwrite)
            temporary.clear();
    }
};
ssize_t Read(int fd, char *b, std::size_t n) {
    ssize_t k;
    do {
        k = read(fd, b, n);
    } while (k < 0 && errno == EINTR);
    Need(k >= 0, "source read failed");
    return k;
}
} // namespace
FileManager::FileManager(TaskManager &t, Sender s, std::function<void()> abort, std::uint32_t chunk)
    : tasks_(t), sender_(s), abort_(abort), chunk_size_(chunk), stop_(false), timedout_(false), started_(0),
      last_message_(0), committed_(false) {
    worker_ = std::thread(&FileManager::Run, this);
    try {
        timer_ = std::thread(&FileManager::WatchDeadline, this);
    } catch (...) {
        stop_.store(true);
        condition_.notify_all();
        worker_.join();
        throw;
    }
}
FileManager::~FileManager() {
    stop_.store(true);
    abort_();
    condition_.notify_all();
    worker_.join();
    timer_.join();
}
void FileManager::Enqueue(const ExecTask &t) {
    std::lock_guard<std::mutex> lock(mutex_);
    Slot slot;
    slot.task = t;
    slots_[t.task_id] = slot;
    queue_.push_back(t.task_id);
    condition_.notify_all();
}
void FileManager::Enable(const std::string &id) {
    std::lock_guard<std::mutex> lock(mutex_);
    slots_.at(id).enabled = true;
    condition_.notify_all();
}
bool FileManager::Feed(const Frame &f, std::string *error) {
    try {
        const std::uint16_t flags = f.header.type == kTypeFileChunk
                                        ? kFlagBinary
                                        : (f.header.type == kTypeFileAck ? kFlagResponse : 0);
        Need(f.header.flags == flags, "invalid FILE flags");
        std::unique_lock<std::mutex> lock(mutex_);
        if (f.header.type == kTypeFileBegin) {
            FileBegin b;
            Need(ParseFileBegin(Text(f), chunk_size_, &b, error), "invalid FILE_BEGIN");
            std::map<std::string, Slot>::iterator i = slots_.find(b.task_id);
            Need(i != slots_.end() && i->second.task.type == "upload" && !i->second.has_begin,
                 "unexpected/duplicate FILE_BEGIN");
            Need(b.file.transfer_id == i->second.task.file.transfer_id && b.direction == "server_to_device",
                 "BEGIN identity mismatch");
            i->second.begin = b;
            i->second.begin_id = f.header.message_id;
            i->second.has_begin = true;
            condition_.notify_all();
            return true;
        }
        Need(!active_.empty(), "FILE without active transfer");
        // Bounded receive mailbox provides TCP backpressure; no filesystem calls run
        // on the reader. The independent writer still serves queued controls.
        condition_.wait(lock, [this] { return stop_.load() || frames_.size() < 16; });
        Need(!stop_.load(), "file session stopped");
        frames_.push_back(f);
        condition_.notify_all();
        return true;
    } catch (const std::exception &e) {
        *error = e.what();
        return false;
    }
}
void FileManager::WatchDeadline() {
    std::unique_lock<std::mutex> lock(mutex_);
    while (!stop_.load()) {
        condition_.wait(lock, [this] { return stop_.load() || !active_.empty(); });
        if (stop_.load())
            return;
        const std::chrono::steady_clock::time_point deadline = deadline_;
        if (!condition_.wait_until(lock, deadline, [this, deadline] {
                return stop_.load() || active_.empty() || deadline_ != deadline;
            })) {
            timedout_.store(true);
            stop_.store(true);
            lock.unlock();
            abort_();
            condition_.notify_all();
            return;
        }
    }
}
void FileManager::Tick() {
    std::lock_guard<std::mutex> lock(mutex_);
    if (!active_.empty() && std::chrono::steady_clock::now() >= deadline_) {
        timedout_.store(true);
        stop_.store(true);
        abort_();
        condition_.notify_all();
    }
}
void FileManager::Check() {
    if (timedout_.load())
        throw std::runtime_error("file task timeout");
    if (stop_.load())
        throw std::runtime_error("file connection interrupted");
    std::lock_guard<std::mutex> lock(mutex_);
    if (std::chrono::steady_clock::now() >= deadline_) {
        timedout_.store(true);
        throw std::runtime_error("file task timeout");
    }
}
Frame FileManager::Next() {
    std::unique_lock<std::mutex> lock(mutex_);
    if (!condition_.wait_until(lock, deadline_, [this] { return stop_.load() || !frames_.empty(); })) {
        timedout_.store(true);
        throw std::runtime_error("file task timeout");
    }
    Need(!stop_.load(), "file connection interrupted");
    Frame f = frames_.front();
    frames_.pop_front();
    last_message_ = f.header.message_id;
    condition_.notify_all();
    return f;
}
std::uint64_t FileManager::Send(std::uint8_t t, std::uint16_t f, const std::string &p) {
    std::uint64_t id = 0;
    Need(sender_(t, f, p, &id), "file transport write failed");
    return id;
}
bool FileManager::Ack(std::uint64_t id, const std::string &phase, const FileParams &p) {
    Frame f = Next();
    Need(f.header.type == kTypeFileAck, "expected FILE_ACK");
    std::string e;
    bool accepted = false;
    if (!CheckFileAck(Text(f), id, p.transfer_id, phase, p.size, &accepted, &e))
        throw std::runtime_error(e);
    return accepted;
}
void FileManager::Complete(const ExecTask &t, const std::string &status, std::uint64_t start,
                           const std::string &message, bool send) {
    std::uint64_t end = static_cast<std::uint64_t>(time(NULL));
    if (!start)
        start = end;
    if (end < start)
        end = start;
    if (!committed_) {
        result_payload_ = "{\"task_id\":" + EscapeJsonString(t.task_id) + ",\"status\":\"" + status +
                          "\",\"started_at\":" + std::to_string(start) +
                          ",\"finished_at\":" + std::to_string(end) +
                          ",\"exit_code\":" + (status == "success" ? "0" : "-1") +
                          ",\"stdout\":\"\",\"stderr\":" + EscapeJsonString(message.substr(0, 128)) +
                          ",\"truncated\":false,\"result\":{";
        result_payload_ += "\"transfer_id\":" + EscapeJsonString(t.file.transfer_id);
        if (status == "success")
            result_payload_ +=
                ",\"size\":" + std::to_string(t.file.size) + ",\"sha256\":" + EscapeJsonString(t.file.sha256);
        result_payload_ += "}}";
        tasks_.FileComplete(t.task_id, status, result_payload_);
        committed_ = true;
    }
    if (send && !stop_.load())
        Send(kTypeTaskResult, 0, result_payload_);
}
bool FileManager::Receive(Slot &slot) {
    {
        std::unique_lock<std::mutex> lock(mutex_);
        if (!condition_.wait_until(lock, deadline_, [this, &slot] {
                return stop_.load() || slots_.at(slot.task.task_id).has_begin;
            })) {
            timedout_.store(true);
            throw std::runtime_error("FILE_BEGIN timeout");
        }
        Need(!stop_.load(), "file connection interrupted");
        Slot &saved = slots_.at(slot.task.task_id);
        slot.begin = saved.begin;
        slot.begin_id = saved.begin_id;
    }
    last_message_ = slot.begin_id;
    const FileParams &p = slot.task.file;
    LocalFile dst;
    try {
        Need(slot.begin.file == p, "BEGIN metadata conflict");
        Check();
        dst.Target(p);
    } catch (const std::exception &) {
        if (timedout_.load() || stop_.load())
            throw;
        Send(kTypeFileAck, kFlagResponse, FileAckPayload(slot.begin_id, p.transfer_id, "failed", 0));
        return false;
    }
    Send(kTypeFileAck, kFlagResponse, FileAckPayload(slot.begin_id, p.transfer_id, "ready", 0));
    Sha256 hash;
    std::uint64_t received = 0;
    while (true) {
        Check();
        Frame f = Next();
        if (f.header.type == kTypeFileChunk) {
            std::string e;
            if (!CheckFileChunk(f.payload, p, received, slot.begin.chunk_size, &e))
                throw std::runtime_error(e);
            dst.Write(&f.payload[28], f.payload.size() - 28);
            hash.Update(&f.payload[28], f.payload.size() - 28);
            received += f.payload.size() - 28;
            continue;
        }
        Need(f.header.type == kTypeFileEnd, "expected FILE_END");
        JsonObject end;
        std::string e;
        Need(ParseJsonObject(Text(f), &end, &e), "invalid END JSON");
        bool valid = FileString(end, "transfer_id") == p.transfer_id && FileUInt(end, "size") == p.size &&
                     FileString(end, "sha256") == p.sha256 && received == p.size && hash.Finish() == p.sha256;
        if (valid) {
            try {
                Check();
                dst.Publish(p);
            } catch (const std::exception &) {
                if (timedout_.load() || stop_.load())
                    throw;
                valid = false;
            }
        }
        if (valid)
            Complete(slot.task, "success", started_, "",
                     false); // irrevocable local commit before any network notification
        Send(kTypeFileAck, kFlagResponse,
             FileAckPayload(f.header.message_id, p.transfer_id, valid ? "done" : "failed", received));
        return valid;
    }
}
bool FileManager::SendFile(Slot &slot) {
    LocalFile src;
    src.Source(slot.task.file.remote_path);
    std::vector<char> buf(chunk_size_);
    Sha256 pre;
    std::uint64_t size = 0;
    while (true) {
        Check();
        ssize_t n = Read(src.fd, &buf[0], buf.size());
        if (!n)
            break;
        pre.Update(&buf[0], static_cast<std::size_t>(n));
        Need(size <= std::uint64_t(INT64_MAX) - n, "source too large");
        size += n;
    }
    slot.task.file.size = size;
    slot.task.file.sha256 = pre.Finish();
    Need(lseek(src.fd, 0, SEEK_SET) == 0, "source seek failed");
    FileBegin b;
    b.file = slot.task.file;
    b.task_id = slot.task.task_id;
    b.direction = "device_to_server";
    b.name = slot.task.file.result_name;
    b.chunk_size = chunk_size_;
    Check();
    std::uint64_t id = Send(kTypeFileBegin, 0, FileBeginPayload(b));
    if (!Ack(id, "ready", b.file))
        return false;
    last_message_ = 0;
    Sha256 sent;
    std::uint64_t off = 0;
    while (true) {
        Check();
        ssize_t n = Read(src.fd, &buf[0], buf.size());
        if (!n)
            break;
        Need(off <= size && std::uint64_t(n) <= size - off, "source size changed");
        sent.Update(&buf[0], static_cast<std::size_t>(n));
        Send(kTypeFileChunk, kFlagBinary,
             FileChunkPayload(b.file.transfer_id, off, &buf[0], static_cast<std::size_t>(n)));
        off += n;
    }
    Need(off == size && sent.Finish() == b.file.sha256, "source checksum changed");
    Check();
    id = Send(kTypeFileEnd, 0, FileEndPayload(b.file));
    return Ack(id, "done", b.file);
}
bool FileManager::Transfer(Slot &slot) { return slot.task.type == "upload" ? Receive(slot) : SendFile(slot); }
void FileManager::Run() {
    while (true) {
        Slot slot;
        {
            std::unique_lock<std::mutex> lock(mutex_);
            condition_.wait(lock, [this] {
                return stop_.load() || (!queue_.empty() && slots_.at(queue_.front()).enabled);
            });
            if (stop_.load())
                break;
            active_ = queue_.front();
            queue_.pop_front();
            slot = slots_.at(active_);
            deadline_ = std::chrono::steady_clock::now() + std::chrono::seconds(slot.task.timeout);
            started_ = static_cast<std::uint64_t>(time(NULL));
            committed_ = false;
            timedout_.store(false);
            tasks_.FileRunning(active_);
            condition_.notify_all();
        }
        last_message_ = 0;
        std::cout << "file_active task_id=" << slot.task.task_id
                  << " transfer_id=" << slot.task.file.transfer_id << std::endl;
        try {
            bool success = Transfer(slot);
            Complete(slot.task, success ? "success" : "failed", started_,
                     success ? "" : "file rejected or verification failed", true);
        } catch (const std::exception &e) {
            try {
                Complete(slot.task, timedout_.load() ? "timeout" : "failed", started_, e.what(), false);
            } catch (...) {
                std::terminate();
            }
            if (!stop_.load()) {
                std::string failure = "{\"code\":\"TRANSFER_ERROR\",\"message\":\"file transfer failed\"";
                if (last_message_)
                    failure += ",\"reply_to\":" + std::to_string(last_message_);
                failure += "}";
                std::uint64_t ignored = 0;
                sender_(kTypeError, last_message_ ? kFlagResponse : 0, failure, &ignored);
            }
            stop_.store(true);
            abort_();
            condition_.notify_all();
        }
        {
            std::lock_guard<std::mutex> lock(mutex_);
            slots_.erase(active_);
            active_.clear();
            condition_.notify_all();
        }
    }
    std::lock_guard<std::mutex> lock(mutex_);
    for (std::map<std::string, Slot>::const_iterator i = slots_.begin(); i != slots_.end(); ++i) {
        committed_ = false;
        Complete(i->second.task, "failed", 0, "file connection interrupted", false);
    }
    slots_.clear();
    queue_.clear();
}
} // namespace rmp
