#ifndef RMP_EGRESS_H
#define RMP_EGRESS_H

#include <atomic>
#include <string>

namespace rmp {
struct EgressResult {
    std::string value, reason;
    EgressResult(const std::string& address = "", const std::string& failure = "") : value(address), reason(failure) {}
};
struct EgressEndpoint {
    std::string host, port, roots;
    unsigned timeout_ms;
    EgressEndpoint(const std::string& hostname, const std::string& service, const std::string& ca, unsigned timeout = 6000)
        : host(hostname), port(service), roots(ca), timeout_ms(timeout) {}
};
EgressResult NativeEgress(int family, const std::atomic<bool>* stop);
EgressResult NativeEgress(int family, const EgressEndpoint&, const std::atomic<bool>* stop);
// Returns 1 for a complete response, 0 while incomplete, and -1 on invalid HTTP.
int ParseEgressResponse(const std::string&, bool eof, std::string* body);
}
#endif
