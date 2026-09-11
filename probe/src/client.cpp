#include "rmp/file_manager.h"
#include "rmp/tunnel.h"
#include "rmp/priority_gate.h"
#include "rmp/pending_heartbeats.h"
#include "rmp/client.h"
#include "rmp/collection.h"
#include "rmp/telemetry.h"
#include "rmp/live_config.h"

#include "rmp/frame.h"
#include "rmp/json.h"
#include "rmp/task.h"
#include "rmp/task_manager.h"

#include <arpa/inet.h>
#include <algorithm>
#include <atomic>
#include <cerrno>
#include <chrono>
#include <climits>
#include <condition_variable>
#include <cstring>
#include <deque>
#include <fcntl.h>
#include <iostream>
#include <limits>
#include <mutex>
#include <netdb.h>
#include <poll.h>
#include <sstream>
#include <stdexcept>
#include <sys/socket.h>
#include "rmp/system_info.h"
#include <sys/time.h>
#include <thread>
#include <unistd.h>

namespace rmp {
namespace {

typedef std::chrono::steady_clock SteadyClock;

struct SessionResult {
    std::uint32_t retry_after;
    SessionResult() : retry_after(0) {}
};

std::vector<std::uint8_t> Bytes(const std::string& value) {
    return std::vector<std::uint8_t>(value.begin(), value.end());
}

bool SendAll(int socket_fd, const std::vector<std::uint8_t>& data) {
    std::size_t written = 0;
    while (written < data.size()) {
#ifdef MSG_NOSIGNAL
        const int flags = MSG_NOSIGNAL;
#else
        const int flags = 0;
#endif
        const ssize_t result = send(socket_fd, &data[written], data.size() - written, flags);
        if (result > 0) {
            written += static_cast<std::size_t>(result);
            continue;
        }
        if (result < 0 && errno == EINTR) {
            continue;
        }
        return false;
    }
    return true;
}

class SessionWriter {
public:
    explicit SessionWriter(int socket_fd)
        : socket_fd_(socket_fd), next_message_id_(1), max_control_(1024*1024), failed_(false) {}

    void SetLimit(std::uint32_t limit) {max_control_=limit;}
    bool Send(std::uint8_t type,
              std::uint16_t flags,
              const std::string& payload,
              std::uint64_t* message_id) {
        PriorityGuard priority(priority_,type==kTypeFileChunk);
        std::lock_guard<std::mutex> lock(mutex_);
        if(failed_) return false;
        if(type!=kTypeFileChunk && payload.size()>max_control_) {failed_=true;shutdown(socket_fd_,SHUT_RDWR);return false;}
        if (next_message_id_ == 0 ||
            next_message_id_ == std::numeric_limits<std::uint64_t>::max()) {
            std::cerr << "state=PROTOCOL_ERROR detail=probe_message_id_exhausted" << std::endl;
            failed_=true;
            shutdown(socket_fd_, SHUT_RDWR);
            return false;
        }
        Header header;
        header.type = type;
        header.flags = flags;
        header.message_id = next_message_id_;
        try {
            if (!SendAll(socket_fd_, EncodeFrame(header, Bytes(payload)))) {
                failed_=true;
            shutdown(socket_fd_, SHUT_RDWR);
                return false;
            }
        } catch (const std::exception& exception) {
            std::cerr << "state=PROTOCOL_ERROR detail=" << exception.what() << std::endl;
            failed_=true;
            shutdown(socket_fd_, SHUT_RDWR);
            return false;
        }
        *message_id = next_message_id_++;
        return true;
    }

private:
    PriorityGate priority_;
    int socket_fd_;
    std::uint64_t next_message_id_;
    std::uint32_t max_control_;
    bool failed_;
    std::mutex mutex_;
};

std::string RegisterPayload(const ClientConfig& config) {
    std::ostringstream output;
    output << "{\"device_id\":" << EscapeJsonString(config.device_id)
           << ",\"probe_version\":" << EscapeJsonString(config.probe_version);
    if (!config.hostname.empty()) {
        output << ",\"hostname\":" << EscapeJsonString(config.hostname);
    }
    for(std::map<std::string,std::string>::const_iterator i=config.properties.begin();i!=config.properties.end();++i)
        output<<','<<EscapeJsonString(i->first)<<':'<<EscapeJsonString(i->second);
    output << ",\"arch\":" << EscapeJsonString(config.arch)
           << ",\"boot_id\":" << EscapeJsonString(config.boot_id)
           << ",\"capabilities\":[\"exec\",\"file\",\"tunnel\",\"router_config\",\"telemetry_v2\",\"managed_config_v1\",\"port_counters_v1\",\"neighbors_v1\",\"neighbors_inspect_v1\"]}";
    return output.str();
}

int Connect(const ClientConfig& config, std::string* error, bool bounded = false) {
    struct addrinfo hints;
    std::memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;

    struct addrinfo* addresses = NULL;
    const int lookup = getaddrinfo(config.server_host.c_str(), config.server_port.c_str(), &hints, &addresses);
    if (lookup != 0) {
        *error = gai_strerror(lookup);
        return -1;
    }

    int connected = -1;
    int last_error = 0;
    for (struct addrinfo* current = addresses; current != NULL; current = current->ai_next) {
        std::unique_lock<std::mutex> fork_lock(ExecForkMutex());
        const int candidate = socket(current->ai_family, current->ai_socktype, current->ai_protocol);
        if (candidate < 0) {
            last_error = errno;
            continue;
        }
        const int descriptor_flags = fcntl(candidate, F_GETFD, 0);
        if (descriptor_flags < 0 || fcntl(candidate, F_SETFD, descriptor_flags | FD_CLOEXEC) != 0) {
            last_error = errno;
            close(candidate);
            continue;
        }
        fork_lock.unlock();
        // Keep kernel queued file bytes bounded so frame-boundary priority is useful on slow links.
        const int send_buffer = 64 * 1024;
        setsockopt(candidate, SOL_SOCKET, SO_SNDBUF, &send_buffer, sizeof(send_buffer));
        const int enabled = 1;
        setsockopt(candidate, SOL_SOCKET, SO_KEEPALIVE, &enabled, sizeof(enabled));
        struct timeval send_timeout;
        send_timeout.tv_sec = 10;
        send_timeout.tv_usec = 0;
        setsockopt(candidate, SOL_SOCKET, SO_SNDTIMEO, &send_timeout, sizeof(send_timeout));
        const int original_flags=fcntl(candidate,F_GETFL,0);
        if(bounded && (original_flags<0 || fcntl(candidate,F_SETFL,original_flags|O_NONBLOCK)<0)) {last_error=errno;close(candidate);continue;}
        int connect_result=connect(candidate,current->ai_addr,current->ai_addrlen);
        if(bounded && connect_result<0 && errno==EINPROGRESS){
            struct pollfd writable={candidate,POLLOUT,0};int pending=0;socklen_t size=sizeof(pending);
            const int ready=poll(&writable,1,10000);
            if(ready>0&&getsockopt(candidate,SOL_SOCKET,SO_ERROR,&pending,&size)==0&&pending==0)connect_result=0;
            else errno=pending?pending:ETIMEDOUT;
        }
        if (connect_result == 0) {
            if(bounded&&fcntl(candidate,F_SETFL,original_flags)<0){last_error=errno;close(candidate);continue;}
            connected = candidate;
            break;
        }
        last_error = errno;
        close(candidate);
    }
    freeaddrinfo(addresses);
    if (connected < 0) {
        *error = std::strerror(last_error == 0 ? ECONNREFUSED : last_error);
    }
    return connected;
}

bool PollReadable(int socket_fd, int timeout_ms, std::string* error) {
    struct pollfd descriptor;
    descriptor.fd = socket_fd;
    descriptor.events = POLLIN;
    descriptor.revents = 0;
    int result;
    do {
        result = poll(&descriptor, 1, timeout_ms);
    } while (result < 0 && errno == EINTR);
    if (result < 0) {
        *error = std::strerror(errno);
        return false;
    }
    if (result == 0) {
        *error = "timeout";
        return false;
    }
    if ((descriptor.revents & POLLIN) == 0) {
        *error = "connection closed";
        return false;
    }
    return true;
}

bool ReceiveFrames(int socket_fd,
                   StreamDecoder* decoder,
                   int timeout_ms,
                   std::vector<Frame>* frames,
                   std::string* error) {
    if (!PollReadable(socket_fd, timeout_ms, error)) {
        return false;
    }
    std::uint8_t buffer[32768];
    ssize_t received;
    do {
        received = recv(socket_fd, buffer, sizeof(buffer), 0);
    } while (received < 0 && errno == EINTR);
    if (received <= 0) {
        *error = received == 0 ? "connection closed" : std::strerror(errno);
        return false;
    }
    FrameErrorCode frame_error = FrameErrorCode::kNone;
    if (!decoder->Feed(buffer, static_cast<std::size_t>(received), frames, &frame_error)) {
        *error = FrameErrorName(frame_error);
        return false;
    }
    return true;
}

bool ValidateIncomingMessageID(const Frame& frame,
                               std::uint64_t* expected_message_id,
                               std::string* error) {
    if (frame.header.message_id != *expected_message_id) {
        std::ostringstream message;
        message << "server message_id " << frame.header.message_id
                << " does not match expected " << *expected_message_id;
        *error = message.str();
        return false;
    }
    if (*expected_message_id == std::numeric_limits<std::uint64_t>::max()) {
        *error = "server message_id exhausted";
        return false;
    }
    ++*expected_message_id;
    return true;
}

int MillisecondsUntil(const SteadyClock::time_point& deadline) {
    const SteadyClock::time_point now = SteadyClock::now();
    if (deadline <= now) {
        return 0;
    }
    const std::chrono::milliseconds remaining =
        std::chrono::duration_cast<std::chrono::milliseconds>(deadline - now);
    if (remaining.count() > INT_MAX) {
        return INT_MAX;
    }
    return static_cast<int>(remaining.count());
}

bool HandleOnlineFrames(const std::vector<Frame>& frames,
                        std::uint64_t* expected_server_message_id,
                        PendingHeartbeats* pending_heartbeats,
                        SessionWriter* writer,
                        TaskManager* task_worker,
                        FileManager* files,
                        TunnelManager* tunnels,
 LiveTelemetry* telemetry,
                        std::uint32_t file_chunk_size,
                        std::uint32_t max_payload,
                        SteadyClock::time_point* last_seen,
                        std::string* error) {
    for (std::vector<Frame>::const_iterator frame = frames.begin(); frame != frames.end(); ++frame) {
        error->clear();
        if (frame->payload.size() > (frame->header.type==kTypeFileChunk?28+file_chunk_size:max_payload)) {
            *error = "payload exceeds negotiated limit";
            return false;
        }
        if (!ValidateIncomingMessageID(*frame, expected_server_message_id, error)) {
            return false;
        }
        if(frame->header.type==0x07){if(frame->header.flags!=0||!telemetry->Apply(std::string(frame->payload.begin(),frame->payload.end()),frame->header.message_id,error))return false;*last_seen=SteadyClock::now();continue;}
 if (frame->header.type==kTypeTunnelConnect || frame->header.type==kTypeTunnelClose) {
            if(!tunnels->Feed(*frame,error))return false;
            *last_seen=SteadyClock::now();continue;
        }
        if (frame->header.type>=kTypeFileBegin && frame->header.type<=kTypeFileAck) {
            if(!files->Feed(*frame,error)) return false;
            *last_seen=SteadyClock::now();continue;
        }
        if (frame->header.type == kTypeHeartbeatAck) {
            if (frame->header.flags != kFlagResponse) {
                *error = "HEARTBEAT_ACK flags are invalid";
                return false;
            }
            const std::string heartbeat_json(frame->payload.begin(), frame->payload.end());
            HeartbeatAck heartbeat_ack;
            if (!ParseHeartbeatAck(heartbeat_json, &heartbeat_ack, error) ||
                !pending_heartbeats->Acknowledge(heartbeat_ack.reply_to)) {
                if (error->empty()) {
                    *error = "HEARTBEAT_ACK reply_to mismatch";
                }
                return false;
            }
            *last_seen = SteadyClock::now();
            std::cout << "received=HEARTBEAT_ACK message_id=" << frame->header.message_id
                      << " reply_to=" << heartbeat_ack.reply_to << std::endl;
            continue;
        }
        if (frame->header.type == kTypeTask) {
            if (frame->header.flags != 0) {
                *error = "TASK flags are invalid";
                return false;
            }
            const std::string task_json(frame->payload.begin(), frame->payload.end());
            ExecTask task;
            const bool parsed = ParseTask(task_json, &task, error);
            if (!parsed && (task.task_id.empty() || task.task_id.size() > 128)) {
                return false;
            }
            bool fresh=false;
            const std::string state = task_worker->Submit(task, parsed, frame->payload.size(), max_payload,&fresh);
            const bool file=task.type=="upload"||task.type=="download";
            if(fresh&&file) files->Enqueue(task);
            std::uint64_t outgoing_id = 0;
            if (state == "conflict") {
                const std::string payload = "{\"reply_to\":" + std::to_string(frame->header.message_id) +
                    ",\"code\":\"INVALID_PAYLOAD\",\"message\":\"task_id content conflict\"}";
                if (!writer->Send(kTypeError, kFlagResponse, payload, &outgoing_id)) return false;
            } else {
                const bool accepted = state != "rejected";
                std::string reason;
                if (!parsed) reason = "invalid task payload";
                else if (task.type != "exec" && task.type != "router_config" && task.type != "neighbor_scan" && task.type != "neighbor_cancel" && task.type != "neighbor_inspect" && !file) reason = "unsupported task type";
                else if (!accepted) reason = "task capacity exhausted";
                std::string ack = TaskAckPayload(frame->header.message_id, task.task_id, accepted, reason, state);
                if (!writer->Send(kTypeTaskAck, kFlagResponse, ack, &outgoing_id)) return false;
                if(fresh&&file) files->Enable(task.task_id);
                std::cout << "sent=TASK_ACK message_id=" << outgoing_id << " task_id=" << task.task_id
                          << " state=" << state << std::endl;
                if (state == "success" || state == "failed" || state == "timeout") {
                    std::string payload;
                    if (task_worker->CachedResult(task.task_id, max_payload, &payload)) {
                        if (!writer->Send(kTypeTaskResult, 0, payload, &outgoing_id)) return false;
                    } else {
                        const std::string failure = "{\"reply_to\":" + std::to_string(frame->header.message_id) +
                            ",\"code\":\"PAYLOAD_TOO_LARGE\",\"message\":\"cached result exceeds session limit\"}";
                        if (!writer->Send(kTypeError, kFlagResponse, failure, &outgoing_id)) return false;
                    }
                }
            }
            *last_seen = SteadyClock::now();
            continue;
        }
        std::ostringstream message;
        message << "unexpected message type 0x" << std::hex
                << static_cast<unsigned>(frame->header.type);
        *error = message.str();
        return false;
    }
    return true;
}

SessionResult RunSession(int socket_fd, const ClientConfig& config, TaskManager& task_worker, SystemSampler& sampler) {
    SessionResult result;
    std::uint64_t expected_server_message_id = 1;
    SessionWriter writer(socket_fd);

    std::cout << "state=REGISTERING device_id=" << config.device_id << std::endl;
    const std::string register_payload = RegisterPayload(config);
    std::uint64_t register_message_id = 0;
    if (!writer.Send(kTypeRegister, 0, register_payload, &register_message_id)) {
        return result;
    }
    std::cout << "sent=REGISTER message_id=" << register_message_id << std::endl;

    StreamDecoder registration_decoder(kMaxControlPayload);
    std::vector<Frame> frames;
    std::string receive_error;
    const SteadyClock::time_point registration_deadline = SteadyClock::now() + std::chrono::seconds(30);
    while (frames.empty()) {
        const int timeout_ms = MillisecondsUntil(registration_deadline);
        if (timeout_ms == 0 || !ReceiveFrames(socket_fd, &registration_decoder, timeout_ms, &frames, &receive_error)) {
            std::cerr << "state=REGISTERING error=" << receive_error << std::endl;
            return result;
        }
    }
    std::string validation_error;
    if (!ValidateIncomingMessageID(frames[0], &expected_server_message_id, &validation_error) ||
        frames[0].header.type != kTypeRegisterAck || frames[0].header.flags != kFlagResponse) {
        if (validation_error.empty()) {
            validation_error = "REGISTER_ACK type or flags are invalid";
        }
        std::cerr << "state=PROTOCOL_ERROR detail=" << validation_error << std::endl;
        return result;
    }
    const std::string ack_json(frames[0].payload.begin(), frames[0].payload.end());
    RegisterAck register_ack;
    if (!ParseRegisterAck(ack_json, &register_ack, &validation_error) ||
        register_ack.reply_to != register_message_id) {
        if (validation_error.empty()) {
            validation_error = "REGISTER_ACK reply_to mismatch";
        }
        std::cerr << "state=PROTOCOL_ERROR detail=" << validation_error << std::endl;
        return result;
    }
    if (!register_ack.success) {
        result.retry_after = register_ack.retry_after;
        std::cerr << "state=REGISTER_REJECTED error_code=" << register_ack.error_code
                  << " retry_after=" << register_ack.retry_after
                  << " message=" << register_ack.message << std::endl;
        return result;
    }
    frames.erase(frames.begin());
    registration_decoder.SetMaxPayload(register_ack.max_control_payload);
    // A coalesced ACK can leave only the next header buffered under the old
    // limit. Revalidate it now without waiting for another byte from the peer.
    FrameErrorCode negotiated_error = FrameErrorCode::kNone;
    if (!registration_decoder.Feed(NULL, 0, &frames, &negotiated_error)) {
        std::cerr << "state=PROTOCOL_ERROR detail=" << FrameErrorName(negotiated_error) << std::endl;
        return result;
    }

    std::cout << "state=ONLINE device_id=" << config.device_id
              << " session_id=" << register_ack.session_id
              << " heartbeat_interval=" << register_ack.heartbeat_interval << std::endl;

    LiveTelemetry telemetry(config,&sampler);
    PendingHeartbeats pending_heartbeats;
    writer.SetLimit(register_ack.max_control_payload);
    task_worker.BeginSession();
    FileManager files(task_worker,[&writer](std::uint8_t t,std::uint16_t f,const std::string&p,std::uint64_t*id){return writer.Send(t,f,p,id);},[socket_fd]{shutdown(socket_fd,SHUT_RDWR);},register_ack.file_chunk_size);
    TunnelManager tunnels(register_ack.session_id,config.tunnel_connections,[&writer](const std::string& p){std::uint64_t id=0;return writer.Send(kTypeTunnelStatus,0,p,&id);});
    const std::chrono::seconds heartbeat_interval(register_ack.heartbeat_interval);
    SteadyClock::time_point last_seen = SteadyClock::now();
    SteadyClock::time_point next_heartbeat = last_seen;

    while (true) {
        if (!frames.empty()) {
            if (!HandleOnlineFrames(frames, &expected_server_message_id, &pending_heartbeats,
                                    &writer, &task_worker, &files, &tunnels, &telemetry, register_ack.file_chunk_size, register_ack.max_control_payload, &last_seen, &validation_error)) {
                std::cerr << "state=PROTOCOL_ERROR detail=" << validation_error << std::endl;
                return result;
            }
            frames.clear();
        }
        const SteadyClock::time_point lost_deadline = last_seen + 3 * heartbeat_interval;
        SteadyClock::time_point wake_at = next_heartbeat;
        if (lost_deadline < wake_at) {
            wake_at = lost_deadline;
        }
        const int timeout_ms = std::min(50, MillisecondsUntil(wake_at));
        if (timeout_ms > 0) {
            struct pollfd descriptor;
            descriptor.fd = socket_fd;
            descriptor.events = POLLIN;
            descriptor.revents = 0;
            int poll_result;
            do {
                poll_result = poll(&descriptor, 1, timeout_ms);
            } while (poll_result < 0 && errno == EINTR);
            if (poll_result < 0) {
                std::cerr << "state=ONLINE error=" << std::strerror(errno) << std::endl;
                return result;
            }
            if (poll_result > 0) {
                if ((descriptor.revents & POLLIN) == 0) {
                    std::cerr << "state=ONLINE error=connection_closed" << std::endl;
                    return result;
                }
                std::uint8_t buffer[32768];
                ssize_t received;
                do {
                    received = recv(socket_fd, buffer, sizeof(buffer), 0);
                } while (received < 0 && errno == EINTR);
                if (received <= 0) {
                    std::cerr << "state=ONLINE error=connection_closed" << std::endl;
                    return result;
                }
                FrameErrorCode frame_error = FrameErrorCode::kNone;
                if (!registration_decoder.Feed(buffer, static_cast<std::size_t>(received), &frames, &frame_error)) {
                    std::cerr << "state=PROTOCOL_ERROR detail=" << FrameErrorName(frame_error) << std::endl;
                    return result;
                }
                if (!frames.empty() &&
                    !HandleOnlineFrames(frames, &expected_server_message_id, &pending_heartbeats,
                                        &writer, &task_worker, &files, &tunnels, &telemetry, register_ack.file_chunk_size, register_ack.max_control_payload, &last_seen, &validation_error)) {
                    std::cerr << "state=PROTOCOL_ERROR detail=" << validation_error << std::endl;
                    return result;
                }
                frames.clear();
            }
        }

        std::string observation;
 if(telemetry.NextAck(&observation)){std::uint64_t id;if(!writer.Send(0x08,kFlagResponse,observation,&id))return result;}
        if(telemetry.Next(register_ack.max_control_payload,&observation)){std::uint64_t id;if(!writer.Send(0x20,0,observation,&id))return result;}
        files.Tick();
        if(!tunnels.Tick())return result;
        std::string result_payload;
        if (task_worker.NextResult(register_ack.max_control_payload, &result_payload)) {
            std::uint64_t result_id = 0;
            if (!writer.Send(kTypeTaskResult, 0, result_payload, &result_id)) return result;
            std::cout << "sent=TASK_RESULT message_id=" << result_id << std::endl;
        }

        const SteadyClock::time_point now = SteadyClock::now();
        if (now >= last_seen + 3 * heartbeat_interval) {
            std::cerr << "state=RECONNECTING reason=server_lost" << std::endl;
            return result;
        }
        if (now >= next_heartbeat) {
            if (pending_heartbeats.Full()) {
                std::cerr << "state=RECONNECTING reason=pending_heartbeat_capacity" << std::endl;
                return result;
            }
            const unsigned running_tasks = task_worker.RunningTasks();
            std::uint64_t heartbeat_message_id = 0;
            if (!writer.Send(kTypeHeartbeat, 0, HeartbeatPayload(running_tasks, ReadSystemUptime()),
                             &heartbeat_message_id)) {
                return result;
            }
            pending_heartbeats.Add(heartbeat_message_id);
            std::cout << "sent=HEARTBEAT message_id=" << heartbeat_message_id
                      << " running_tasks=" << running_tasks << std::endl;
            next_heartbeat = now + heartbeat_interval;
        }
    }
}


}  // namespace

bool ParseServerAddress(const std::string& address,
                        std::string* host,
                        std::string* port,
                        std::string* error) {
    if (address.empty()) {
        *error = "server address is empty";
        return false;
    }
    if (address[0] == '[') {
        const std::size_t closing = address.find(']');
        if (closing == std::string::npos || closing + 2 > address.size() || address[closing + 1] != ':') {
            *error = "IPv6 server address must use [host]:port";
            return false;
        }
        *host = address.substr(1, closing - 1);
        *port = address.substr(closing + 2);
    } else {
        const std::size_t colon = address.rfind(':');
        if (colon == std::string::npos) {
            *error = "server address must use host:port";
            return false;
        }
        *host = address.substr(0, colon);
        *port = address.substr(colon + 1);
    }
    if (host->empty() || port->empty()) {
        *error = "server host and port must not be empty";
        return false;
    }
    char* end = NULL;
    errno = 0;
    const long parsed_port = std::strtol(port->c_str(), &end, 10);
    if (errno != 0 || end == NULL || *end != '\0' || parsed_port < 1 || parsed_port > 65535) {
        *error = "server port must be 1-65535";
        return false;
    }
    return true;
}

int RunClient(const ClientConfig& initial) {
    ClientConfig config=initial;
    SystemSampler sampler("",config.network_interfaces);
    sampler.StartHardware(config.monitoring.at("cpu")?config.monitoring.at("cpu"):5);
    CollectFirmware(&config);
    TaskManager task_worker(config.task_workers, config.task_capacity, config.task_cache_bytes, config.file_queue_capacity);
    task_worker.SetNeighbors(sampler.NeighborCollector());
    const unsigned delays[] = {1, 2, 5, 10, 30};
    std::size_t backoff_index = 0;
    while (true) {
        std::cout << "state=CONNECTING server=" << config.server_host << ':' << config.server_port << std::endl;
        std::string connect_error;
        const int socket_fd = Connect(config, &connect_error);
        if (socket_fd < 0) {
            const unsigned delay = delays[backoff_index];
            if (backoff_index + 1 < sizeof(delays) / sizeof(delays[0])) {
                ++backoff_index;
            }
            std::cerr << "state=RECONNECTING reason=" << connect_error
                      << " delay=" << delay << std::endl;
            std::this_thread::sleep_for(std::chrono::seconds(delay));
            continue;
        }

        backoff_index = 0;
        const SessionResult session = RunSession(socket_fd, config, task_worker, sampler);
        close(socket_fd);
        if (session.retry_after > 0) {
            std::cerr << "state=RECONNECTING delay=" << session.retry_after << std::endl;
            std::this_thread::sleep_for(std::chrono::seconds(session.retry_after));
            continue;
        }
        const unsigned delay = delays[backoff_index];
        if (backoff_index + 1 < sizeof(delays) / sizeof(delays[0])) {
            ++backoff_index;
        }
        std::cerr << "state=RECONNECTING delay=" << delay << std::endl;
        std::this_thread::sleep_for(std::chrono::seconds(delay));
    }
}

}  // namespace rmp
