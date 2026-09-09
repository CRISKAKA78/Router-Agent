#include "rmp/system_info.h"
#include "rmp/json.h"
#include <fstream>
#include <iostream>
#include <sys/utsname.h>

int main() {
    int failures = 0;
    const auto check = [&](bool ok, const char* message) { if (!ok) { std::cerr << message << '\n'; ++failures; } };
    check(rmp::ResolveArchitecture("aarch64", "arm", 32, true) == "arm", "64-bit kernel must not override ARM32 build");
    check(rmp::ResolveArchitecture("x86_64", "x86", 32, true) == "x86", "64-bit kernel must not override x86 build");
    check(rmp::ResolveArchitecture("mips", "mipsel", 32, true) == "mipsel", "MIPS build endianness lost");
    check(rmp::ResolveArchitecture("mips", "unknown", 32, true) == "mipsel", "MIPS little endian fallback");
    check(rmp::ResolveArchitecture("mipsel", "unknown", 32, false) == "mips", "MIPS native endianness");
    check(rmp::ResolveArchitecture("armv7l", "unknown", 32, true) == "arm", "ARM generation must retain existing family");
    check(rmp::ResolveArchitecture("arm64", "unknown", 64, true) == "aarch64", "ARM64 alias");
    check(rmp::ResolveArchitecture("amd64", "unknown", 64, true) == "x86_64", "AMD64 alias");
    check(rmp::ResolveArchitecture("aarch64", "unknown", 32, true) == "unknown", "unsafe bitness fallback");
    check(rmp::ResolveArchitecture("future", "unknown", 64, true) == "unknown", "unknown architecture guessed");
    std::uint64_t seconds = 77;
    check(rmp::ParseProcUptime("1234.99 456.00", &seconds) && seconds == 1234, "proc uptime floor");
    check(rmp::ParseProcUptime("0.00 0.00", &seconds) && seconds == 0, "valid zero");
    check(rmp::ParseProcUptime("9223372036854775807.99 0", &seconds) && seconds == INT64_MAX, "max integer precision");
    for (const char* invalid : {"", "-1.00 2", "nan 0", "1e3 0", "1. 0", "1x 0", "9223372036854775808.00 0"})
        check(!rmp::ParseProcUptime(invalid, &seconds), "invalid proc uptime accepted");
    rmp::SystemUptime zero;
    zero.valid = true;
    rmp::JsonObject payload; std::string error;
    check(rmp::ParseJsonObject(rmp::HeartbeatPayload(3, zero), &payload, &error) && payload["uptime_valid"].raw_value == "true" && payload["uptime"].unsigned_value == 0, "true zero wire value");
    zero.valid = false; zero.seconds = 999;
    check(rmp::HeartbeatPayload(0, zero) == "{\"uptime\":0,\"uptime_valid\":false,\"running_tasks\":0}", "failure must not report stale value");
    const rmp::SystemInfo info = rmp::ReadSystemInfo(); struct utsname expected;
    check(uname(&expected) == 0 && info.kernel == expected.release && !info.arch.empty(), "real kernel collection");
    const auto uptime = rmp::ReadSystemUptime();
    std::ifstream proc("/proc/uptime"); std::string line; std::getline(proc, line);
    check(uptime.valid && rmp::ParseProcUptime(line, &seconds) && uptime.seconds + 2 >= seconds && seconds + 2 >= uptime.seconds, "real uptime differs from /proc/uptime");
    return failures ? 1 : 0;
}
