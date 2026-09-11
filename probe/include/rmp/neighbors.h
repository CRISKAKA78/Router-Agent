#ifndef RMP_NEIGHBORS_H
#define RMP_NEIGHBORS_H
#include "rmp/task.h"
#include <chrono>
#include <memory>
#include <thread>
#include <vector>
namespace rmp {
struct NeighborDomain {
  std::string id, scope, interface, lease_file;
  std::vector<std::string> ports;
};
struct NeighborPlan {
  unsigned interval = 30;
  std::vector<NeighborDomain> domains;
  std::string fdb_command, fdb_preset;
};
struct NeighborRow {
  std::string ip, mac, port, hostname, source, state, interface;
 long active_age_ms = -1;
};
struct NeighborNetwork {
 std::string interface, master, reason;
 bool bridge=false, vlan=false, eligible=false;
 std::vector<std::string> ipv4, ports;
};
std::vector<NeighborNetwork> DiscoverNeighborNetworks(const std::string& root="");
bool ParseFNR100ARL(const std::string&, std::vector<NeighborRow>*);
bool FNR100Environment(const std::vector<NeighborNetwork>&);
bool ReadFNR100(const std::string&,const std::atomic<bool>*,std::vector<NeighborRow>*,std::string*);
ExecResult InspectNeighbors(const ExecTask&,const std::atomic<bool>*);
bool ParseNeighborInspect(const std::string&,RouterConfigParams*);
bool ParseNeighborPlan(const std::string &, NeighborPlan *);
bool ParseNeighborTask(const std::string &, bool, RouterConfigParams *);
bool NeighborRange(const std::string &, std::uint32_t *, std::uint32_t *);
std::vector<NeighborRow> ParseARP(const std::string &, const std::string &);
// One process-owned sampler. Configuration/session changes invalidate both
// passive observations and ongoing scans; no worker owns the control socket.
class Neighbors {
public:
  explicit Neighbors(const std::string &root = "");
  ~Neighbors();
  void Configure(const std::string &, std::uint64_t);
  bool Next(std::size_t, std::string *);
  ExecResult Scan(const ExecTask &, const std::shared_ptr<std::atomic<bool>> &);

private:
  void Run();
  std::string root_, json_, latest_;
  NeighborPlan plan_;
  std::uint64_t revision_ = 0;
  bool pending_ = false, refresh_ = false;
  std::atomic<bool> stop_{false};
  std::thread worker_;
  std::mutex mutex_;
  std::shared_ptr<std::atomic<bool>> collecting_, active_;
  struct Recent {
    std::string domain;
    NeighborRow row;
    std::chrono::steady_clock::time_point at;
  };
  std::vector<Recent> recent_;
  std::chrono::steady_clock::time_point sampled_;
};
} // namespace rmp
#endif
