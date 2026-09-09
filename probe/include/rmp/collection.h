#ifndef RMP_COLLECTION_H
#define RMP_COLLECTION_H
#include "rmp/client.h"
#include "rmp/router_config.h"
#include <map>
namespace rmp {
struct CollectionProperty { std::string name, command; unsigned timeout; unsigned interval; RouterConfigParams config; CollectionProperty(const std::string& n="",const std::string& c="",unsigned t=5,const RouterConfigParams& params=RouterConfigParams()):name(n),command(c),timeout(t),interval(0),config(params){} };
struct CollectionTemplate {
 std::map<std::string,unsigned> monitoring;
 std::string raw_json;
 std::string switch_json;
 std::vector<std::string> network_interfaces;
 bool has_network_interfaces=false;
 std::string id,name; std::uint64_t version;
 std::map<std::string,CollectionProperty> properties;
};
bool ParseCollectionTemplate(const std::string& json,CollectionTemplate* value,std::string* error);
bool ParseNetworkInterfaces(const std::string&,std::vector<std::string>*);
void CollectFirmware(ClientConfig*,const std::string& root="");
}
#endif
