#ifndef RMP_PORT_COUNTERS_H
#define RMP_PORT_COUNTERS_H
#include "rmp/telemetry.h"

namespace rmp {
bool ValidatePortCounters(const std::string& config);
bool ParseMibBytes(const std::string&,const std::string&,const std::string&,std::uint64_t*,std::uint64_t*);
class PortCounterSampler {
public:
 Metrics Collect(const std::string&,const std::atomic<bool>*);
 // Exact integer subtraction precedes conversion to a rate. Public for deterministic fixtures.
 void Observe(Metrics&,const std::string& id,const std::string& source,std::uint64_t rx,std::uint64_t tx,
              unsigned bits,double speed,std::chrono::steady_clock::time_point at);
 void Clear(){previous_.clear();}
private:
 struct Sample {std::string source;std::uint64_t rx=0,tx=0,total_rx=0,total_tx=0;
  std::chrono::steady_clock::time_point at,start;};
 std::map<std::string,Sample> previous_;
};
}
#endif
