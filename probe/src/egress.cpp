#include "rmp/egress.h"
#include "rmp/task.h"
#include "rmp/egress_roots.h"
#include <mbedtls/ctr_drbg.h>
#include <mbedtls/entropy.h>
#include <mbedtls/ssl.h>
#include <mbedtls/x509_crt.h>
#include <algorithm>
#include <arpa/inet.h>
#include <cerrno>
#include <chrono>
#include <cstring>
#include <fcntl.h>
#include <memory>
#include <mutex>
#include <netdb.h>
#include <poll.h>
#include <sstream>
#include <sys/socket.h>
#include <thread>
#include <unistd.h>
#include <vector>

namespace rmp {
namespace {
typedef std::chrono::steady_clock Clock;
bool Stopped(const std::atomic<bool>* stop) { return stop && stop->load(); }
std::string Trim(const std::string& value) {
    const auto first = value.find_first_not_of(" \t\r\n");
    return first == std::string::npos ? "" : value.substr(first, value.find_last_not_of(" \t\r\n") - first + 1);
}
std::string Lower(std::string value) {
    for (auto& c : value) if (c >= 'A' && c <= 'Z') c = static_cast<char>(c + 'a' - 'A');
    return value;
}
bool Number(const std::string& text, unsigned base, std::size_t* value) {
    *value = 0;
    if (text.empty()) return false;
    for (const auto c : text) {
        const unsigned digit = c >= '0' && c <= '9' ? c - '0' : c >= 'a' && c <= 'f' ? c - 'a' + 10 : c >= 'A' && c <= 'F' ? c - 'A' + 10 : 255;
        if (digit >= base || *value > (8192 - digit) / base) return false;
        *value = *value * base + digit;
    }
    return true;
}
struct Address { sockaddr_storage storage; socklen_t length; };
struct Resolution {
    std::atomic<bool> done{false};
    std::vector<Address> addresses;
};

// libc resolution is isolated from control and bounded to one outstanding job
// per address family, including after caller cancellation or DNS timeout.
std::shared_ptr<Resolution> Resolve(const EgressEndpoint& endpoint, int family) {
    static std::mutex gate;
    static std::shared_ptr<Resolution> jobs[2];
    std::lock_guard<std::mutex> lock(gate);
    const int index = family == 4 ? 0 : 1;
    if (jobs[index] && !jobs[index]->done) return std::shared_ptr<Resolution>();
    auto result = std::make_shared<Resolution>();
    jobs[index] = result;
    try {
        std::thread([result, endpoint, family]() {
            addrinfo hints = {}, *addresses = NULL;
            hints.ai_family = family == 4 ? AF_INET : AF_INET6;
            hints.ai_socktype = SOCK_STREAM;
            if (getaddrinfo(endpoint.host.c_str(), endpoint.port.c_str(), &hints, &addresses) == 0) {
                for (auto item = addresses; item && result->addresses.size() < 8; item = item->ai_next) {
                    if (item->ai_addrlen > sizeof(sockaddr_storage)) continue;
                    Address value = {}; value.length = item->ai_addrlen;
                    std::memcpy(&value.storage, item->ai_addr, item->ai_addrlen);
                    result->addresses.push_back(value);
                }
                freeaddrinfo(addresses);
            }
            result->done = true;
        }).detach();
    } catch (...) { result->done = true; }
    return result;
}
struct Socket {
    int fd = -1;
    ~Socket() { if (fd >= 0) close(fd); }
    bool Open(int family) {
        if (fd >= 0) close(fd);
        std::lock_guard<std::mutex> lock(ExecForkMutex());
        fd = socket(family, SOCK_STREAM, 0);
        return fd >= 0 && fcntl(fd, F_SETFD, FD_CLOEXEC) == 0 && fcntl(fd, F_SETFL, O_NONBLOCK) == 0;
    }
};
bool Wait(int fd, short events, Clock::time_point deadline, const std::atomic<bool>* stop) {
    while (!Stopped(stop) && Clock::now() < deadline) {
        pollfd item = {fd, events, 0};
        const int result = poll(&item, 1, 25);
        if (result > 0) return (item.revents & (events | POLLHUP | POLLERR)) != 0 && !(item.revents & POLLNVAL);
        if (result < 0 && errno != EINTR) return false;
    }
    return false;
}
int Send(void* context, const unsigned char* bytes, std::size_t size) {
    const int fd = *static_cast<int*>(context);
    const auto count = send(fd, bytes, size, MSG_NOSIGNAL);
    if (count >= 0) return static_cast<int>(count);
    return errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR ? MBEDTLS_ERR_SSL_WANT_WRITE : MBEDTLS_ERR_SSL_INTERNAL_ERROR;
}
int Receive(void* context, unsigned char* bytes, std::size_t size) {
    const auto count = recv(*static_cast<int*>(context), bytes, size, 0);
    if (count >= 0) return static_cast<int>(count);
    return errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR ? MBEDTLS_ERR_SSL_WANT_READ : MBEDTLS_ERR_SSL_INTERNAL_ERROR;
}
int RandomBytes(void*, unsigned char* output, std::size_t size, std::size_t* written) {
    *written = 0;
    int fd;
    { std::lock_guard<std::mutex> lock(ExecForkMutex());
      fd = open("/dev/urandom", O_RDONLY);
      if (fd >= 0 && fcntl(fd, F_SETFD, FD_CLOEXEC) != 0) { close(fd); fd = -1; } }
    if (fd < 0) return MBEDTLS_ERR_ENTROPY_SOURCE_FAILED;
    while (*written < size) {
        const auto count = read(fd, output + *written, size - *written);
        if (count > 0) *written += static_cast<std::size_t>(count);
        else if (count == 0 || errno != EINTR) { close(fd); return MBEDTLS_ERR_ENTROPY_SOURCE_FAILED; }
    }
    close(fd); return 0;
}
struct Tls {
    mbedtls_ssl_context ssl;
    mbedtls_ssl_config config;
    mbedtls_x509_crt roots;
    mbedtls_entropy_context entropy;
    mbedtls_ctr_drbg_context random;
    Tls() {
        mbedtls_ssl_init(&ssl); mbedtls_ssl_config_init(&config); mbedtls_x509_crt_init(&roots);
        mbedtls_entropy_init(&entropy); mbedtls_ctr_drbg_init(&random);
    }
    ~Tls() {
        mbedtls_ssl_free(&ssl); mbedtls_ssl_config_free(&config); mbedtls_x509_crt_free(&roots);
        mbedtls_ctr_drbg_free(&random); mbedtls_entropy_free(&entropy);
    }
};
bool RetryTls(int result, int fd, Clock::time_point deadline, const std::atomic<bool>* stop) {
    if (result == MBEDTLS_ERR_SSL_WANT_READ) return Wait(fd, POLLIN, deadline, stop);
    if (result == MBEDTLS_ERR_SSL_WANT_WRITE) return Wait(fd, POLLOUT, deadline, stop);
    return false;
}
}

int ParseEgressResponse(const std::string& response, bool eof, std::string* body) {
    body->clear();
    if (response.size() > 8192) return -1;
    const auto headerEnd = response.find("\r\n\r\n");
    if (headerEnd == std::string::npos) return eof ? -1 : 0;
    std::istringstream headers(response.substr(0, headerEnd));
    std::string line;
    std::getline(headers, line);
    if (line.compare(0, 9, "HTTP/1.0 ") && line.compare(0, 9, "HTTP/1.1 ")) return -1;
    if (line.size() < 13 || line.substr(9, 3) != "200" || (line[12] != ' ' && line[12] != '\r')) return -1;
    bool hasLength = false, chunked = false;
    std::size_t length = 0;
    while (std::getline(headers, line)) {
        const auto colon = line.find(':');
        if (colon == std::string::npos) return -1;
        const auto name = Lower(Trim(line.substr(0, colon)));
        const auto value = Lower(Trim(line.substr(colon + 1)));
        if (name == "content-length") {
            if (hasLength || !Number(value, 10, &length) || length > 128) return -1;
            hasLength = true;
        } else if (name == "transfer-encoding") {
            if (chunked || value != "chunked") return -1;
            chunked = true;
        } else if (name == "content-encoding" && value != "identity") return -1;
    }
    if (chunked && hasLength) return -1;
    const auto data = response.substr(headerEnd + 4);
    if (chunked) {
        std::size_t offset = 0;
        for (;;) {
            const auto end = data.find("\r\n", offset);
            if (end == std::string::npos) return eof ? -1 : 0;
            const auto token = data.substr(offset, end - offset);
            std::size_t count;
            if (!Number(token.substr(0, token.find(';')), 16, &count) || count > 128 - body->size()) return -1;
            offset = end + 2;
            if (!count) {
                if (data.size() < offset + 2) return eof ? -1 : 0;
                if (data.compare(offset, 2, "\r\n") == 0) return 1;
                return data.find("\r\n\r\n", offset) != std::string::npos ? 1 : eof ? -1 : 0;
            }
            if (data.size() < offset + count + 2) return eof ? -1 : 0;
            if (data.compare(offset + count, 2, "\r\n")) return -1;
            body->append(data, offset, count); offset += count + 2;
        }
    }
    if (hasLength) {
        if (data.size() < length) return eof ? -1 : 0;
        if (data.size() > length) return -1;
        *body = data; return 1;
    }
    if (data.size() > 128) return -1;
    if (!eof) return 0;
    *body = data; return 1;
}

EgressResult NativeEgress(int family, const EgressEndpoint& endpoint, const std::atomic<bool>* stop) {
    if ((family != 4 && family != 6) || endpoint.host.empty() || endpoint.host.find_first_of("\r\n") != std::string::npos) return EgressResult("", "invalid_endpoint");
    const auto deadline = Clock::now() + std::chrono::milliseconds(endpoint.timeout_ms);
    const auto interrupted = [&]() { return EgressResult("", Stopped(stop) ? "cancelled" : "timeout"); };
    if (Stopped(stop)) return interrupted();
    const auto resolved = Resolve(endpoint, family);
    if (!resolved) return EgressResult("", "dns_busy");
    while (!resolved->done) {
        if (Stopped(stop) || Clock::now() >= deadline) return interrupted();
        std::this_thread::sleep_for(std::chrono::milliseconds(10));
    }
    if (resolved->addresses.empty()) return EgressResult("", "dns_failed");
    Socket socket;
    bool connected = false;
    for (const auto& address : resolved->addresses) {
        if (Stopped(stop) || Clock::now() >= deadline) return interrupted();
        if (!socket.Open(address.storage.ss_family)) continue;
        int result = connect(socket.fd, reinterpret_cast<const sockaddr*>(&address.storage), address.length);
        if (result == 0) { connected = true; break; }
        if (errno != EINPROGRESS) continue;
        const auto connectDeadline = std::min(deadline, Clock::now() + std::chrono::seconds(3));
        if (!Wait(socket.fd, POLLOUT, connectDeadline, stop)) continue;
        int error = 0; socklen_t size = sizeof(error);
        if (getsockopt(socket.fd, SOL_SOCKET, SO_ERROR, &error, &size) == 0 && !error) { connected = true; break; }
    }
    if (!connected) return Stopped(stop) || Clock::now() >= deadline ? interrupted() : EgressResult("", "connect_failed");
    Tls tls;
    const unsigned char personal[] = "router-probe-egress";
    if (mbedtls_entropy_add_source(&tls.entropy, RandomBytes, NULL, 32, MBEDTLS_ENTROPY_SOURCE_STRONG) != 0 ||
        mbedtls_ctr_drbg_seed(&tls.random, mbedtls_entropy_func, &tls.entropy, personal, sizeof(personal)) != 0 ||
        mbedtls_x509_crt_parse(&tls.roots, reinterpret_cast<const unsigned char*>(endpoint.roots.c_str()), endpoint.roots.size() + 1) != 0 ||
        mbedtls_ssl_config_defaults(&tls.config, MBEDTLS_SSL_IS_CLIENT, MBEDTLS_SSL_TRANSPORT_STREAM, MBEDTLS_SSL_PRESET_DEFAULT) != 0) return EgressResult("", "tls_setup_failed");
    mbedtls_ssl_conf_authmode(&tls.config, MBEDTLS_SSL_VERIFY_REQUIRED);
    mbedtls_ssl_conf_ca_chain(&tls.config, &tls.roots, NULL);
    mbedtls_ssl_conf_rng(&tls.config, mbedtls_ctr_drbg_random, &tls.random);
    if (mbedtls_ssl_setup(&tls.ssl, &tls.config) != 0 || mbedtls_ssl_set_hostname(&tls.ssl, endpoint.host.c_str()) != 0) return EgressResult("", "tls_setup_failed");
    mbedtls_ssl_set_bio(&tls.ssl, &socket.fd, Send, Receive, NULL);
    for (;;) {
        if (Stopped(stop) || Clock::now() >= deadline) return interrupted();
        const int result = mbedtls_ssl_handshake(&tls.ssl);
        if (!result) break;
        if (!RetryTls(result, socket.fd, deadline, stop)) {
            const auto verification = mbedtls_ssl_get_verify_result(&tls.ssl);
            const bool rejected = verification != 0 && verification != static_cast<std::uint32_t>(-1);
            return Stopped(stop) || Clock::now() >= deadline ? interrupted() : EgressResult("", rejected ? "tls_certificate_failed" : "tls_handshake_failed");
        }
    }
    const std::string request = "GET / HTTP/1.1\r\nHost: " + endpoint.host + "\r\nConnection: close\r\nAccept: text/plain\r\n\r\n";
    std::size_t sent = 0;
    while (sent < request.size()) {
        if (Stopped(stop) || Clock::now() >= deadline) return interrupted();
        const int count = mbedtls_ssl_write(&tls.ssl, reinterpret_cast<const unsigned char*>(request.data() + sent), request.size() - sent);
        if (count > 0) sent += static_cast<std::size_t>(count);
        else if (!RetryTls(count, socket.fd, deadline, stop)) return Stopped(stop) || Clock::now() >= deadline ? interrupted() : EgressResult("", "egress_request_failed");
    }
    std::string response, body;
    for (;;) {
        if (Stopped(stop) || Clock::now() >= deadline) return interrupted();
        unsigned char buffer[1024];
        const int count = mbedtls_ssl_read(&tls.ssl, buffer, sizeof(buffer));
        bool eof = count == 0 || count == MBEDTLS_ERR_SSL_PEER_CLOSE_NOTIFY;
        if (count > 0) response.append(reinterpret_cast<char*>(buffer), count);
        else if (!eof) {
            if (RetryTls(count, socket.fd, deadline, stop)) continue;
            return Stopped(stop) || Clock::now() >= deadline ? interrupted() : EgressResult("", "egress_request_failed");
        }
        const int parsed = ParseEgressResponse(response, eof, &body);
        if (parsed < 0) return EgressResult("", "invalid_http_response");
        if (parsed > 0) break;
    }
    body = Trim(body); unsigned char address[16];
    if (body.size() >= 64 || body.find('\0') != std::string::npos || inet_pton(family == 4 ? AF_INET : AF_INET6, body.c_str(), address) != 1) return EgressResult("", "invalid_ip");
    return EgressResult(body);
}

EgressResult NativeEgress(int family, const std::atomic<bool>* stop) {
    return NativeEgress(family, EgressEndpoint(family == 4 ? "api.ipify.org" : "api6.ipify.org", "443", EgressRoots), stop);
}
}
