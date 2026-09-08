#ifndef RMP_ROUTER_CONFIG_H
#define RMP_ROUTER_CONFIG_H
#include <map>
#include <string>
#include <vector>
namespace rmp {
// Presence matters: set with an empty value differs from an absent value.
typedef std::map<std::string, std::string> RouterConfigParams;
bool ParseRouterConfig(const std::string& json, RouterConfigParams* params, std::string* error);
bool RouterConfigArguments(const RouterConfigParams& params, std::vector<std::string>* args, std::string* error);
}
#endif
