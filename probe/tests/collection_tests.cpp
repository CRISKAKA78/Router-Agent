#include "template_fixture.h"
#include "rmp/json.h"
#include <iostream>
#include <chrono>

int main(){
 int failures=0;const auto check=[&](bool ok,const char* message){if(!ok){std::cerr<<message<<'\n';++failures;}};
 rmp::CollectionTemplate value;std::string error;
 check(rmp::ParseCollectionTemplate("{\"template_id\":\"id\",\"name\":\"router\",\"version\":1,\"properties\":{\"model\":{\"name\":\"model\",\"command\":\"printf model\",\"timeout_seconds\":5}}}",&value,&error),"parse template");
 for(const char* key:{"device_id","arch","capabilities","Bad","x-y"}){
  const std::string payload="{\"template_id\":\"id\",\"name\":\"router\",\"version\":1,\"properties\":{"+rmp::EscapeJsonString(key)+":{\"name\":\"x\",\"command\":\"true\",\"timeout_seconds\":5}}}";
  rmp::CollectionTemplate ignored;check(!rmp::ParseCollectionTemplate(payload,&ignored,&error),"protected/invalid field accepted");
 }
 value.properties["custom"]={"Custom","printf '  hello\\n'",5};
 value.properties["hostname"]={"Host","printf discovered",5};
 value.properties["empty"]={"Empty","printf ' '",5};
 value.properties["failure"]={"Failure","printf secret >&2; exit 3",5};
 value.properties["timeout"]={"Timeout","sleep 20",1};
 value.properties["invalid"]={"Invalid","printf '\\377'",5};
 value.properties["libc"]={"Libc","printf '\\303\\251'",5};
 rmp::ClientConfig config;config.hostname="manual";config.explicit_hostname=true;
 const auto start=std::chrono::steady_clock::now();auto metrics=CollectCurrent(value,config);
 check(std::chrono::steady_clock::now()-start<std::chrono::seconds(8),"timeout failed");
 check(config.hostname=="manual"&&metrics["model"].value=="model"&&metrics["libc"].status=="error","standard values");
 check(metrics["custom"].value=="hello","trimmed output");
 for(const auto& key:{"empty","failure","timeout","invalid","libc"})check(metrics[key].status=="error","failure summary");
 check(metrics["failure"].reason=="command_failed"&&metrics["failure"].value.empty(),"stderr leak");
 value.properties.clear();value.properties["model"]={"Model","printf replacement",5};
 metrics=CollectCurrent(value,config);check(metrics.size()==1&&metrics["model"].value=="replacement","configuration replaces previous collection");
 config.properties["kernel"]="6.6-vendor";
 metrics=CollectCurrent(value,config);check(!metrics.count("kernel")&&config.properties["kernel"]=="6.6-vendor","template without kernel preserves built-in fact");
 value.properties["kernel"]={"Kernel","printf 5.10-custom",5};
 metrics=CollectCurrent(value,config);check(metrics["kernel"].value=="5.10-custom","explicit kernel observation");
 value.properties["kernel"]={"Kernel","exit 3",5};metrics=CollectCurrent(value,config);
 check(metrics["kernel"].status=="error"&&metrics["kernel"].value.empty(),"failed kernel must be explicit");
 return failures?1:0;
}
