#include "rmp/task_manager.h"
#include <iostream>
#include <stdexcept>

namespace rmp {
namespace {
bool SameTask(const ExecTask& a, const ExecTask& b) {
    return a.type == b.type && a.timeout == b.timeout && a.command == b.command &&
           a.cwd == b.cwd && a.env == b.env;
}
}

TaskManager::TaskManager(unsigned workers, std::size_t capacity, std::size_t byte_capacity)
    : stop_(false), running_(0), capacity_(capacity), byte_capacity_(byte_capacity),
      reserved_bytes_(0) {
    if (workers == 0 || workers > 64 || capacity == 0) {
        throw std::invalid_argument("invalid task manager limits");
    }
    try {
        for (unsigned i = 0; i < workers; ++i) workers_.push_back(std::thread(&TaskManager::Run, this));
    } catch (...) {
        stop_.store(true);
        condition_.notify_all();
        for (std::size_t i = 0; i < workers_.size(); ++i) workers_[i].join();
        throw;
    }
}

TaskManager::~TaskManager() {
    stop_.store(true);
    condition_.notify_all();
    for (std::size_t i = 0; i < workers_.size(); ++i) workers_[i].join();
}

std::string TaskManager::Submit(const ExecTask& task, bool valid,
                                std::size_t input_size, std::uint32_t max_payload) {
    std::lock_guard<std::mutex> lock(mutex_);
    std::map<std::string, Entry>::iterator existing = entries_.find(task.task_id);
    if (existing != entries_.end()) {
        if (!valid || !SameTask(existing->second.task, task)) return "conflict";
        return existing->second.state;
    }
    // Reserve the maximum encoded result at admission, including while offline.
    // Two input copies worth of room cover decoded strings and identity storage.
    const std::size_t reservation = input_size * 2 + max_payload;
    if (!valid || task.type != "exec" || entries_.size() >= capacity_ ||
        reservation > byte_capacity_ - reserved_bytes_) return "rejected";
    Entry entry;
    entry.task = task;
    entry.state = "queued";
    entry.max_payload = max_payload;
    entries_.insert(std::make_pair(task.task_id, entry));
    reserved_bytes_ += reservation;
    queue_.push_back(task.task_id);
    condition_.notify_one();
    return "queued";
}

void TaskManager::BeginSession() {
    std::lock_guard<std::mutex> lock(mutex_);
    for (std::map<std::string, Entry>::iterator it = entries_.begin(); it != entries_.end(); ++it)
        it->second.sent = false;
}

bool TaskManager::NextResult(std::uint32_t max_payload, std::string* payload) {
    std::lock_guard<std::mutex> lock(mutex_);
    for (std::map<std::string, Entry>::iterator it = entries_.begin(); it != entries_.end(); ++it) {
        Entry& entry = it->second;
        if (!entry.sent && !entry.payload.empty() && entry.payload.size() <= max_payload) {
            *payload = entry.payload;
            entry.sent = true; // Any write failure ends this session; BeginSession replays it.
            return true;
        }
    }
    return false;
}

bool TaskManager::CachedResult(const std::string& id, std::uint32_t max_payload,
                               std::string* payload) {
    std::lock_guard<std::mutex> lock(mutex_);
    Entry& entry = entries_.at(id);
    if (entry.payload.empty() || entry.payload.size() > max_payload) return false;
    *payload = entry.payload;
    entry.sent = true;
    return true;
}

unsigned TaskManager::RunningTasks() const { return running_.load(); }

void TaskManager::Run() {
    while (true) {
        ExecTask task;
        std::uint32_t max_payload;
        {
            std::unique_lock<std::mutex> lock(mutex_);
            condition_.wait(lock, [this] { return stop_.load() || !queue_.empty(); });
            if (stop_.load()) return;
            Entry& entry = entries_.at(queue_.front());
            queue_.pop_front();
            entry.state = "running";
            task = entry.task;
            max_payload = entry.max_payload;
            running_.fetch_add(1);
            std::cout << "task_state=RUNNING task_id=" << task.task_id << std::endl;
        }
        ExecResult result = ExecuteExec(task, &stop_);
        std::string payload, error;
        // Valid task IDs and >=1024-byte negotiation guarantee metadata fits.
        if (!BuildTaskResultPayload(result, max_payload, &payload, &error)) {
            result.stdout_text.clear();
            result.stderr_text.clear();
            result.truncated = true;
            BuildTaskResultPayload(result, max_payload, &payload, &error);
        }
        {
            std::lock_guard<std::mutex> lock(mutex_);
            Entry& entry = entries_.at(task.task_id);
            entry.state = result.status;
            reserved_bytes_ -= entry.max_payload - payload.size();
            entry.payload.swap(payload);
            running_.fetch_sub(1);
            std::cout << "task_state=" << result.status << " task_id=" << task.task_id << std::endl;
        }
    }
}
}
