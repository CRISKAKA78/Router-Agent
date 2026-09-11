#ifndef RMP_NETWORK_AGENT_H
#define RMP_NETWORK_AGENT_H
#include "rmp/task.h"
namespace rmp {
bool ParseNetworkAgent(const std::string& input, std::map<std::string,std::string>* params, std::string* error);
ExecResult ExecuteNetworkAgent(const ExecTask& task, const std::atomic<bool>* stop);
}
#endif
