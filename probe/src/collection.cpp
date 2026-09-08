#include "rmp/collection.h"
#include "rmp/json.h"
#include "rmp/task.h"
#include <chrono>
#include <iostream>
#include <sstream>
#include <algorithm>

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
void Entry(std::ostringstream& out,bool* first,const std::string& key,const std::string& name,const std::string& field,const std::string& value){if(!*first)out<<',';*first=false;out<<EscapeJsonString(key)<<":{\"name\":"<<EscapeJsonString(name)<<','<<EscapeJsonString(field)<<':'<<EscapeJsonString(value)<<'}';}
}
bool ParseCollectionTemplate(const std::string& json,CollectionTemplate* value,std::string* error){
 *error="invalid collection template";JsonObject root,props;
 if(json.size()>64*1024||!ParseJsonObject(json,&root,error))return false;
 CollectionTemplate parsed;
 if(!Text(root,"template_id",128,&parsed.id)||!Text(root,"name",128,&parsed.name)||!Number(root,"version",1,UINT64_MAX,&parsed.version))return false;
 if(root.find("properties")==root.end()||!ParseJsonObject(root["properties"].raw_value,&props,error)||props.empty()||props.size()>38)return false;
 unsigned custom=0;
 for(JsonObject::const_iterator i=props.begin();i!=props.end();++i){
  JsonObject p;CollectionProperty property;std::uint64_t seconds;
  if(!Key(i->first)||!ParseJsonObject(i->second.raw_value,&p,error)||!Text(p,"name",128,&property.name)||!Number(p,"timeout_seconds",1,30,&seconds))return false;
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
void CollectProperties(const CollectionTemplate& value,ClientConfig* config){
 typedef std::chrono::steady_clock Clock;
 const Clock::time_point deadline=Clock::now()+std::chrono::seconds(60);
 std::ostringstream attributes,errors,reference;attributes<<'{';errors<<'{';bool first_attr=true,first_error=true;
 config->properties.clear();
 if(!config->explicit_hostname)config->hostname.clear();
 for(std::map<std::string,CollectionProperty>::const_iterator i=value.properties.begin();i!=value.properties.end();++i){
  if(i->first=="hostname"&&config->explicit_hostname)continue;
  const CollectionProperty&p=i->second;std::string reason,output;
  const long remaining=static_cast<long>(std::chrono::duration_cast<std::chrono::seconds>(deadline-Clock::now()).count());
  if(remaining<1)reason="budget_exhausted";
  else{
   ExecTask task;task.command=p.command;task.timeout=std::min(p.timeout,static_cast<unsigned>(remaining));
   if(!p.config.empty()){task.type="router_config";task.config=p.config;}
   const ExecResult result=ExecuteExec(task,NULL);
   output=Trim(result.stdout_text);
   if(result.status=="timeout")reason="timeout";
   else if(result.status!="success")reason="command_failed";
   else if(output.empty())reason="empty";
   else{
    const std::size_t limit=StandardLimit(i->first)?StandardLimit(i->first):4096;
    JsonObject test;std::string e;
    if(result.truncated||output.size()>limit||output.find('\0')!=std::string::npos||!ParseJsonObject("{\"v\":"+EscapeJsonString(output)+"}",&test,&e))reason="invalid_output";
    if(i->first=="libc")for(std::size_t n=0;n<output.size();++n)if(static_cast<unsigned char>(output[n])>127)reason="invalid_output";
   }
  }
  if(!reason.empty()){Entry(errors,&first_error,i->first,p.name,"reason",reason);std::cerr<<"collection_property="<<i->first<<" reason="<<reason<<std::endl;}
  else if(i->first=="hostname")config->hostname=output;
  else if(StandardLimit(i->first))config->properties[i->first]=output;
  else Entry(attributes,&first_attr,i->first,p.name,"value",output);
 }
 attributes<<'}';errors<<'}';
 reference<<"{\"template_id\":"<<EscapeJsonString(value.id)<<",\"name\":"<<EscapeJsonString(value.name)<<",\"version\":"<<value.version<<'}';
 config->template_reference=reference.str();config->attributes=attributes.str();config->collection_errors=errors.str();
}
}
