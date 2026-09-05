#ifndef RMP_FILE_MANAGER_H
#define RMP_FILE_MANAGER_H
#include "rmp/frame.h"
#include "rmp/task_manager.h"
#include <chrono>
#include <functional>
namespace rmp {
// Session-owned FIFO and file I/O worker. Task identities/results are owned by
// the process TaskManager. This worker never reads the control socket.
class FileManager {
  public:
    typedef std::function<bool(std::uint8_t, std::uint16_t, const std::string &, std::uint64_t *)> Sender;
    FileManager(TaskManager &, Sender, std::function<void()>, std::uint32_t);
    ~FileManager();
    void Enqueue(const ExecTask &);
    void Enable(const std::string &);
    bool Feed(const Frame &, std::string *);
    void Tick();

  private:
    struct Slot {
        ExecTask task;
        bool enabled, has_begin;
        FileBegin begin;
        std::uint64_t begin_id;
        Slot() : enabled(false), has_begin(false), begin_id(0) {}
    };
    void Run();
    void WatchDeadline();
    void Check();
    Frame Next();
    bool Transfer(Slot &);
    bool Receive(Slot &);
    bool SendFile(Slot &);
    std::uint64_t Send(std::uint8_t, std::uint16_t, const std::string &);
    bool Ack(std::uint64_t, const std::string &, const FileParams &);
    void Complete(const ExecTask &, const std::string &, std::uint64_t, const std::string &, bool);
    TaskManager &tasks_;
    Sender sender_;
    std::function<void()> abort_;
    std::uint32_t chunk_size_;
    std::mutex mutex_;
    std::condition_variable condition_;
    std::atomic<bool> stop_, timedout_;
    std::map<std::string, Slot> slots_;
    std::deque<std::string> queue_;
    std::deque<Frame> frames_;
    std::string active_;
    std::chrono::steady_clock::time_point deadline_;
    std::thread worker_, timer_;
    std::uint64_t started_, last_message_;
    bool committed_;
    std::string result_payload_;
};
} // namespace rmp
#endif
