#ifndef RMP_COLLECTION_H
#define RMP_COLLECTION_H
#include "rmp/client.h"
#include "rmp/router_config.h"
#include <map>
namespace rmp {
struct CollectionProperty { std::string name, command; unsigned timeout; RouterConfigParams config; };
struct CollectionTemplate {
 std::string id,name; std::uint64_t version;
 std::uint32_t max_control_payload = 1024U*1024U;
 std::map<std::string,CollectionProperty> properties;
};
bool ParseCollectionTemplate(const std::string& json,CollectionTemplate* value,std::string* error);
void CollectProperties(const CollectionTemplate& value,ClientConfig* config);
}
#endif
