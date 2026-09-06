#ifndef RMP_CLIENT_H
#define RMP_CLIENT_H

#include <cstdint>
#include <cstddef>
#include <string>

namespace rmp {

struct ClientConfig {
    std::size_t tunnel_connections = 64;
    std::size_t file_queue_capacity = 8;
    unsigned task_workers = 4;
    std::size_t task_capacity = 128;
    std::size_t task_cache_bytes = 8U * 1024U * 1024U;
    std::string server_host;
    std::string server_port;
    std::string device_id;
    std::string probe_version;
    std::string hostname;
    std::string arch;
    std::string boot_id;
};

bool ParseServerAddress(const std::string& address,
                        std::string* host,
                        std::string* port,
                        std::string* error);

int RunClient(const ClientConfig& config);

}  // namespace rmp

#endif
