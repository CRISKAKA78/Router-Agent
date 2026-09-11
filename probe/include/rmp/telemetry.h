#ifndef RMP_TELEMETRY_H
#define RMP_TELEMETRY_H
#include "rmp/collection.h"
#include "rmp/cellular.h"
#include "rmp/neighbors.h"
#include <atomic>
#include <chrono>
#include <map>
#include <mutex>
#include <thread>
#include <vector>
#include <memory>
namespace rmp {
class PortCounterSampler;
struct Metric {
 std::string name,value,unit,status,entity,reason;
 unsigned interval=0;
 std::chrono::steady_clock::time_point sampled=std::chrono::steady_clock::now();
};
typedef std::map<std::string,Metric> Metrics;
std::string MetricUnit(const std::string& key);
Metrics HardwareMetrics(const std::string& root="");
class SystemSampler {
 public:
 explicit SystemSampler(const std::string& root="",const std::vector<std::string>& names={}):root_(root),names_(names){}
 std::shared_ptr<Neighbors> NeighborCollector(){std::lock_guard<std::mutex> lock(neighbor_mutex_);if(!neighbors_)neighbors_=std::make_shared<Neighbors>(root_);return neighbors_;}
 Metrics Sample(const std::string& group);
 Metrics SampleSwitch(const std::string&,const std::atomic<bool>*);
 void DisableSwitchCounters();
 void SetInterfaces(const std::vector<std::string>& names){names_=names;}
 void StartHardware(unsigned seconds);
 Metrics Hardware();
 unsigned HardwareAttempts();
 Metrics StaticProperties(const std::string& templateBytes);
 void SaveStaticProperty(const std::string&,const std::string&,const Metric&);
 ~SystemSampler();
 private:
 std::mutex template_mutex_;std::string static_template_;Metrics static_properties_;
 std::mutex hardware_mutex_;Metrics hardware_;unsigned hardware_attempts_=0;
 std::atomic<bool> hardware_stop_{false};std::thread hardware_worker_;
 std::string root_;
 std::vector<std::string> names_;
 std::uint64_t cpu_total_=0,cpu_idle_=0;
 bool cpu_valid_=false;
 struct Net {std::uint64_t rx,tx;std::string index;std::chrono::steady_clock::time_point at;std::uint64_t start_rx=0,start_tx=0;std::chrono::steady_clock::time_point start;};
 std::map<std::string,Net> networks_;
 std::shared_ptr<PortCounterSampler> ports_;
 std::mutex neighbor_mutex_;std::shared_ptr<Neighbors> neighbors_;
};
// The control thread alone drains coalesced observations and owns wire sends.
class TelemetryCollector {
 public:
 TelemetryCollector(const ClientConfig& config,SystemSampler* sampler=NULL);
 ~TelemetryCollector();
 bool Next(std::size_t max_payload,std::string* payload);
 void ReconfigureNetwork(const ClientConfig&);
private:
 void Builtins();void Network();void StartNetwork();void Templates();void Egress();void Switches();void Publish(const std::string&,const Metrics&);
 ClientConfig config_;std::atomic<bool> stop_,network_stop_{false};std::uint64_t revision_;unsigned network_seconds_;
 std::unique_ptr<CellularCollector> cellular_;
 std::thread builtin_,network_,templates_,egress_,switches_;
 SystemSampler owned_sampler_;SystemSampler* sampler_;
 std::mutex mutex_;std::map<std::string,Metrics> pending_,latest_;
};
}
#endif
