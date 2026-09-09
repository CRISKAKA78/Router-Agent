#include "rmp/switch_probe.h"
#include "rmp/port_counters.h"
#include "rmp/json.h"
#include "rmp/task.h"
#include <fstream>
#include <sstream>
#include <dirent.h>
#include <unistd.h>
#include <cstdlib>
#include <set>
namespace rmp {
namespace {
std::string Read(const std::string&p){std::ifstream f(p.c_str());std::string s;std::getline(f,s);return s;}
bool Name(const std::string&s){return !s.empty()&&s.size()<=32&&s.find_first_not_of("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_.-")==std::string::npos&&s!="."&&s!="..";}
std::string Hex(const std::string&s){static const char*h="0123456789abcdef";std::string out;for(unsigned char c:s){out+=h[c>>4];out+=h[c&15];}return out;}
struct Port {std::string id,chip,number,system,uplink,label,state,admin,speed,duplex,role;};
void Emit(Metrics&out,const Port&p){
 const std::pair<const char*,std::string> fields[]={{"state",p.state},{"admin",p.admin},{"system",p.system},{"uplink",p.uplink},{"label",p.label},{"chip",p.chip},{"port",p.number},{"speed",p.speed},{"duplex",p.duplex},{"role",p.role}};
 for(const auto&f:fields){Metric m;m.name=f.first;m.entity=p.id;m.unit="text";m.value=f.second;m.status=m.value.empty()?"unknown":"ok";if(m.value.empty())m.reason="not_reported";out["switch_"+p.id+"_"+f.first]=m;}
}
std::string Run(const std::string&command,const std::atomic<bool>*stop,bool*ok){ExecTask t;t.command=command;t.timeout=5;auto r=ExecuteExec(t,stop);*ok=r.status=="success"&&!r.truncated&&r.stdout_text.size()<=32768;return *ok?r.stdout_text:"";}
std::vector<JsonObject> Objects(const std::string&text){
 std::vector<JsonObject> out;bool quoted=false,escape=false;int depth=0;std::size_t start=0;
 for(std::size_t i=0;i<text.size();++i){char c=text[i];if(quoted){if(escape)escape=false;else if(c=='\\')escape=true;else if(c=='"')quoted=false;continue;}if(c=='"'){quoted=true;continue;}if(c=='{'){if(depth++==0)start=i;}else if(c=='}'&&--depth==0){JsonObject p;std::string error;if(!ParseJsonObject(text.substr(start,i-start+1),&p,&error)||out.size()>=64)return {};out.push_back(p);}}
 return out;
}
std::string Str(JsonObject&o,const std::string&k){return o[k].type==JsonType::kString?o[k].string_value:"";}
}
Metrics ParseSwitchRows(const std::string&text){
 Metrics out;std::istringstream lines(text);std::string line;unsigned count=0;
 while(std::getline(lines,line)){
  if(line.empty())continue;
  std::vector<std::string>v;std::istringstream row(line);std::string s;while(std::getline(row,s,'\t'))v.push_back(s);
  if(v.size()!=9||++count>64||v[0].empty()||v[0].size()>32||v[0].find_first_not_of("abcdefghijklmnopqrstuvwxyz0123456789_")!=std::string::npos||out.count("switch_"+v[0]+"_state"))return Metrics();
  for(const auto&f:v)if(f.size()>128)return Metrics();
  Port p;p.id=v[0];p.chip=v[1]=="-"?"":v[1];p.number=v[2]=="-"?"":v[2];p.system=v[3]=="-"?"":v[3];p.uplink=v[4]=="-"?"":v[4];p.state=v[5]=="up"||v[5]=="down"?v[5]:"";p.admin=v[6]=="up"||v[6]=="down"?v[6]:"";p.speed=v[7]=="-"?"":v[7];p.duplex=v[8]=="full"||v[8]=="half"?v[8]:"";p.label=p.id;Emit(out,p);
 }
 return out;
}
bool ValidateSwitchProbe(const std::string&json){
 if(!ValidatePortCounters(json))return false;
 JsonObject c;std::string error;if(!ParseJsonObject(json,&c,&error))return false;
 auto backend=Str(c,"backend");if(backend!="auto"&&backend!="dsa"&&backend!="swconfig"&&backend!="command")return false;
 auto command=Str(c,"command");if(backend=="command"?(command.empty()||command.size()>4096||command.find('\0')!=std::string::npos):!command.empty())return false;
 if(!c.count("ports"))return true;
 const auto&array=c["ports"].raw_value;
 if(array.empty()||array.front()!='['||array.back()!=']')return false;
 // ParseJsonObject validated JSON syntax. Outside an object only array punctuation may occur.
 int depth=0;bool quoted=false,escape=false;unsigned count=0;
 for(std::size_t pos=0;pos<array.size();++pos){char ch=array[pos];if(depth==0&&((ch=='['&&pos!=0)||(ch==']'&&pos+1!=array.size())))return false;if(quoted){if(escape)escape=false;else if(ch=='\\')escape=true;else if(ch=='"')quoted=false;continue;}if(ch=='"'){if(depth==0)return false;quoted=true;continue;}if(ch=='{'){if(depth++==0)++count;}else if(ch=='}')--depth;else if(depth==0&&ch!='['&&ch!=']'&&ch!=','&&ch!=' '&&ch!='\t'&&ch!='\n'&&ch!='\r')return false;}
 auto ports=Objects(array);if(ports.size()!=count||ports.size()>64)return false;std::set<std::string> ids,sources;
 for(auto p:ports){auto id=Str(p,"id"),chip=Str(p,"switch_id"),system=Str(p,"system_name"),uplink=Str(p,"uplink"),label=Str(p,"display_name"),role=Str(p,"role");
  if(id.empty()||id.size()>32||id[0]<'a'||id[0]>'z'||id.find_first_not_of("abcdefghijklmnopqrstuvwxyz0123456789_")!=std::string::npos||!ids.insert(id).second||(p.count("port")&&(p["port"].type!=JsonType::kUnsignedInteger||p["port"].unsigned_value>255))||(!chip.empty()&&!p.count("port"))||chip.size()>64||system.size()>15||uplink.size()>15||label.size()>128||(!role.empty()&&role!="external"&&role!="cpu"))return false;
  auto source=chip.empty()?"system:"+system:chip+":"+std::to_string(p["port"].unsigned_value);if(source!="system:"&&!sources.insert(source).second)return false;
 }
 return true;
}
Metrics CollectSwitchPorts(const std::string&json,const std::atomic<bool>*stop,const std::string&root){
 Metrics out;JsonObject cfg;std::string error;if(!ParseJsonObject(json,&cfg,&error))return out;auto backend=Str(cfg,"backend");
 if(backend=="command"){bool ok=false;out=ParseSwitchRows(Run(Str(cfg,"command"),stop,&ok));if(!ok||out.empty()){Metric m;m.name="交换机采集";m.unit="text";m.status="error";m.reason="switch_command_failed";out["switch_collection_status"]=m;}}
 if(backend=="swconfig"||backend=="auto"){
  bool ok=false;auto listing=Run("swconfig list",stop,&ok);std::istringstream lines(listing);std::string line;unsigned chips=0,total=0;
  while(ok&&std::getline(lines,line)&&chips<8){std::istringstream words(line);std::string found,chip;words>>found>>chip;if(found!="Found:"||!Name(chip)||chip.size()>12)continue;++chips;bool valid=false;auto text=Run("swconfig dev "+chip+" show",stop,&valid);if(!valid)continue;
   std::istringstream rows(text);Port p;bool active=false;unsigned count=0;
   while(std::getline(rows,line)){
    if(line.compare(0,5,"Port ")==0){if(active)Emit(out,p);p=Port();auto end=line.find(':',5);auto n=line.substr(5,end-5);active=end!=std::string::npos&&!n.empty()&&n.find_first_not_of("0123456789")==std::string::npos&&n.size()<=3&&std::strtoul(n.c_str(),NULL,10)<=255&&++count<=64&&++total<=64;if(!active)continue;p.id="s"+Hex(chip)+"p"+n;p.chip=chip;p.number=n;p.label="端口 "+n;}
    if(active&&line.find("link:")!=std::string::npos){p.state=line.find("link:up")!=std::string::npos?"up":line.find("link:down")!=std::string::npos?"down":"";auto speed=line.find("speed:");if(speed!=std::string::npos){auto end=line.find_first_not_of("0123456789",speed+6);p.speed=line.substr(speed+6,end-(speed+6));}p.duplex=line.find("full-duplex")!=std::string::npos?"full":line.find("half-duplex")!=std::string::npos?"half":"";}
   }if(active)Emit(out,p);
  }
 }
 if(backend=="dsa"||(backend=="auto"&&out.empty())){
  auto dir=opendir((root+"/sys/class/net").c_str());if(dir){struct dirent*entry;unsigned count=0;while((entry=readdir(dir))&&count<64){std::string n=entry->d_name;if(!Name(n)||n.size()>15)continue;auto base=root+"/sys/class/net/"+n;auto chip=Read(base+"/phys_switch_id");auto port=Read(base+"/phys_port_name");if(port.size()>1&&port[0]=='p'&&port.find_first_not_of("0123456789",1)==std::string::npos)port.erase(0,1);if(chip.empty()&&access((base+"/device").c_str(),F_OK)!=0)continue;++count;Port p;p.id="n"+Hex(n);p.chip=chip;p.number=port;p.system=n;p.label=n;auto carrier=Read(base+"/carrier");p.state=carrier=="1"?"up":carrier=="0"?"down":"";auto flags=Read(base+"/flags");if(!flags.empty())p.admin=(std::strtoul(flags.c_str(),NULL,0)&1)?"up":"down";p.speed=Read(base+"/speed");if(p.speed=="-1")p.speed.clear();p.duplex=Read(base+"/duplex");auto link=Read(base+"/iflink");if(!link.empty()&&link!=Read(base+"/ifindex")){auto peers=opendir((root+"/sys/class/net").c_str());if(peers){struct dirent*peer;while((peer=readdir(peers))){std::string pn=peer->d_name;if(Name(pn)&&Read(root+"/sys/class/net/"+pn+"/ifindex")==link){p.uplink=pn;break;}}closedir(peers);}}Emit(out,p);}closedir(dir);}
 }
 // Board mappings provide physical labels and associations; they never invent link state.
 for(auto p:Objects(cfg["ports"].raw_value)){auto id=Str(p,"id");if(id.empty())continue;
  auto chip=Str(p,"switch_id"),system=Str(p,"system_name");auto number=p.count("port")?std::to_string(p["port"].unsigned_value):std::string();std::string source="switch_"+id;
  for(const auto&v:out){if(chip.empty()||v.second.name!="chip"||v.second.value!=chip)continue;auto candidate=v.first.substr(0,v.first.size()-5);auto n=out.find(candidate+"_port");if(n!=out.end()&&n->second.value==number){source=candidate;break;}}
  if(backend!="command"&&chip.empty()&&!system.empty())source="switch_n"+Hex(system);
  Port mapped;mapped.id=id;mapped.chip=chip;mapped.number=number;mapped.system=system;mapped.uplink=Str(p,"uplink");mapped.label=Str(p,"display_name");mapped.role=Str(p,"role");
  auto detected=[&](const std::string&key){auto i=out.find(source+"_"+key);return i==out.end()?std::string():i->second.value;};
  if(mapped.chip.empty())mapped.chip=detected("chip");
  if(mapped.number.empty())mapped.number=detected("port");
  if(mapped.system.empty())mapped.system=detected("system");
  if(mapped.uplink.empty())mapped.uplink=detected("uplink");
  if(mapped.label.empty())mapped.label=detected("label");
  if(mapped.label.empty())mapped.label=id;
  for(auto field:{"state","admin","speed","duplex"}){auto i=out.find(source+"_"+field);std::string val=i==out.end()?"":i->second.value;if(std::string(field)=="state")mapped.state=val;else if(std::string(field)=="admin")mapped.admin=val;else if(std::string(field)=="speed")mapped.speed=val;else mapped.duplex=val;}
  if(source!="switch_"+id){for(auto i=out.begin();i!=out.end();){if(i->first.compare(0,source.size()+1,source+"_")==0)i=out.erase(i);else ++i;}}
  Emit(out,mapped);
 }
 if(out.empty()){Metric m;m.name="交换机采集";m.unit="text";m.status="unknown";m.reason="switch_not_supported";out["switch_collection_status"]=m;}
 return out;
}
}
