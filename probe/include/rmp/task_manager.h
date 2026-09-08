#ifndef RMP_TASK_MANAGER_H
#define RMP_TASK_MANAGER_H

#include "rmp/task.h"
#include <condition_variable>
#include <deque>
#include <thread>
#include <vector>

namespace rmp {

// Process lifetime registry. Only the connection thread sends frames; workers
// never hold or access a socket. Accepted identities and results are not evicted.
class TaskManager {
public:
    TaskManager(unsigned workers = 4, std::size_t capacity = 128,
                std::size_t byte_capacity = 8U * 1024U * 1024U, std::size_t file_capacity = 8);
    ~TaskManager();
    // Returns queued/running/terminal, rejected, or conflict. Admission occurs
    // before ACK transmission, so losing an ACK never loses an accepted task.
    std::string Submit(const ExecTask& task, bool valid, std::size_t input_size,
                       std::uint32_t max_payload, bool* fresh = NULL);
    void FileRunning(const std::string& id);
    void FileComplete(const std::string& id,const std::string& status,const std::string& payload);
    void BeginSession();
    bool NextResult(std::uint32_t max_payload, std::string* payload);
    bool CachedResult(const std::string& id, std::uint32_t max_payload,
                      std::string* payload);
    unsigned RunningTasks() const;

private:
    struct Entry {
        ExecTask task;
        std::string state;
        std::string payload;
        std::uint32_t max_payload;
        bool sent;
        Entry() : max_payload(0), sent(false) {}
    };
    void Run();
    bool Runnable() const;
    bool config_running_;
    std::atomic<bool> stop_;
    std::atomic<unsigned> running_;
    std::mutex mutex_;
    std::condition_variable condition_;
    std::map<std::string, Entry> entries_;
    std::deque<std::string> queue_;
    std::vector<std::thread> workers_;
    std::size_t file_capacity_, file_count_;
    std::size_t capacity_;
    std::size_t byte_capacity_;
    std::size_t reserved_bytes_;
};
}
#endif
