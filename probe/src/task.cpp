#include "rmp/task.h"

#include "rmp/json.h"

#include <cerrno>
#include <chrono>
#include <csignal>
#include <cstring>
#include <ctime>
#include <fcntl.h>
#include <limits>
#include <poll.h>
#include <sstream>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
#include <vector>

extern char** environ;

namespace rmp {
namespace {

typedef std::chrono::steady_clock SteadyClock;

bool Required(const JsonObject& object,
              const std::string& name,
              JsonType type,
              const JsonValue** value,
              std::string* error) {
    const JsonObject::const_iterator found = object.find(name);
    if (found == object.end()) {
        if (error != NULL) {
            *error = name + " is required";
        }
        return false;
    }
    if (found->second.type != type) {
        if (error != NULL) {
            *error = name + " has the wrong type";
        }
        return false;
    }
    *value = &found->second;
    return true;
}

bool Optional(const JsonObject& object,
              const std::string& name,
              JsonType type,
              const JsonValue** value,
              std::string* error) {
    const JsonObject::const_iterator found = object.find(name);
    if (found == object.end()) {
        *value = NULL;
        return true;
    }
    if (found->second.type != type) {
        if (error != NULL) {
            *error = name + " has the wrong type";
        }
        return false;
    }
    *value = &found->second;
    return true;
}

bool IsObject(const JsonValue& value) {
    return value.type == JsonType::kOther && !value.raw_value.empty() && value.raw_value[0] == '{';
}

bool ConfigurePipe(int descriptors[2], std::string* error) {
    if (pipe(descriptors) != 0) {
        *error = std::strerror(errno);
        return false;
    }
    for (int index = 0; index < 2; ++index) {
        const int descriptor_flags = fcntl(descriptors[index], F_GETFD, 0);
        if (descriptor_flags < 0 || fcntl(descriptors[index], F_SETFD, descriptor_flags | FD_CLOEXEC) != 0) {
            *error = std::strerror(errno);
            close(descriptors[0]);
            close(descriptors[1]);
            return false;
        }
    }
    return true;
}

bool SetNonBlocking(int descriptor, std::string* error) {
    const int flags = fcntl(descriptor, F_GETFL, 0);
    if (flags < 0 || fcntl(descriptor, F_SETFL, flags | O_NONBLOCK) != 0) {
        *error = std::strerror(errno);
        return false;
    }
    return true;
}

void AppendLimited(const char* data,
                   std::size_t size,
                   std::string* output,
                   bool* truncated) {
    const std::size_t available = output->size() < kMaxTaskOutput
                                      ? kMaxTaskOutput - output->size()
                                      : 0;
    const std::size_t append = size < available ? size : available;
    output->append(data, append);
    if (append < size) {
        *truncated = true;
    }
}

void DrainDescriptor(int* descriptor,
                     std::string* output,
                     bool* truncated,
                     bool* read_failed) {
    if (*descriptor < 0) {
        return;
    }
    char buffer[16384];
    ssize_t count;
    do {
        count = read(*descriptor, buffer, sizeof(buffer));
    } while (count < 0 && errno == EINTR);
    if (count > 0) {
        AppendLimited(buffer, static_cast<std::size_t>(count), output, truncated);
        return;
    }
    if (count == 0) {
        close(*descriptor);
        *descriptor = -1;
        return;
    }
    if (errno != EAGAIN && errno != EWOULDBLOCK) {
        *read_failed = true;
        close(*descriptor);
        *descriptor = -1;
    }
}

void SignalProcessGroup(pid_t child, int signal_number) {
    if (kill(-child, signal_number) != 0) {
        kill(child, signal_number);
    }
}

std::string SanitizeUtf8(const std::string& input) {
    std::string output;
    output.reserve(input.size());
    std::size_t index = 0;
    while (index < input.size()) {
        const unsigned char first = static_cast<unsigned char>(input[index]);
        std::size_t length = 0;
        if (first <= 0x7F) {
            length = 1;
        } else if (first >= 0xC2 && first <= 0xDF) {
            length = 2;
        } else if (first >= 0xE0 && first <= 0xEF) {
            length = 3;
        } else if (first >= 0xF0 && first <= 0xF4) {
            length = 4;
        }
        bool valid = length != 0 && index + length <= input.size();
        for (std::size_t offset = 1; valid && offset < length; ++offset) {
            const unsigned char continuation = static_cast<unsigned char>(input[index + offset]);
            valid = (continuation & 0xC0U) == 0x80U;
        }
        if (valid && length == 3) {
            const unsigned char second = static_cast<unsigned char>(input[index + 1]);
            valid = !((first == 0xE0 && second < 0xA0) || (first == 0xED && second >= 0xA0));
        }
        if (valid && length == 4) {
            const unsigned char second = static_cast<unsigned char>(input[index + 1]);
            valid = !((first == 0xF0 && second < 0x90) || (first == 0xF4 && second >= 0x90));
        }
        if (valid) {
            output.append(input, index, length);
            index += length;
        } else {
            output.append("\xEF\xBF\xBD", 3);
            ++index;
        }
    }
    return output;
}

std::string SerializeResult(const ExecResult& result) {
    std::ostringstream output;
    output << "{\"task_id\":" << EscapeJsonString(result.task_id)
           << ",\"status\":" << EscapeJsonString(result.status)
           << ",\"started_at\":" << result.started_at
           << ",\"finished_at\":" << result.finished_at
           << ",\"exit_code\":" << result.exit_code
           << ",\"stdout\":" << EscapeJsonString(result.stdout_text)
           << ",\"stderr\":" << EscapeJsonString(result.stderr_text)
           << ",\"truncated\":" << (result.truncated ? "true" : "false")
           << ",\"result\":{}}";
    return output.str();
}

std::size_t Utf8PrefixLength(const std::string& input, std::size_t wanted) {
    if (wanted >= input.size()) {
        return input.size();
    }
    if (wanted == 0) {
        return 0;
    }
    std::size_t start = wanted - 1;
    while (start > 0 &&
           (static_cast<unsigned char>(input[start]) & 0xC0U) == 0x80U) {
        --start;
    }
    const unsigned char first = static_cast<unsigned char>(input[start]);
    std::size_t length = 1;
    if (first >= 0xC2 && first <= 0xDF) {
        length = 2;
    } else if (first >= 0xE0 && first <= 0xEF) {
        length = 3;
    } else if (first >= 0xF0 && first <= 0xF4) {
        length = 4;
    }
    return start + length <= wanted ? wanted : start;
}

}  // namespace

std::mutex& ExecForkMutex() {
    static std::mutex mutex;
    return mutex;
}

bool ParseTask(const std::string& input, ExecTask* task, std::string* error) {
    *task = ExecTask();
    JsonObject object;
    if (!ParseJsonObject(input, &object, error)) {
        return false;
    }
    const JsonValue* value = NULL;
    if (!Required(object, "task_id", JsonType::kString, &value, error)) {
        return false;
    }
    task->task_id = value->string_value;
    if (task->task_id.empty() || task->task_id.size() > 128) {
        *error = "task_id must be 1-128 bytes";
        return false;
    }
    if (!Required(object, "type", JsonType::kString, &value, error)) {
        return false;
    }
    task->type = value->string_value;
    if (task->type.empty()) {
        *error = "type must not be empty";
        return false;
    }
    if (!Optional(object, "created_at", JsonType::kUnsignedInteger, &value, error)) {
        return false;
    }
    if (!Required(object, "timeout", JsonType::kUnsignedInteger, &value, error) ||
        value->unsigned_value > std::numeric_limits<std::uint32_t>::max()) {
        if (value != NULL && value->type == JsonType::kUnsignedInteger) {
            *error = "timeout is outside uint32 range";
        }
        return false;
    }
    task->timeout = static_cast<std::uint32_t>(value->unsigned_value);
    if (!Required(object, "params", JsonType::kOther, &value, error) || !IsObject(*value)) {
        if (value != NULL && !IsObject(*value)) {
            *error = "params must be an object";
        }
        return false;
    }
    if (task->type == "upload" || task->type == "download") {
        if (task->timeout == 0) { *error="file timeout must be positive"; return false; }
        return ParseFileParams(value->raw_value,task->type,&task->file,error);
    }
    if (task->type == "router_config") {
        if (task->timeout < 1 || task->timeout > 30) { *error="configuration timeout must be 1-30 seconds"; return false; }
        return ParseRouterConfig(value->raw_value, &task->config, error);
    }
    if (task->type != "exec") { return true; }

    JsonObject params;
    if (!ParseJsonObject(value->raw_value, &params, error)) {
        return false;
    }
    if (!Required(params, "command", JsonType::kString, &value, error)) {
        return false;
    }
    task->command = value->string_value;
    if (task->command.find('\0') != std::string::npos) {
        *error = "command must not contain NUL";
        return false;
    }
    if (!Optional(params, "cwd", JsonType::kString, &value, error)) {
        return false;
    }
    if (value != NULL) {
        task->cwd = value->string_value;
        if (task->cwd.find('\0') != std::string::npos) {
            *error = "cwd must not contain NUL";
            return false;
        }
    }
    if (!Optional(params, "env", JsonType::kOther, &value, error)) {
        return false;
    }
    if (value != NULL) {
        if (!IsObject(*value)) {
            *error = "env must be an object";
            return false;
        }
        JsonObject environment;
        if (!ParseJsonObject(value->raw_value, &environment, error)) {
            return false;
        }
        for (JsonObject::const_iterator item = environment.begin(); item != environment.end(); ++item) {
            if (item->first.empty() || item->first.find('=') != std::string::npos ||
                item->first.find('\0') != std::string::npos || item->second.type != JsonType::kString ||
                item->second.string_value.find('\0') != std::string::npos) {
                *error = "env names must be non-empty without '=' or NUL, and values must be NUL-free strings";
                return false;
            }
            task->env[item->first] = item->second.string_value;
        }
    }
    return true;
}

std::string TaskAckPayload(std::uint64_t reply_to,
                           const std::string& task_id,
                           bool accepted,
                           const std::string& reason,
                           const std::string& state) {
    std::ostringstream output;
    output << "{\"reply_to\":" << reply_to
           << ",\"task_id\":" << EscapeJsonString(task_id)
           << ",\"accepted\":" << (accepted ? "true" : "false")
           << ",\"state\":\"" << (state.empty() ? (accepted ? "queued" : "rejected") : state) << '"';
    if (!reason.empty()) {
        output << ",\"message\":" << EscapeJsonString(reason);
    }
    output << '}';
    return output.str();
}

ExecResult ExecuteExec(const ExecTask& task, const std::atomic<bool>* stop_requested) {
    ExecResult result;
    result.task_id = task.task_id;
    result.started_at = static_cast<std::uint64_t>(std::time(NULL));

    std::map<std::string, std::string> environment;
    for (char** current = environ; current != NULL && *current != NULL; ++current) {
        const std::string entry(*current);
        const std::size_t separator = entry.find('=');
        if (separator != std::string::npos) {
            environment[entry.substr(0, separator)] = entry.substr(separator + 1);
        }
    }
    for (std::map<std::string, std::string>::const_iterator item = task.env.begin();
         item != task.env.end(); ++item) {
        environment[item->first] = item->second;
    }
    std::vector<std::string> environment_storage;
    environment_storage.reserve(environment.size());
    for (std::map<std::string, std::string>::const_iterator item = environment.begin();
         item != environment.end(); ++item) {
        environment_storage.push_back(item->first + "=" + item->second);
    }
    std::vector<char*> environment_pointers;
    environment_pointers.reserve(environment_storage.size() + 1);
    for (std::vector<std::string>::iterator item = environment_storage.begin();
         item != environment_storage.end(); ++item) {
        environment_pointers.push_back(const_cast<char*>(item->c_str()));
    }
    environment_pointers.push_back(NULL);

    // Prepare argv and PATH candidates before fork: the multithreaded child
    // performs only async-signal-safe operations and never invokes a shell.
    std::vector<std::string> arguments_storage, executable_paths;
    if (task.type == "router_config") {
        std::string error;
        if (!RouterConfigArguments(task.config, &arguments_storage, &error)) {
            result.status="failed"; result.stderr_text=error;
            result.finished_at=static_cast<std::uint64_t>(std::time(NULL)); return result;
        }
        const std::string path=environment.count("PATH")?environment.at("PATH"):"/usr/sbin:/usr/bin:/sbin:/bin";
        std::size_t start=0;
        do {
            const std::size_t end=path.find(':',start);
            const std::string dir=path.substr(start,end==std::string::npos?end:end-start);
            executable_paths.push_back((dir.empty()?".":dir)+"/"+arguments_storage[0]);
            if(end==std::string::npos) break;
            start=end+1;
        } while(true);
    } else {
        arguments_storage.push_back("sh"); arguments_storage.push_back("-c"); arguments_storage.push_back(task.command);
        executable_paths.push_back("/bin/sh");
    }
    std::vector<char*> argument_pointers;
    for(std::size_t i=0;i<arguments_storage.size();++i) argument_pointers.push_back(const_cast<char*>(arguments_storage[i].c_str()));
    argument_pointers.push_back(NULL);

    int stdout_pipe[2] = {-1, -1};
    int stderr_pipe[2] = {-1, -1};
    std::string setup_error;
    // Hold through both pipe()+fcntl() pairs and fork, so another exec cannot
    // inherit a descriptor in the interval before FD_CLOEXEC is installed.
    std::unique_lock<std::mutex> fork_lock(ExecForkMutex());
    if (!ConfigurePipe(stdout_pipe, &setup_error)) {
        result.status = "failed";
        result.stderr_text = "pipe stdout: " + setup_error;
        result.finished_at = static_cast<std::uint64_t>(std::time(NULL));
        return result;
    }
    if (!ConfigurePipe(stderr_pipe, &setup_error)) {
        close(stdout_pipe[0]);
        close(stdout_pipe[1]);
        result.status = "failed";
        result.stderr_text = "pipe stderr: " + setup_error;
        result.finished_at = static_cast<std::uint64_t>(std::time(NULL));
        return result;
    }

    const pid_t child = fork();
    if (child != 0) {
        fork_lock.unlock();
    }
    if (child < 0) {
        setup_error = std::strerror(errno);
        close(stdout_pipe[0]);
        close(stdout_pipe[1]);
        close(stderr_pipe[0]);
        close(stderr_pipe[1]);
        result.status = "failed";
        result.stderr_text = "fork: " + setup_error;
        result.finished_at = static_cast<std::uint64_t>(std::time(NULL));
        return result;
    }
    if (child == 0) {
        setpgid(0, 0);
        close(stdout_pipe[0]);
        close(stderr_pipe[0]);
        if (dup2(stdout_pipe[1], STDOUT_FILENO) < 0 || dup2(stderr_pipe[1], STDERR_FILENO) < 0) {
            _exit(126);
        }
        close(stdout_pipe[1]);
        close(stderr_pipe[1]);
        if (!task.cwd.empty() && chdir(task.cwd.c_str()) != 0) {
            static const char message[] = "chdir failed\n";
            write(STDERR_FILENO, message, sizeof(message) - 1);
            _exit(126);
        }
        for (std::size_t i=0;i<executable_paths.size();++i) {
            execve(executable_paths[i].c_str(), &argument_pointers[0], &environment_pointers[0]);
            if(errno!=ENOENT&&errno!=ENOTDIR&&errno!=EACCES) break;
        }
        static const char message[] = "exec program failed (missing, inaccessible or invalid executable)\n";
        write(STDERR_FILENO, message, sizeof(message) - 1);
        _exit(127);
    }

    close(stdout_pipe[1]);
    close(stderr_pipe[1]);
    setpgid(child, child);
    bool read_failed = false;
    if (!SetNonBlocking(stdout_pipe[0], &setup_error) || !SetNonBlocking(stderr_pipe[0], &setup_error)) {
        SignalProcessGroup(child, SIGKILL);
        int ignored_status = 0;
        while (waitpid(child, &ignored_status, 0) < 0 && errno == EINTR) {
        }
        if (stdout_pipe[0] >= 0) close(stdout_pipe[0]);
        if (stderr_pipe[0] >= 0) close(stderr_pipe[0]);
        result.status = "failed";
        result.stderr_text = "configure pipe: " + setup_error;
        result.finished_at = static_cast<std::uint64_t>(std::time(NULL));
        return result;
    }

    const SteadyClock::time_point deadline = SteadyClock::now() + std::chrono::seconds(task.timeout);
    SteadyClock::time_point terminate_started;
    bool timed_out = false;
    bool stopping = false;
    bool term_sent = false;
    bool kill_sent = false;
    bool child_reaped = false;
    int child_status = 0;

    while (!child_reaped || stdout_pipe[0] >= 0 || stderr_pipe[0] >= 0 ||
           (term_sent && !kill_sent)) {
        const SteadyClock::time_point now = SteadyClock::now();
        if (!term_sent && stop_requested != NULL && stop_requested->load()) {
            stopping = true;
            term_sent = true;
            terminate_started = now;
            SignalProcessGroup(child, SIGTERM);
        } else if (!term_sent && now >= deadline) {
            timed_out = true;
            term_sent = true;
            terminate_started = now;
            SignalProcessGroup(child, SIGTERM);
        }
        if (term_sent && !kill_sent && now >= terminate_started + std::chrono::milliseconds(200)) {
            kill_sent = true;
            SignalProcessGroup(child, SIGKILL);
        }

        struct pollfd descriptors[2];
        nfds_t count = 0;
        if (stdout_pipe[0] >= 0) {
            descriptors[count].fd = stdout_pipe[0];
            descriptors[count].events = POLLIN | POLLHUP;
            descriptors[count].revents = 0;
            ++count;
        }
        if (stderr_pipe[0] >= 0) {
            descriptors[count].fd = stderr_pipe[0];
            descriptors[count].events = POLLIN | POLLHUP;
            descriptors[count].revents = 0;
            ++count;
        }
        int poll_result;
        do {
            poll_result = poll(descriptors, count, 50);
        } while (poll_result < 0 && errno == EINTR);
        if (poll_result < 0) {
            read_failed = true;
        }
        DrainDescriptor(&stdout_pipe[0], &result.stdout_text, &result.truncated, &read_failed);
        DrainDescriptor(&stderr_pipe[0], &result.stderr_text, &result.truncated, &read_failed);

        // Escaped descendants can keep pipes open after the original process
        // group is killed. Give output a bounded drain interval, then close our
        // readers. This does not claim to terminate descendants using setsid.
        if (kill_sent && SteadyClock::now() >= terminate_started + std::chrono::milliseconds(400)) {
            if (stdout_pipe[0] >= 0) {
                close(stdout_pipe[0]);
                stdout_pipe[0] = -1;
                result.truncated = true;
            }
            if (stderr_pipe[0] >= 0) {
                close(stderr_pipe[0]);
                stderr_pipe[0] = -1;
                result.truncated = true;
            }
        }

        // Keep the child (possibly a zombie) until no further group signals
        // are needed. This pins its PID/PGID and avoids signaling a reused PID.
        if (!child_reaped && (kill_sent ||
            (!term_sent && stdout_pipe[0] < 0 && stderr_pipe[0] < 0 && !read_failed))) {
            pid_t waited;
            do {
                waited = waitpid(child, &child_status, WNOHANG);
            } while (waited < 0 && errno == EINTR);
            if (waited == child || (waited < 0 && errno == ECHILD)) {
                child_reaped = true;
            }
        }
        if (read_failed && !term_sent) {
            term_sent = true;
            terminate_started = SteadyClock::now();
            SignalProcessGroup(child, SIGTERM);
        }
    }

    if (!child_reaped) {
        while (waitpid(child, &child_status, 0) < 0 && errno == EINTR) {
        }
    }
    result.finished_at = static_cast<std::uint64_t>(std::time(NULL));
    if (timed_out) {
        result.status = "timeout";
    } else if (stopping || read_failed) {
        result.status = "failed";
    } else if (WIFEXITED(child_status)) {
        result.exit_code = WEXITSTATUS(child_status);
        result.status = result.exit_code == 0 ? "success" : "failed";
    } else if (WIFSIGNALED(child_status)) {
        result.exit_code = 128 + WTERMSIG(child_status);
        result.status = "failed";
    } else {
        result.status = "failed";
    }
    if (timed_out && WIFSIGNALED(child_status)) {
        result.exit_code = 128 + WTERMSIG(child_status);
    }
    return result;
}

bool BuildTaskResultPayload(ExecResult result,
                            std::uint32_t max_payload,
                            std::string* payload,
                            std::string* error) {
    result.stdout_text = SanitizeUtf8(result.stdout_text);
    result.stderr_text = SanitizeUtf8(result.stderr_text);
    *payload = SerializeResult(result);
    if (payload->size() <= max_payload) {
        return true;
    }

    const std::string full_stdout = result.stdout_text;
    const std::string full_stderr = result.stderr_text;
    const std::size_t total = full_stdout.size() + full_stderr.size();
    std::size_t low = 0;
    std::size_t high = total;
    std::string best;
    while (low <= high) {
        const std::size_t keep = low + (high - low) / 2;
        const std::size_t stdout_share = total == 0
                                             ? 0
                                             : static_cast<std::size_t>(
                                                   (static_cast<long double>(full_stdout.size()) * keep) / total);
        const std::size_t stderr_share = keep - stdout_share;
        result.stdout_text.assign(full_stdout, 0, Utf8PrefixLength(full_stdout, stdout_share));
        result.stderr_text.assign(full_stderr, 0, Utf8PrefixLength(full_stderr, stderr_share));
        result.truncated = true;
        const std::string candidate = SerializeResult(result);
        if (candidate.size() <= max_payload) {
            best = candidate;
            low = keep + 1;
        } else {
            if (keep == 0) {
                break;
            }
            high = keep - 1;
        }
    }
    if (best.empty()) {
        if (error != NULL) {
            *error = "TASK_RESULT metadata exceeds max_control_payload";
        }
        return false;
    }
    *payload = best;
    return true;
}

}  // namespace rmp
