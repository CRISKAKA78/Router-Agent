#ifndef RMP_PENDING_HEARTBEATS_H
#define RMP_PENDING_HEARTBEATS_H

#include <cstdint>
#include <set>

namespace rmp {
// Session-local resource bound. Never evict an outstanding correlation: when
// full, the connection owner reconnects rather than rejecting a valid late ACK.
class PendingHeartbeats {
public:
    bool Full() const { return ids_.size() >= 1024; }
    bool Add(std::uint64_t id) {
        return id != 0 && !Full() && ids_.insert(id).second;
    }
    bool Acknowledge(std::uint64_t id) { return ids_.erase(id) == 1; }
private:
    std::set<std::uint64_t> ids_;
};
}
#endif
