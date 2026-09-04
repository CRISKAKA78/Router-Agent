#include "rmp/client.h"

#include <cstdlib>
#include <fstream>
#include <iostream>
#include <random>
#include <sstream>
#include <string>
#include <sys/types.h>
#include <unistd.h>

namespace {

std::string Trim(const std::string& input) {
    const std::string whitespace = " \t\r\n";
    const std::size_t first = input.find_first_not_of(whitespace);
    if (first == std::string::npos) {
        return "";
    }
    const std::size_t last = input.find_last_not_of(whitespace);
    return input.substr(first, last - first + 1);
}

std::string ReadBootId() {
    std::ifstream input("/proc/sys/kernel/random/boot_id");
    std::string boot_id;
    if (input && std::getline(input, boot_id)) {
        boot_id = Trim(boot_id);
        if (!boot_id.empty()) {
            return boot_id;
        }
    }
    std::random_device random;
    std::ostringstream fallback;
    fallback << "probe-" << static_cast<unsigned long>(getpid())
             << '-' << random() << '-' << random();
    return fallback.str();
}

std::string Hostname() {
    char buffer[256];
    if (gethostname(buffer, sizeof(buffer)) != 0) {
        return "";
    }
    buffer[sizeof(buffer) - 1] = '\0';
    return std::string(buffer);
}

std::string Architecture() {
#if defined(__x86_64__) || defined(_M_X64)
    return "x86_64";
#elif defined(__aarch64__)
    return "aarch64";
#elif defined(__arm__)
    return "arm";
#elif defined(__mips__)
#if defined(__MIPSEL__)
    return "mipsel";
#else
    return "mips";
#endif
#else
    return "unknown";
#endif
}

void Usage(const char* program) {
    std::cout << "Usage: " << program
              << " --device-id ID [--server HOST:PORT] [--probe-version VERSION]"
                 " [--arch ARCH] [--boot-id ID] [--hostname NAME]"
              << std::endl;
}

bool NextValue(int argc, char** argv, int* index, std::string* value) {
    if (*index + 1 >= argc) {
        return false;
    }
    ++*index;
    *value = argv[*index];
    return true;
}

}  // namespace

int main(int argc, char** argv) {
    std::string server_address = "127.0.0.1:9000";
    rmp::ClientConfig config;
    config.probe_version = "0.1.0";
    config.hostname = Hostname();
    config.arch = Architecture();
    config.boot_id = ReadBootId();

    for (int index = 1; index < argc; ++index) {
        const std::string argument = argv[index];
        std::string* destination = NULL;
        if (argument == "--server") {
            destination = &server_address;
        } else if (argument == "--device-id") {
            destination = &config.device_id;
        } else if (argument == "--probe-version") {
            destination = &config.probe_version;
        } else if (argument == "--arch") {
            destination = &config.arch;
        } else if (argument == "--boot-id") {
            destination = &config.boot_id;
        } else if (argument == "--hostname") {
            destination = &config.hostname;
        } else if (argument == "--help" || argument == "-h") {
            Usage(argv[0]);
            return 0;
        } else {
            std::cerr << "unknown argument: " << argument << std::endl;
            Usage(argv[0]);
            return 2;
        }
        if (!NextValue(argc, argv, &index, destination)) {
            std::cerr << argument << " requires a value" << std::endl;
            return 2;
        }
    }

    if (config.device_id.empty() || config.device_id.size() > 128 ||
        config.probe_version.empty() || config.probe_version.size() > 64 ||
        config.arch.empty() || config.arch.size() > 32 ||
        config.boot_id.empty() || config.boot_id.size() > 128 ||
        config.hostname.size() > 255) {
        std::cerr << "Probe identity fields are missing or outside Protocol v1 ranges" << std::endl;
        return 2;
    }

    std::string address_error;
    if (!rmp::ParseServerAddress(server_address, &config.server_host, &config.server_port, &address_error)) {
        std::cerr << address_error << std::endl;
        return 2;
    }
    return rmp::RunClient(config);
}
