#include "rmp/collection.h"
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
 const auto start=std::chrono::steady_clock::now();rmp::CollectProperties(value,&config);
 check(std::chrono::steady_clock::now()-start<std::chrono::seconds(8),"timeout failed");
 check(config.hostname=="manual"&&config.properties["model"]=="model"&&config.properties.count("libc")==0,"standard values");
 rmp::JsonObject attrs,errs,custom;
 check(rmp::ParseJsonObject(config.attributes,&attrs,&error)&&attrs.size()==1,"custom attributes");
 check(rmp::ParseJsonObject(attrs["custom"].raw_value,&custom,&error)&&custom["value"].string_value=="hello","trimmed output");
 check(rmp::ParseJsonObject(config.collection_errors,&errs,&error)&&errs.size()==5,"failure summary");
 check(config.collection_errors.find("secret")==std::string::npos,"stderr leak");
 config.explicit_hostname=false;value.properties.clear();value.properties["model"]={"Model","printf replacement",5};
 rmp::CollectProperties(value,&config);check(config.hostname.empty()&&config.attributes=="{}"&&config.collection_errors=="{}","old snapshot retained");
 return failures?1:0;
}
