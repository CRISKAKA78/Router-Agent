#include "rmp/neighbors.h"
#include "rmp/task_manager.h"
#include <iostream>
#include <stdexcept>
#include <thread>
using namespace rmp;
static void Check(bool ok, const char *why) {
  if (!ok)
    throw std::runtime_error(why);
}
int main() {
  try {
    NeighborPlan p;
    std::vector<NeighborRow> arl;
    Check(ParseFNR100ARL("MAC: 02:00:00:00:00:01 PORTMAP: 0x01 VID: 0x3 STATUS: 0x0\nMAC: 02:00:00:00:00:02 PORTMAP: 0x3e VID: 0x3 STATUS: 0x0\n", &arl)&&arl.size()==5&&arl[0].port=="lan1"&&arl[4].port=="wan","FNR100 external ports include WAN, exclude CPU");
    Check(!ParseFNR100ARL("MAC: 02:00:00:00:00:01 PORTMAP: 0x40 VID: 0x3 STATUS: 0x0",&arl)&&arl.empty(),"unknown port bits fail closed");
    Check(!ParseFNR100ARL("bad output",&arl)&&!ParseFNR100ARL("",&arl),"malformed and empty output not verified");
    Check(ParseFNR100ARL("MAC: 02:00:00:00:00:01 PORTMAP: 0x20 VID: 0x2 STATUS: 0x0",&arl)&&arl.empty(),"other VLAN not leaked into br0");
    RouterConfigParams inspection;
    Check(ParseNeighborInspect(R"({"session_id":"s-a","config_revision":7,"vendor_test":false})", &inspection) &&
              inspection["session_id"] == "s-a" && inspection["config_revision"] == "7" && inspection["vendor_test"] == "false",
          "inspection preserves full Session/revision identity for duplicate-task comparison");
    Check(!ParseNeighborInspect(R"({"session_id":"s-a","config_revision":7})", &inspection) &&
              !ParseNeighborInspect(R"({"session_id":"s-a","config_revision":7,"vendor_test":"false"})", &inspection),
          "inspection requires explicit boolean and complete scoped parameters");
    NeighborNetwork bridge;bridge.interface="br0";bridge.bridge=true;bridge.eligible=true;bridge.ports={"eth0","vlan3","ath0","ath1"};
    Check(FNR100Environment({bridge}),"validated bridge profile");bridge.ports.pop_back();Check(!FNR100Environment({bridge}),"model name alone cannot validate environment");
    Check(ParseNeighborPlan(R"({"fdb_preset":"fnr100","domains":[{"id":"local","scope":"broadcast","interface":"br0"}]})",&p),"preset parsed");
    Check(!ParseNeighborPlan(R"({"fdb_preset":"fnr100","fdb_command":"echo x","domains":[{"id":"local","scope":"broadcast","interface":"br0"}]})",&p),"conflicting command rejected");
    Check(
        ParseNeighborPlan(
            R"({"domains":[{"id":"lan","scope":"lan","interface":"br0","ports":["LAN1"]},{"id":"local","scope":"broadcast","interface":"br0"}]})",
            &p),
        "overlapping views");
    Check(!ParseNeighborPlan(
              R"({"domains":[{"id":"lan","scope":"lan","interface":"br0"}]})",
              &p),
          "LAN requires port evidence");
    Check(!ParseNeighborPlan(
              R"({"domains":[{"id":"a","scope":"uplink","interface":"eth0"}]})",
              &p),
          "uplink removed");
    Check(
        !ParseNeighborPlan(
            R"({"domains":[{"id":"a","scope":"broadcast","interface":"../x"}]})",
            &p),
        "bad interface");
    Check(
        !ParseNeighborPlan(
            R"({"domains":[1,{"id":"local","scope":"broadcast","interface":"br0"}]})",
            &p),
        "mixed domain array rejected");
    std::uint32_t first, last;
    Check(NeighborRange("192.0.2.0/24", &first, &last) && last - first == 255,
          "bounded range");
    Check(!NeighborRange("192.0.0.0/16", &first, &last), "large scan rejected");
    Check(!NeighborRange("192.0.2.1/24", &first, &last),
          "noncanonical rejected");
    auto rows =
        ParseARP("192.0.2.2 0x1 0x2 02:00:00:00:00:02 * br0\n192.0.2.3 0x1 0x0 "
                 "00:00:00:00:00:00 * br0\n192.0.2.4 0x1 0x2 02:00:00:00:00:04 "
                 "* eth0\n192.0.2.2 0x1 0x2 02:00:00:00:00:02 * br0\n",
                 "br0");
    Check(rows.size() == 1 && rows[0].state == "cached",
          "ARP isolation/incomplete/dedup");
    auto collector = std::make_shared<Neighbors>();
    collector->Configure(
        R"({"domains":[{"id":"local","scope":"broadcast","interface":"missing999"}]})",
        1);
    std::string sample;
    for (int i = 0; i < 80 && !collector->Next(65536, &sample); ++i)
      std::this_thread::sleep_for(std::chrono::milliseconds(25));
    Check(sample.find("interface_missing") != std::string::npos,
          "missing interface explicit");
    collector->Configure("", 0);
    Check(!collector->Next(65536, &sample), "disabled snapshot removed");
    TaskManager manager(1);
    manager.SetNeighbors(collector);
    ExecTask block;
    block.task_id = "block";
    block.type = "exec";
    block.command = "sleep 0.3";
    block.timeout = 2;
    Check(manager.Submit(block, true, 100, 65536) == "queued",
          "worker blocker");
    ExecTask scan;
    std::string error;
    Check(
        ParseTask(
            R"({"task_id":"scan","type":"neighbor_scan","timeout":30,"params":{"domain_id":"local","cidr":"192.0.2.0/24","config_revision":1}})",
            &scan, &error),
        "parse scan");
    Check(manager.Submit(scan, true, 200, 65536) == "queued", "scan queued");
    ExecTask cancel;
    Check(
        ParseTask(
            R"({"task_id":"cancel","type":"neighbor_cancel","timeout":30,"params":{"target_task_id":"scan"}})",
            &cancel, &error),
        "parse cancel");
    Check(manager.Submit(cancel, true, 100, 65536) == "success",
          "cancel does not wait for occupied worker");
    Check(manager.CachedResult("cancel", 65536, &sample) &&
              sample.find("stop_requested") != std::string::npos,
          "cancel result cached");
    Check(manager.Submit(cancel, true, 100, 65536) == "success",
          "cancel idempotent");
    std::cout << "neighbor tests passed\n";
    return 0;
  } catch (const std::exception &e) {
    std::cerr << e.what() << "\n";
    return 1;
  }
}
