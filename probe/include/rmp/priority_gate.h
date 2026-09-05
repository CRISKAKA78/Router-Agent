#ifndef RMP_PRIORITY_GATE_H
#define RMP_PRIORITY_GATE_H
#include <condition_variable>
#include <mutex>
namespace rmp {
class PriorityGate {
  public:
    PriorityGate() : busy_(false), controls_(0) {}
    void Lock(bool low) {
        std::unique_lock<std::mutex> lock(mutex_);
        if (!low)
            ++controls_;
        condition_.wait(lock, [this, low] { return !busy_ && (!low || controls_ == 0); });
        if (!low)
            --controls_;
        busy_ = true;
    }
    void Unlock() {
        std::lock_guard<std::mutex> lock(mutex_);
        busy_ = false;
        condition_.notify_all();
    }

  private:
    std::mutex mutex_;
    std::condition_variable condition_;
    bool busy_;
    unsigned controls_;
};
class PriorityGuard {
  public:
    PriorityGuard(PriorityGate &g, bool low) : gate_(g) { gate_.Lock(low); }
    ~PriorityGuard() { gate_.Unlock(); }

  private:
    PriorityGate &gate_;
};
} // namespace rmp
#endif
