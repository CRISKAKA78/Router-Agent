#ifndef RMP_TEST_TEMPLATE_FIXTURE_H
#define RMP_TEST_TEMPLATE_FIXTURE_H
#include "rmp/telemetry.h"
#include "rmp/json.h"
#include <sstream>
#include <stdexcept>
#include <unistd.h>
// Exercise the current configuration collector, including its asynchronous queue.
static rmp::Metrics CollectCurrent(const rmp::CollectionTemplate& t,rmp::ClientConfig config){
 std::ostringstream body;body<<"{\"template_id\":\"fixture\",\"version\":1,\"name\":\"Fixture\",\"properties\":{";bool comma=false;std::size_t expected=0;
 for(const auto& p:t.properties){if(p.first=="hostname"&&config.explicit_hostname)continue;++expected;if(comma)body<<',';comma=true;
  body<<rmp::EscapeJsonString(p.first)<<":{\"name\":"<<rmp::EscapeJsonString(p.second.name)<<",\"timeout_seconds\":"<<p.second.timeout;
  if(p.second.config.empty())body<<",\"command\":"<<rmp::EscapeJsonString(p.second.command);
  else body<<",\"source\":"<<rmp::EscapeJsonString(p.second.config.at("backend"))<<",\"key\":"<<rmp::EscapeJsonString(p.second.config.at("key"));
  body<<'}';
 }body<<"}}";config.collection_json=body.str();config.config_revision=1;config.monitoring={{"cpu",0},{"memory",0},{"disk",0},{"network",0},{"egress",0}};
 rmp::TelemetryCollector collector(config);rmp::Metrics result;
 for(int i=0;i<400&&result.size()<expected;++i){std::string event,error;if(collector.Next(65536,&event)){rmp::JsonObject root,values;if(!rmp::ParseJsonObject(event,&root,&error))throw std::runtime_error(error);
  if(root["group"].string_value=="template"){rmp::ParseJsonObject(root["values"].raw_value,&values,&error);for(const auto& p:values){rmp::JsonObject value;rmp::ParseJsonObject(p.second.raw_value,&value,&error);auto&m=result[p.first];m.value=value["value"].string_value;m.status=value["status"].string_value;m.reason=value["reason"].string_value;}}
 }usleep(20000);}
 if(result.size()!=expected)throw std::runtime_error("current template sampling deadline");return result;
}
#endif
