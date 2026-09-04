#ifndef RMP_CLIENT_H
#define RMP_CLIENT_H

#include <cstdint>
#include <string>

namespace rmp {

struct ClientConfig {
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
