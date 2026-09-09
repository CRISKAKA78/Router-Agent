#ifndef RMP_SWITCH_PROBE_H
#define RMP_SWITCH_PROBE_H
#include "rmp/telemetry.h"
namespace rmp {
bool ValidateSwitchProbe(const std::string& json);
Metrics CollectSwitchPorts(const std::string& json,const std::atomic<bool>* stop,const std::string& root="");
Metrics ParseSwitchRows(const std::string& text);
}
#endif
