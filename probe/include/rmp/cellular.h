#ifndef RMP_CELLULAR_H
#define RMP_CELLULAR_H
#include <atomic>
#include <chrono>
#include <cstdint>
#include <mutex>
#include <string>
#include <thread>
#include <vector>
namespace rmp {
struct CellularPlan { unsigned interval=30; };
bool ParseCellularPlan(const std::string&,CellularPlan*);
struct AtIdentity {
 std::string command,value,status="not_queried";
};
struct CellularPort {
 std::string path,device_key,status="pending",reason;
 bool selected=false;
 AtIdentity ati,imei;
 std::chrono::steady_clock::time_point sampled=std::chrono::steady_clock::now();
};
struct CellularObservation {
 std::string status="no_ports",reason;
 bool limited=false;
 std::vector<CellularPort> ports;
 std::chrono::steady_clock::time_point sampled=std::chrono::steady_clock::now();
};
// root is only injected by native tests; production always uses the host filesystem.
CellularObservation SampleCellular(const std::string& root,const std::atomic<bool>* stop,
 unsigned* cursor,unsigned timeout_ms=1500,unsigned round_ms=15000);
std::string CellularEvent(const CellularObservation&,std::uint64_t revision,unsigned interval,std::size_t limit);
std::string ParseIMEI(const std::string& text);
class CellularCollector {
public:
 CellularCollector(const CellularPlan&,std::uint64_t revision);
 ~CellularCollector();
 bool Next(std::size_t,std::string*);
 void SetRevision(std::uint64_t);
private:
 void Run();
 CellularPlan plan_;std::uint64_t revision_;
 std::atomic<bool> stop_{false};std::thread worker_;std::mutex mutex_;
 CellularObservation observation_;bool pending_=false,observed_=false;
};
}
#endif
