#include "rmp/system_info.h"

#include <fstream>
#include <limits>
#include <sstream>
#include <sys/sysinfo.h>
#include <sys/utsname.h>

namespace rmp {
namespace {
std::string BuildArchitecture() {
#if defined(__x86_64__)
    return "x86_64";
#elif defined(__i386__)
    return "x86";
#elif defined(__aarch64__)
    return "aarch64";
#elif defined(__arm__)
    return "arm";
#elif defined(__mips__)
    const std::uint16_t word = 1;
    return *reinterpret_cast<const unsigned char*>(&word) == 1 ? "mipsel" : "mips";
#else
    return "unknown";
#endif
}
}

std::string ResolveArchitecture(const std::string& machine, const std::string& target,
                                unsigned pointer_bits, bool little_endian) {
    if (!target.empty() && target != "unknown") return target;
    if (pointer_bits == 64 && (machine == "x86_64" || machine == "amd64")) return "x86_64";
    if (pointer_bits == 64 && (machine == "aarch64" || machine == "arm64")) return "aarch64";
    if (pointer_bits == 32) {
        if (machine == "i386" || machine == "i486" || machine == "i586" || machine == "i686") return "x86";
        if (machine == "arm" || machine == "armv6l" || machine == "armv7l" || machine == "armv8l") return "arm";
        if (machine == "mips" || machine == "mipsel") return little_endian ? "mipsel" : "mips";
    }
    return "unknown";
}

SystemInfo ReadSystemInfo() {
    struct utsname info;
    const bool valid = uname(&info) == 0;
    const std::uint16_t word = 1;
    SystemInfo result;
    result.arch = ResolveArchitecture(valid ? info.machine : "", BuildArchitecture(),
        sizeof(void*) * 8, *reinterpret_cast<const unsigned char*>(&word) == 1);
    if (valid) {
        const std::string release(info.release);
        // Kernel release is an identifier; reject invalid/control bytes instead of truncating.
        bool printable = !release.empty() && release.size() <= 128;
        for (std::size_t i = 0; i < release.size(); ++i)
            if (static_cast<unsigned char>(release[i]) < 32 || static_cast<unsigned char>(release[i]) > 126) printable = false;
        if (printable) result.kernel = release;
    }
    return result;
}

bool ParseProcUptime(const std::string& text, std::uint64_t* seconds) {
    // Parse the first decimal field without float rounding or locale dependence.
    std::size_t i = 0;
    std::uint64_t value = 0;
    while (i < text.size() && text[i] >= '0' && text[i] <= '9') {
        const unsigned digit = static_cast<unsigned>(text[i++] - '0');
        if (value > (static_cast<std::uint64_t>(INT64_MAX) - digit) / 10) return false;
        value = value * 10 + digit;
    }
    if (i == 0) return false;
    if (i < text.size() && text[i] == '.') {
        const std::size_t start = ++i;
        while (i < text.size() && text[i] >= '0' && text[i] <= '9') ++i;
        if (i == start) return false;
    }
    if (i < text.size() && text[i] != ' ' && text[i] != '\t' && text[i] != '\n') return false;
    *seconds = value;
    return true;
}

SystemUptime ReadSystemUptime() {
    struct sysinfo info;
    SystemUptime result;
    if (sysinfo(&info) == 0 && info.uptime >= 0) {
        result.valid = true;
        result.seconds = static_cast<std::uint64_t>(info.uptime);
        return result;
    }
    std::ifstream input("/proc/uptime");
    char line[128];
    if (input.getline(line, sizeof(line)) && ParseProcUptime(line, &result.seconds)) result.valid = true;
    return result;
}

std::string HeartbeatPayload(unsigned running_tasks, const SystemUptime& uptime) {
    std::ostringstream output;
    output << "{\"uptime\":" << (uptime.valid ? uptime.seconds : 0)
           << ",\"uptime_valid\":" << (uptime.valid ? "true" : "false")
           << ",\"running_tasks\":" << running_tasks << '}';
    return output.str();
}
}  // namespace rmp
