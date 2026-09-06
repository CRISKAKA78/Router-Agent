#ifndef RMP_TUNNEL_H
#define RMP_TUNNEL_H

#include "rmp/frame.h"
#include <atomic>
#include <functional>
#include <memory>
#include <string>
#include <thread>
#include <vector>

namespace rmp {
// Owned by one control Session. Only Feed/Tick run on the control reader.
class TunnelManager {
public:
    typedef std::function<bool(const std::string&)> Report;
    TunnelManager(const std::string& session, std::size_t limit, Report report);
    ~TunnelManager();
    bool Feed(const Frame& frame, std::string* error);
    bool Tick();
private:
    struct Worker;
    std::string session_;
    std::size_t limit_;
    Report report_;
    std::vector<std::shared_ptr<Worker> > workers_;
};
}
#endif
