#ifndef RMP_CLIENT_H
#define RMP_CLIENT_H

#include <cstdint>
#include <cstddef>
#include <string>
#include <map>
#include <vector>

namespace rmp {

struct ClientConfig {
 std::uint64_t config_revision=0;
 std::uint64_t template_generation=0;
 std::string switch_json;
 std::string neighbor_json;
 std::vector<std::string> default_network_interfaces;
 std::vector<std::string> network_interfaces;
 bool explicit_network_interfaces=false;
 std::map<std::string,std::string> builtin_errors;
 std::map<std::string,unsigned> monitoring = {{"cpu",5},{"memory",5},{"disk",60},{"network",5},{"egress",600}};
 std::map<std::string,unsigned> monitoring_overrides;
 std::string collection_json;
    std::map<std::string,std::string> properties;
    bool explicit_hostname = false;
    std::size_t tunnel_connections = 8;
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
