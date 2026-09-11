#include "rmp/client.h"
#include "rmp/identity.h"
#include "rmp/collection.h"
#ifndef RMP_NETWORK_INTERFACES
#define RMP_NETWORK_INTERFACES "br0,eth0,eth1,usb0"
#endif
#include "rmp/system_info.h"

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

void Usage(const char* program) {
    std::cout << "Usage: " << program
              << " [--device-id ID] [--server HOST:PORT] [--probe-version VERSION]"
                 " [--arch ARCH] [--boot-id ID] [--hostname NAME] [--tunnel-connections N]"
                 " [--cpu-interval SECONDS] [--memory-interval SECONDS] [--disk-interval SECONDS] [--network-interval SECONDS] [--network-interfaces eth0,br0] [--egress-interval SECONDS]"
              << "\nWithout --device-id, runs nvram get SN (5 second timeout)." << std::endl;
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
    std::string server_address = "47.119.168.150:9000";
    rmp::ClientConfig config;
    bool explicit_device_id = false;
    std::string tunnel_connections="8";
    config.probe_version = "0.2.0";
    if(!rmp::ParseNetworkInterfaces(RMP_NETWORK_INTERFACES,&config.network_interfaces))return 2;
    config.default_network_interfaces=config.network_interfaces;
    config.hostname = Hostname();
    const rmp::SystemInfo system_info = rmp::ReadSystemInfo();
    config.arch = system_info.arch;
    if (!system_info.kernel.empty()) config.properties["kernel"] = system_info.kernel;
    config.boot_id = ReadBootId();

    for (int index = 1; index < argc; ++index) {
        const std::string argument = argv[index];
        std::string* destination = NULL;
        if(argument=="--network-interfaces"){
            std::string value;if(!NextValue(argc,argv,&index,&value)||!rmp::ParseNetworkInterfaces(value,&config.network_interfaces)){std::cerr<<"invalid interface names (comma separated, exact names)"<<std::endl;return 2;}config.explicit_network_interfaces=true;continue;
        }
        if (argument=="--cpu-interval"||argument=="--memory-interval"||argument=="--disk-interval"||argument=="--network-interval"||argument=="--egress-interval") {
            std::string value;if(!NextValue(argc,argv,&index,&value)||value.empty()||value.find_first_not_of("0123456789")!=std::string::npos||value.size()>5){std::cerr<<"monitor interval must be 0-86400 seconds"<<std::endl;return 2;}
            unsigned n=static_cast<unsigned>(std::strtoul(value.c_str(),NULL,10));if(n>86400)return 2;
            std::string group=argument.substr(2,argument.size()-11);config.monitoring[group]=n;config.monitoring_overrides[group]=n;continue;
        } else if (argument == "--server") {
            destination = &server_address;
        } else if (argument == "--tunnel-connections") {
            destination = &tunnel_connections;
        } else if (argument == "--device-id" || argument == "--device_id") {
            explicit_device_id = true;
            destination = &config.device_id;
        } else if (argument == "--probe-version") {
            destination = &config.probe_version;
        } else if (argument == "--arch") {
            destination = &config.arch;
        } else if (argument == "--boot-id") {
            destination = &config.boot_id;
        } else if (argument == "--hostname") {
            config.explicit_hostname = true;
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

    char* tail=NULL;unsigned long tunnel_limit=std::strtoul(tunnel_connections.c_str(),&tail,10);
    if(tunnel_connections.empty()||tail==NULL||*tail!='\0'||tunnel_limit<1||tunnel_limit>64){std::cerr<<"tunnel-connections must be 1-64"<<std::endl;return 2;}
    config.tunnel_connections=static_cast<std::size_t>(tunnel_limit);
    if (config.probe_version.empty() || config.probe_version.size() > 64 ||
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
    if (!rmp::ResolveDeviceId(explicit_device_id, &config.device_id, &address_error)) {
        std::cerr << address_error << std::endl;
        return 2;
    }
    return rmp::RunClient(config);
}
