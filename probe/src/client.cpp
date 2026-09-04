#include "rmp/client.h"

#include "rmp/frame.h"
#include "rmp/json.h"

#include <arpa/inet.h>
#include <cerrno>
#include <chrono>
#include <climits>
#include <cstring>
#include <iostream>
#include <limits>
#include <netdb.h>
#include <poll.h>
#include <set>
#include <sstream>
#include <stdexcept>
#include <sys/socket.h>
#include <sys/sysinfo.h>
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

bool SendJsonFrame(int socket_fd,
                   std::uint8_t type,
                   std::uint64_t message_id,
                   const std::string& payload) {
    Header header;
    header.type = type;
    header.message_id = message_id;
    try {
        return SendAll(socket_fd, EncodeFrame(header, Bytes(payload)));
    } catch (const std::exception& exception) {
        std::cerr << "state=PROTOCOL_ERROR detail=" << exception.what() << std::endl;
        return false;
    }
}

std::string RegisterPayload(const ClientConfig& config) {
    std::ostringstream output;
    output << "{\"device_id\":" << EscapeJsonString(config.device_id)
           << ",\"probe_version\":" << EscapeJsonString(config.probe_version);
    if (!config.hostname.empty()) {
        output << ",\"hostname\":" << EscapeJsonString(config.hostname);
    }
    output << ",\"arch\":" << EscapeJsonString(config.arch)
           << ",\"boot_id\":" << EscapeJsonString(config.boot_id)
           << ",\"capabilities\":[]}";
    return output.str();
}

std::string HeartbeatPayload() {
    struct sysinfo info;
    std::uint64_t uptime = 0;
    if (sysinfo(&info) == 0 && info.uptime > 0) {
        uptime = static_cast<std::uint64_t>(info.uptime);
    }
    std::ostringstream output;
    output << "{\"uptime\":" << uptime << ",\"running_tasks\":0}";
    return output.str();
}

int Connect(const ClientConfig& config, std::string* error) {
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
        const int candidate = socket(current->ai_family, current->ai_socktype, current->ai_protocol);
        if (candidate < 0) {
            last_error = errno;
            continue;
        }
        const int enabled = 1;
        setsockopt(candidate, SOL_SOCKET, SO_KEEPALIVE, &enabled, sizeof(enabled));
        if (connect(candidate, current->ai_addr, current->ai_addrlen) == 0) {
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

bool ValidateIncomingHeader(const Frame& frame,
                            std::uint64_t* expected_message_id,
                            std::uint8_t expected_type,
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
    if (frame.header.type != expected_type) {
        std::ostringstream message;
        message << "unexpected message type 0x" << std::hex
                << static_cast<unsigned>(frame.header.type);
        *error = message.str();
        return false;
    }
    if (frame.header.flags != kFlagResponse) {
        *error = "response flags are invalid";
        return false;
    }
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

SessionResult RunSession(int socket_fd, const ClientConfig& config) {
    SessionResult result;
    std::uint64_t next_probe_message_id = 1;
    std::uint64_t expected_server_message_id = 1;

    std::cout << "state=REGISTERING device_id=" << config.device_id << std::endl;
    const std::string register_payload = RegisterPayload(config);
    const std::uint64_t register_message_id = next_probe_message_id++;
    if (!SendJsonFrame(socket_fd, kTypeRegister, register_message_id, register_payload)) {
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
    if (frames.size() != 1) {
        std::cerr << "state=PROTOCOL_ERROR detail=unexpected_frames_during_registration" << std::endl;
        return result;
    }

    std::string validation_error;
    if (!ValidateIncomingHeader(frames[0], &expected_server_message_id, kTypeRegisterAck, &validation_error)) {
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

    std::cout << "state=ONLINE device_id=" << config.device_id
              << " session_id=" << register_ack.session_id
              << " heartbeat_interval=" << register_ack.heartbeat_interval << std::endl;

    StreamDecoder online_decoder(register_ack.max_control_payload);
    std::set<std::uint64_t> pending_heartbeats;
    const std::chrono::seconds heartbeat_interval(register_ack.heartbeat_interval);
    SteadyClock::time_point last_seen = SteadyClock::now();
    SteadyClock::time_point next_heartbeat = last_seen + heartbeat_interval;

    while (true) {
        const SteadyClock::time_point lost_deadline = last_seen + 3 * heartbeat_interval;
        SteadyClock::time_point wake_at = next_heartbeat;
        if (lost_deadline < wake_at) {
            wake_at = lost_deadline;
        }
        const int timeout_ms = MillisecondsUntil(wake_at);
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
                frames.clear();
                FrameErrorCode frame_error = FrameErrorCode::kNone;
                if (!online_decoder.Feed(buffer, static_cast<std::size_t>(received), &frames, &frame_error)) {
                    std::cerr << "state=PROTOCOL_ERROR detail=" << FrameErrorName(frame_error) << std::endl;
                    return result;
                }
                for (std::vector<Frame>::const_iterator frame = frames.begin(); frame != frames.end(); ++frame) {
                    if (!ValidateIncomingHeader(*frame, &expected_server_message_id, kTypeHeartbeatAck, &validation_error)) {
                        std::cerr << "state=PROTOCOL_ERROR detail=" << validation_error << std::endl;
                        return result;
                    }
                    const std::string heartbeat_json(frame->payload.begin(), frame->payload.end());
                    HeartbeatAck heartbeat_ack;
                    if (!ParseHeartbeatAck(heartbeat_json, &heartbeat_ack, &validation_error) ||
                        pending_heartbeats.erase(heartbeat_ack.reply_to) != 1) {
                        if (validation_error.empty()) {
                            validation_error = "HEARTBEAT_ACK reply_to mismatch";
                        }
                        std::cerr << "state=PROTOCOL_ERROR detail=" << validation_error << std::endl;
                        return result;
                    }
                    last_seen = SteadyClock::now();
                    std::cout << "received=HEARTBEAT_ACK message_id=" << frame->header.message_id
                              << " reply_to=" << heartbeat_ack.reply_to << std::endl;
                }
            }
        }

        const SteadyClock::time_point now = SteadyClock::now();
        if (now >= last_seen + 3 * heartbeat_interval) {
            std::cerr << "state=RECONNECTING reason=server_lost" << std::endl;
            return result;
        }
        if (now >= next_heartbeat) {
            if (next_probe_message_id == 0 ||
                next_probe_message_id == std::numeric_limits<std::uint64_t>::max()) {
                std::cerr << "state=RECONNECTING reason=message_id_exhausted" << std::endl;
                return result;
            }
            const std::uint64_t heartbeat_message_id = next_probe_message_id++;
            if (!SendJsonFrame(socket_fd, kTypeHeartbeat, heartbeat_message_id, HeartbeatPayload())) {
                return result;
            }
            pending_heartbeats.insert(heartbeat_message_id);
            std::cout << "sent=HEARTBEAT message_id=" << heartbeat_message_id << std::endl;
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

int RunClient(const ClientConfig& config) {
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
        const SessionResult session = RunSession(socket_fd, config);
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
