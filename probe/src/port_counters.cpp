#include "rmp/port_counters.h"
#include "rmp/switch_probe.h"
#include "rmp/json.h"
#include "rmp/task.h"
#include <sstream>
#include <set>
#include <limits>
#include <iomanip>
#include <cstdlib>

namespace rmp {
namespace {
typedef std::chrono::steady_clock Clock;
std::string Str(const JsonObject&o,const std::string&k){auto i=o.find(k);return i!=o.end()&&i->second.type==JsonType::kString?i->second.string_value:"";}
bool UInt(const std::string&s,std::uint64_t*out){
 if(s.empty())return false;
 std::uint64_t n=0;
 for(char c:s){if(c<'0'||c>'9'||n>(UINT64_MAX-(c-'0'))/10)return false;n=n*10+(c-'0');}*out=n;return true;
}
std::string Trim(const std::string&s){auto a=s.find_first_not_of(" \t\r\n"),b=s.find_last_not_of(" \t\r\n");return a==std::string::npos?"":s.substr(a,b-a+1);}
bool Text(const std::string&s,std::size_t n){return !s.empty()&&s.size()<=n&&s.find('\0')==std::string::npos;}
bool Name(const std::string&s){return Text(s,32)&&s!="."&&s!=".."&&s.find_first_not_of("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-")==std::string::npos;}
std::vector<JsonObject> Ports(const JsonObject&o){
 auto i=o.find("ports");std::vector<JsonObject> out;if(i==o.end())return out;
 const auto&s=i->second.raw_value;bool quote=false,escape=false;int depth=0;std::size_t start=0;
 for(std::size_t n=0;n<s.size();++n){char c=s[n];if(quote){if(escape)escape=false;else if(c=='\\')escape=true;else if(c=='"')quote=false;continue;}if(c=='"'){quote=true;continue;}if(c=='{'){if(depth++==0)start=n;}else if(c=='}'&&--depth==0){JsonObject p;std::string error;if(!ParseJsonObject(s.substr(start,n-start+1),&p,&error))return {};out.push_back(p);}}
 return out;
}
void Put(Metrics&m,const std::string&id,const std::string&field,const std::string&value,const std::string&status="ok",const std::string&reason="",Clock::time_point at=Clock::now()){
 auto key="switch_"+id+"_"+field;Metric v;v.name=field;v.entity=id;v.value=value;v.unit=MetricUnit(key);v.status=status;v.reason=reason;v.sampled=at;m[key]=v;
}
void Missing(Metrics&m,const std::string&id,const std::string&reason){for(auto f:{"rx_raw_bytes","tx_raw_bytes","rx_bytes","tx_bytes","rx_bytes_per_sec","tx_bytes_per_sec","elapsed_seconds"})Put(m,id,f,"","error",reason);}
bool Run(const std::string&cmd,const std::atomic<bool>*stop,std::string*out){ExecTask t;t.command=cmd;t.timeout=5;auto r=ExecuteExec(t,stop);if(r.status!="success"||r.truncated||r.stdout_text.size()>32768)return false;*out=r.stdout_text;return true;}
}
bool ValidatePortCounters(const std::string&json){
 JsonObject root,c;std::string error;if(!ParseJsonObject(json,&root,&error))return false;
 if(!root.count("counters"))return true;
 if(!ParseJsonObject(root["counters"].raw_value,&c,&error))return false;
 auto bits=c.find("bits");if(bits==c.end()||bits->second.type!=JsonType::kUnsignedInteger||(bits->second.unsigned_value!=32&&bits->second.unsigned_value!=64)||!Text(Str(c,"basis"),128))return false;
 auto ports=Ports(root);if(ports.empty()||ports.size()>16)return false;
 if(Str(c,"backend")=="swconfig_mib"){
  if(!Str(c,"command").empty()||!Text(Str(c,"rx_field"),64)||!Text(Str(c,"tx_field"),64)||Str(c,"rx_field")==Str(c,"tx_field"))return false;
  for(auto p:ports)if(!Name(Str(p,"switch_id"))||!p.count("port")||p["port"].type!=JsonType::kUnsignedInteger||p["port"].unsigned_value>255)return false;
 }else if(Str(c,"backend")=="command"){
  if(!Text(Str(c,"command"),4096)||!Str(c,"rx_field").empty()||!Str(c,"tx_field").empty())return false;
 }else return false;
 return true;
}
bool ParseMibBytes(const std::string&text,const std::string&rxField,const std::string&txField,std::uint64_t*rx,std::uint64_t*tx){
 bool gotRX=false,gotTX=false;std::istringstream in(text);std::string line;
 while(std::getline(in,line)){auto pos=line.find(':');if(pos==std::string::npos)continue;auto k=Trim(line.substr(0,pos));
  if(k==rxField){if(gotRX||!UInt(Trim(line.substr(pos+1)),rx))return false;gotRX=true;}
  if(k==txField){if(gotTX||!UInt(Trim(line.substr(pos+1)),tx))return false;gotTX=true;}
 }return gotRX&&gotTX;
}
void PortCounterSampler::Observe(Metrics&out,const std::string&id,const std::string&source,std::uint64_t rx,std::uint64_t tx,unsigned bits,double speed,Clock::time_point at){
 if(bits==32&&(rx>UINT32_MAX||tx>UINT32_MAX)){Missing(out,id,"counter_out_of_range");previous_.erase(id);return;}
 auto old=previous_.find(id);Sample n;n.source=source;n.rx=rx;n.tx=tx;n.at=at;n.start=at;
 double dt=old==previous_.end()?0:std::chrono::duration<double>(at-old->second.at).count();
 bool valid=old!=previous_.end()&&old->second.source==source&&dt>0&&rx>=old->second.rx&&tx>=old->second.tx;
 // With 32-bit counters even increasing values can hide a whole wrap. Never guess a reset vs wrap.
 if(bits==32&&(speed<=0||dt*speed*1000000.0/8>=4294967296.0))valid=false;
 std::uint64_t dr=0,dtBytes=0;
 if(valid){dr=rx-old->second.rx;dtBytes=tx-old->second.tx;
  if((speed>0&&(dr/dt>speed*1000000.0/8*1.05||dtBytes/dt>speed*1000000.0/8*1.05))||dr>UINT64_MAX-old->second.total_rx||dtBytes>UINT64_MAX-old->second.total_tx)valid=false;
 }
 if(valid){n.total_rx=old->second.total_rx+dr;n.total_tx=old->second.total_tx+dtBytes;n.start=old->second.start;}
 Put(out,id,"rx_raw_bytes",std::to_string(rx),"ok","",at);Put(out,id,"tx_raw_bytes",std::to_string(tx),"ok","",at);
 Put(out,id,"rx_bytes",std::to_string(n.total_rx),"ok","",at);Put(out,id,"tx_bytes",std::to_string(n.total_tx),"ok","",at);
 Put(out,id,"elapsed_seconds",std::to_string(std::chrono::duration_cast<std::chrono::seconds>(at-n.start).count()),"ok","",at);
 for(int direction=0;direction<2;++direction){std::ostringstream value;if(valid)value<<std::setprecision(12)<<(direction?dtBytes:dr)/dt;
  Put(out,id,direction?"tx_bytes_per_sec":"rx_bytes_per_sec",value.str(),valid?"ok":"waiting",valid?"":"counter_baseline",at);}
 previous_[id]=n;
}
Metrics PortCounterSampler::Collect(const std::string&json,const std::atomic<bool>*stop){
 auto out=CollectSwitchPorts(json,stop);JsonObject root,c;std::string error;ParseJsonObject(json,&root,&error);
 if(stop&&*stop)return out;
 if(!root.count("counters")){Clear();return out;}
 if(!ValidatePortCounters(json)||!ParseJsonObject(root["counters"].raw_value,&c,&error))return out;
 const auto backend=Str(c,"backend"),rxField=Str(c,"rx_field"),txField=Str(c,"tx_field");unsigned bits=static_cast<unsigned>(c["bits"].unsigned_value);
 auto ports=Ports(root);std::set<std::string> ids;for(auto p:ports)ids.insert(Str(p,"id"));
 // An explicit board profile is the inventory; extra driver enumeration slots are not RJ45s.
 for(auto i=out.begin();i!=out.end();)if(i->first!="switch_collection_status"&&!ids.count(i->second.entity))i=out.erase(i);else ++i;
 for(auto i=previous_.begin();i!=previous_.end();)if(!ids.count(i->first))i=previous_.erase(i);else ++i;
 struct Raw {std::uint64_t rx=0,tx=0;};std::map<std::string,Raw> rows;bool commandOK=true;auto commandAt=Clock::now();
 if(backend=="command"){
  std::string text;auto before=Clock::now();commandOK=Run(Str(c,"command"),stop,&text);auto after=Clock::now();commandAt=before+(after-before)/2;
  std::istringstream in(text);std::string line;unsigned count=0;
  while(commandOK&&std::getline(in,line)){if(line.empty())continue;auto a=line.find('\t'),b=a==std::string::npos?a:line.find('\t',a+1);Raw v;auto id=line.substr(0,a);
   if(++count>64||a==std::string::npos||b==std::string::npos||line.find('\t',b+1)!=std::string::npos||rows.count(id)||!ids.count(id)||!UInt(line.substr(a+1,b-a-1),&v.rx)||!UInt(Trim(line.substr(b+1)),&v.tx))commandOK=false;else rows[id]=v;
  }
 }
 auto deadline=Clock::now()+std::chrono::seconds(10);unsigned order=0;
 for(auto p:ports){const auto id=Str(p,"id"),chip=Str(p,"switch_id");auto number=p.count("port")?std::to_string(p["port"].unsigned_value):"";
  std::string source=backend=="swconfig_mib"?backend+" "+chip+":"+number+" "+rxField+"/"+txField:backend+" "+id;
  if(stop&&*stop)return out;
  std::string identity=source+"\n"+chip+":"+number+"\n"+Str(p,"system_name")+"\n"+Str(c,"command")+"\n"+std::to_string(bits)+"\n"+Str(c,"basis");
  Put(out,id,"counter_source",source);Put(out,id,"counter_basis",Str(c,"basis"));Put(out,id,"order",std::to_string(order++));
  std::uint64_t rx=0,tx=0;bool ok=false;auto at=commandAt;
  if(!(stop&&*stop)&&Clock::now()<deadline){
   if(backend=="command"){auto r=rows.find(id);ok=commandOK&&r!=rows.end();if(ok){rx=r->second.rx;tx=r->second.tx;}}
   else {std::string text;auto before=Clock::now();ok=Run("swconfig dev "+chip+" port "+number+" get mib",stop,&text)&&ParseMibBytes(text,rxField,txField,&rx,&tx);auto after=Clock::now();at=before+(after-before)/2;}
  }
  if(stop&&*stop)return out;
  if(!ok){Missing(out,id,"port_counter_failed");previous_.erase(id);continue;}
  auto s=out.find("switch_"+id+"_speed");double speed=s==out.end()?0:std::strtod(s->second.value.c_str(),NULL);
  Observe(out,id,identity,rx,tx,bits,speed,at);
 }
 return out;
}
}
