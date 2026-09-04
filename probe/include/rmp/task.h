#ifndef RMP_TASK_H
#define RMP_TASK_H

#include <atomic>
#include <cstddef>
#include <cstdint>
#include <map>
#include <mutex>
#include <string>

namespace rmp {

// Serializes fd creation + FD_CLOEXEC with fork on older Linux/libc targets.
// The fork child never unlocks this mutex: it only performs execve or _exit.
std::mutex& ExecForkMutex();

static const std::size_t kMaxTaskOutput = 1024U * 1024U;

enum class TaskState {
    kReceived,
    kQueued,
    kRunning,
    kSuccess,
    kFailed,
    kTimeout
};

struct ExecTask {
    std::string task_id;
    std::string type;
    std::uint32_t timeout;
    std::string command;
    std::string cwd;
    std::map<std::string, std::string> env;
    TaskState state;

    ExecTask() : timeout(0), state(TaskState::kReceived) {}
};

struct ExecResult {
    std::string task_id;
    std::string status;
    std::uint64_t started_at;
    std::uint64_t finished_at;
    int exit_code;
    std::string stdout_text;
    std::string stderr_text;
    bool truncated;

    ExecResult()
        : started_at(0), finished_at(0), exit_code(-1), truncated(false) {}
};

bool ParseTask(const std::string& input, ExecTask* task, std::string* error);

std::string TaskAckPayload(std::uint64_t reply_to,
                           const std::string& task_id,
                           bool accepted,
                           const std::string& reason);

ExecResult ExecuteExec(const ExecTask& task, const std::atomic<bool>* stop_requested);

bool BuildTaskResultPayload(ExecResult result,
                            std::uint32_t max_payload,
                            std::string* payload,
                            std::string* error);

}  // namespace rmp

#endif
