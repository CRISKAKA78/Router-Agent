#include "rmp/egress.h"
#include "rmp/json.h"
#include "rmp/egress_roots.h"
#include <mbedtls/x509_crt.h>
#include <fstream>
#include <iostream>
#include <iterator>
#include <stdexcept>
#include <thread>
#include <chrono>
#include <cstdlib>

static void Check(bool condition, const char* message) { if (!condition) throw std::runtime_error(message); }
int main(int argc, char** argv) {
    try {
        if (argc >= 5) {
            std::ifstream file(argv[3]);
            const std::string roots((std::istreambuf_iterator<char>(file)), std::istreambuf_iterator<char>());
            std::atomic<bool> stop(false);
            std::thread cancel;
            if (argc >= 7) cancel = std::thread([&stop, argv]() { std::this_thread::sleep_for(std::chrono::milliseconds(std::atoi(argv[6]))); stop = true; });
            const auto result = rmp::NativeEgress(std::atoi(argv[4]), rmp::EgressEndpoint(argv[1], argv[2], roots, argc >= 6 ? std::atoi(argv[5]) : 2000), &stop);
            if (cancel.joinable()) cancel.join();
            std::cout << "{\"value\":" << rmp::EscapeJsonString(result.value) << ",\"reason\":" << rmp::EscapeJsonString(result.reason) << "}" << std::endl;
            return 0;
        }
        std::string body;
        mbedtls_x509_crt trust;
        mbedtls_x509_crt_init(&trust);
        const int parsed = mbedtls_x509_crt_parse(&trust, reinterpret_cast<const unsigned char*>(rmp::EgressRoots), sizeof(rmp::EgressRoots));
        mbedtls_x509_crt_free(&trust);
        Check(parsed == 0, "every production trust root is supported by the minimal TLS build");
        const std::string header = "HTTP/1.1 200 OK\r\nContent-Length: 9\r\n\r\n";
        Check(rmp::ParseEgressResponse(header + "192.0", false, &body) == 0, "partial body");
        Check(rmp::ParseEgressResponse(header + "192.0", true, &body) == -1, "truncated body");
        Check(rmp::ParseEgressResponse(header + "192.0.2.1", false, &body) == 1 && body == "192.0.2.1", "content length response");
        Check(rmp::ParseEgressResponse("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n4\r\n192.\r\n5\r\n0.2.1\r\n0\r\n\r\n", false, &body) == 1 && body == "192.0.2.1", "chunked response");
        Check(rmp::ParseEgressResponse("HTTP/1.0 200 OK\r\n\r\n2001:db8::1\n", true, &body) == 1, "close-delimited ipv6");
        for (const auto& invalid : {
            std::string("HTTP/1.1 302 Found\r\nContent-Length: 0\r\n\r\n"),
            std::string("HTTP/1.1 200 OK\r\nContent-Length: 0\r\nContent-Length: 0\r\n\r\n"),
            std::string("HTTP/1.1 200 OK\r\nContent-Length: 0\r\nTransfer-Encoding: chunked\r\n\r\n0\r\n\r\n"),
            std::string("HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\nxyz\r\n"),
            std::string("HTTP/1.1 200 OK\r\nContent-Encoding: gzip\r\n\r\n"),
            std::string("HTTP/1.1 200 OK\r\nContent-Length: 99999999999999999999\r\n\r\n"),
            std::string(8193, 'a') }) Check(rmp::ParseEgressResponse(invalid, true, &body) == -1, "invalid bounded HTTP response");
        std::atomic<bool> stopped(true);
        Check(rmp::NativeEgress(4, rmp::EgressEndpoint("localhost", "443", ""), &stopped).reason == "cancelled", "cancel before resolution");
        std::cout << "native egress parser and cancellation passed" << std::endl;
        return 0;
    } catch (const std::exception& error) { std::cerr << error.what() << std::endl; return 1; }
}
