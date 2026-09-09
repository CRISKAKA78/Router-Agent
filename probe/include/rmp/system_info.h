#ifndef RMP_SYSTEM_INFO_H
#define RMP_SYSTEM_INFO_H

#include <cstdint>
#include <string>

namespace rmp {

struct SystemInfo {
    std::string arch;
    std::string kernel;
};

struct SystemUptime {
    bool valid = false;
    std::uint64_t seconds = 0;
};

// Known build targets preserve the existing executable-architecture contract.
std::string ResolveArchitecture(const std::string& machine, const std::string& target,
                                unsigned pointer_bits, bool little_endian);
SystemInfo ReadSystemInfo();
bool ParseProcUptime(const std::string& text, std::uint64_t* seconds);
SystemUptime ReadSystemUptime();
std::string HeartbeatPayload(unsigned running_tasks, const SystemUptime& uptime);

}  // namespace rmp
#endif
