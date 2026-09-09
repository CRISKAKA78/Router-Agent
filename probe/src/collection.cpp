#include "rmp/collection.h"
#include "rmp/json.h"
#include "rmp/task.h"
#include "rmp/switch_probe.h"
#include <sstream>
#include <algorithm>
#include <fstream>

namespace rmp {
namespace {
bool Text(const JsonObject& o,const std::string& key,std::size_t limit,std::string* out){
 JsonObject::const_iterator i=o.find(key);if(i==o.end()||i->second.type!=JsonType::kString)return false;
 *out=i->second.string_value;return !out->empty()&&out->size()<=limit&&out->find('\0')==std::string::npos;
}
bool Number(const JsonObject&o,const std::string& key,std::uint64_t min,std::uint64_t max,std::uint64_t*out){
 JsonObject::const_iterator i=o.find(key);if(i==o.end()||i->second.type!=JsonType::kUnsignedInteger)return false;
 *out=i->second.unsigned_value;return *out>=min&&*out<=max;
}
std::size_t StandardLimit(const std::string& k){if(k=="serial"||k=="model"||k=="firmware"||k=="kernel")return 128;if(k=="hostname")return 255;if(k=="libc")return 64;return 0;}
bool Key(const std::string& k){
 if(k.empty()||k.size()>64||k[0]<'a'||k[0]>'z')return false;
 for(std::size_t i=0;i<k.size();++i)if(!((k[i]>='a'&&k[i]<='z')||(k[i]>='0'&&k[i]<='9')||k[i]=='_'))return false;
 return k!="device_id"&&k!="arch"&&k!="boot_id"&&k!="probe_version"&&k!="capabilities"&&k!="template"&&k!="attributes"&&k!="collection_errors";
}
std::string Trim(const std::string&s){const std::size_t first=s.find_first_not_of(" \t\r\n"),last=s.find_last_not_of(" \t\r\n");return first==std::string::npos?"":s.substr(first,last-first+1);}
}
bool ParseNetworkInterfaces(const std::string& text,std::vector<std::string>* names){
 names->clear();if(text.empty())return true;
 std::istringstream in(text);std::string name;
 while(std::getline(in,name,',')){
  if(name.empty()||name.size()>15||name.find_first_not_of("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-")!=std::string::npos||name=="."||name==".."||names->size()>=32||std::find(names->begin(),names->end(),name)!=names->end())return false;
  names->push_back(name);
 }
 return text.back()!=',';
}
void CollectFirmware(ClientConfig* config,const std::string& root){
 auto boardModel=[&]{for(const auto&path:{"/tmp/sysinfo/model","/sys/firmware/devicetree/base/model","/proc/device-tree/model"}){std::ifstream f((root+path).c_str(),std::ios::binary);char buf[130]={};f.read(buf,129);auto n=f.gcount();if(n<=0||n>128)continue;std::string model=Trim(std::string(buf,static_cast<std::size_t>(n)));while(!model.empty()&&model.back()=='\0')model.pop_back();JsonObject check;std::string error;if(!model.empty()&&model.find('\0')==std::string::npos&&ParseJsonObject("{\"model\":"+EscapeJsonString(model)+"}",&check,&error)){config->properties["model"]=model;config->builtin_errors.erase("model");return;}}};
 ExecTask task;task.type="router_config";task.config["backend"]="nvram";task.config["operation"]="get";task.config["key"]="softver";task.timeout=5;
 const auto result=ExecuteExec(task,NULL);const auto value=Trim(result.stdout_text);std::string reason;
 JsonObject check;std::string error;
 if(result.status=="timeout")reason="timeout";
 else if(result.status!="success")reason="command_failed";
 else if(value.empty())reason="empty";
 else if(result.truncated||value.size()>128||value.find('\0')!=std::string::npos||!ParseJsonObject("{\"v\":"+EscapeJsonString(value)+"}",&check,&error))reason="invalid_output";
 if(!reason.empty()){config->builtin_errors["model"]=reason;config->builtin_errors["firmware"]=reason;boardModel();return;}
 config->properties["firmware"]=value;const auto v=value.find('v');const auto model=v==std::string::npos?"":Trim(value.substr(0,v));
 if(model.empty()){config->builtin_errors["model"]="model_prefix_missing";boardModel();}else config->properties["model"]=model;
}
bool ParseCollectionTemplate(const std::string& json,CollectionTemplate* value,std::string* error){
 *error="invalid collection template";JsonObject root,props;
 if(json.size()>64*1024||!ParseJsonObject(json,&root,error))return false;
 CollectionTemplate parsed;parsed.raw_json=json;
 if(!Text(root,"template_id",128,&parsed.id)||!Text(root,"name",128,&parsed.name)||!Number(root,"version",1,UINT64_MAX,&parsed.version))return false;
 if(root.find("properties")==root.end()||!ParseJsonObject(root["properties"].raw_value,&props,error)||props.size()>38)return false;
 if(root.count("monitoring")){JsonObject m;if(!ParseJsonObject(root["monitoring"].raw_value,&m,error))return false;const char*groups[]={"cpu","memory","disk","network","egress"};const unsigned defaults[]={5,5,60,5,600};for(unsigned n=0;n<5;++n){std::string key=std::string(groups[n])+"_seconds";std::uint64_t seconds=defaults[n];if(m.count(key)&&!Number(m,key,0,86400,&seconds))return false;parsed.monitoring[groups[n]]=static_cast<unsigned>(seconds);}
 if(m.count("network_interfaces")){if(m["network_interfaces"].type!=JsonType::kString||!ParseNetworkInterfaces(m["network_interfaces"].string_value,&parsed.network_interfaces))return false;parsed.has_network_interfaces=true;}}
 if(root.count("switch_probe")){if(!ValidateSwitchProbe(root["switch_probe"].raw_value))return false;parsed.switch_json=root["switch_probe"].raw_value;}
 unsigned custom=0;
 for(JsonObject::const_iterator i=props.begin();i!=props.end();++i){
  JsonObject p;CollectionProperty property;std::uint64_t seconds;
  if(!Key(i->first)||!ParseJsonObject(i->second.raw_value,&p,error)||!Text(p,"name",128,&property.name)||!Number(p,"timeout_seconds",1,30,&seconds))return false;
  if(p.count("interval_seconds")){std::uint64_t interval;if(!Number(p,"interval_seconds",0,86400,&interval))return false;property.interval=static_cast<unsigned>(interval);}
  std::string source="command";
  if(p.count("source")&&!Text(p,"source",16,&source))return false;
  if(source=="command") {
   if(p.count("key")||!Text(p,"command",4096,&property.command)||Trim(property.command).empty())return false;
  } else {
   std::string key;
   if(p.count("command")||!Text(p,"key",256,&key))return false;
   property.config["backend"]=source;property.config["operation"]="get";property.config["key"]=key;
   std::vector<std::string> args;
   if(!RouterConfigArguments(property.config,&args,error))return false;
  }
  if(!StandardLimit(i->first))++custom;
  property.timeout=static_cast<unsigned>(seconds);parsed.properties[i->first]=property;
 }
 if(custom>32)return false;
 *value=parsed;error->clear();return true;
}
}
