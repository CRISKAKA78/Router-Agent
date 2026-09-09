#ifndef RMP_LIVE_CONFIG_H
#define RMP_LIVE_CONFIG_H
#include "rmp/telemetry.h"
#include <memory>
namespace rmp {
// Configuration transitions run outside the control reader. The reader sends
// the acknowledgement before draining observations from the new collector.
class LiveTelemetry {
public:
 LiveTelemetry(const ClientConfig&,SystemSampler*);
 ~LiveTelemetry();
 bool Apply(const std::string&,std::uint64_t,std::string*);
 bool NextAck(std::string*);
 bool Next(std::size_t,std::string*);
private:
 void Run();
 ClientConfig initial_,pending_,applied_;bool requested_=false,ready_=false;
 std::atomic<bool> stop_{false};std::thread worker_;std::mutex mutex_;
 std::unique_ptr<TelemetryCollector> collector_;SystemSampler* sampler_;
 std::uint64_t revision_=0,pending_revision_=0,reply_=0;
 std::string applied_bytes_,pending_bytes_,ack_;
};
}
#endif
