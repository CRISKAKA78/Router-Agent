#include "rmp/egress.h"
#include "rmp/telemetry.h"
#include "rmp/switch_probe.h"
#include "rmp/port_counters.h"
#include "rmp/json.h"
#include "rmp/task.h"
#include <algorithm>
#include <arpa/inet.h>
#include <cmath>
#include <cstdlib>
#include <dirent.h>
#include <fstream>
#include <iomanip>
#include <limits>
#include <set>
#include <sstream>
#include <sys/statvfs.h>
#include <sys/utsname.h>
#include <unistd.h>
namespace rmp {
namespace {
typedef std::chrono::steady_clock Clock;
std::string Trim(const std::string&s){const auto a=s.find_first_not_of(" \t\r\n\0",0,5),b=s.find_last_not_of(" \t\r\n\0",std::string::npos,5);return a==std::string::npos?"":s.substr(a,b-a+1);}
std::string Read(const std::string&p,std::size_t limit=262144){std::ifstream f(p.c_str(),std::ios::binary);if(!f)return "";std::string s;char buf[4096];while(f&&s.size()<=limit){f.read(buf,sizeof(buf));s.append(buf,static_cast<std::size_t>(f.gcount()));}return s.size()>limit?"":Trim(s);}
std::string Num(double n){std::ostringstream o;o.imbue(std::locale::classic());o<<std::fixed<<std::setprecision(2)<<n;return o.str();}
std::string Integer(std::uint64_t n){std::ostringstream o;o<<n;return o.str();}
bool UInt(const std::string&s,std::uint64_t*out){if(s.empty())return false;std::uint64_t n=0;for(char c:s){if(c<'0'||c>'9'||n>(static_cast<std::uint64_t>(INT64_MAX)-(c-'0'))/10)return false;n=n*10+c-'0';}*out=n;return true;}
bool Number(const std::string&s,double*out){std::istringstream i(s);i.imbue(std::locale::classic());double n;char tail;if(!(i>>n)||(i>>tail)||(n!=n||n==std::numeric_limits<double>::infinity())||n<0)return false;*out=n;return true;}
void Add(Metrics&m,const std::string&k,const std::string&name,const std::string&v,const std::string&entity="",const std::string&status="") {Metric x;x.name=name;x.value=v;x.entity=entity;x.unit=MetricUnit(k);x.status=status.empty()?(v.empty()?"unknown":"ok"):status;m[k]=x;}
std::vector<std::string> Dirs(const std::string&p){std::vector<std::string> out;DIR*d=opendir(p.c_str());if(!d)return out;while(dirent*e=readdir(d)){if(e->d_name[0]!='.'&&out.size()<256)out.push_back(e->d_name);}closedir(d);std::sort(out.begin(),out.end());return out;}
std::map<std::string,std::string> Fields(const std::string&s){std::map<std::string,std::string> m;std::istringstream in(s);std::string line;while(std::getline(in,line)){auto p=line.find(':');if(p!=std::string::npos&&!m.count(Trim(line.substr(0,p))))m[Trim(line.substr(0,p))]=Trim(line.substr(p+1));}return m;}
std::string Hex(const std::string&s){std::ostringstream o;o<<std::hex<<std::setfill('0');for(unsigned char c:s)o<<std::setw(2)<<static_cast<unsigned>(c);return o.str();}
std::string MountKey(const std::string&s){std::uint64_t n=14695981039346656037ULL;for(unsigned char c:s){n^=c;n*=1099511628211ULL;}std::ostringstream o;o<<std::hex<<std::setw(16)<<std::setfill('0')<<n;return o.str();}
std::string UnescapeMount(const std::string&s){std::string o;for(std::size_t i=0;i<s.size();++i){if(s[i]=='\\'&&i+3<s.size()&&s[i+1]>='0'&&s[i+1]<='7'&&s[i+2]>='0'&&s[i+2]<='7'&&s[i+3]>='0'&&s[i+3]<='7'){o+=static_cast<char>((s[i+1]-'0')*64+(s[i+2]-'0')*8+s[i+3]-'0');i+=3;}else o+=s[i];}return o;}
double Frequency(const std::string&root,const char* field){double max=0;for(const auto&n:Dirs(root+"/sys/devices/system/cpu")){if(n.size()<4||n.substr(0,3)!="cpu"||n[3]<'0'||n[3]>'9')continue;double f;if(Number(Read(root+"/sys/devices/system/cpu/"+n+"/cpufreq/"+field,128),&f))max=std::max(max,f/1000);}return max;}
}
std::string MetricUnit(const std::string&k){if(k!="probe_bits"&&k!="kernel_bits"&&k.find("cpu_")!=0&&k.find("memory_")!=0&&k.find("disk_")!=0&&k.find("net_")!=0&&k.find("switch_")!=0)return "text";if(k.size()>=8&&k.substr(k.size()-8)=="_seconds")return "seconds";if(k.size()>=6&&k.substr(k.size()-6)=="_usage")return "percent";if(k.size()>=14&&k.substr(k.size()-14)=="_bytes_per_sec")return "bytes_per_sec";if(k.size()>=6&&k.substr(k.size()-6)=="_bytes")return "bytes";if(k.size()>=4&&k.substr(k.size()-4)=="_mhz")return "mhz";if(k=="probe_bits"||k=="kernel_bits"||k=="cpu_hardware_bits")return "bits";return "text";}
Metrics HardwareMetrics(const std::string&root){
 Metrics out;auto f=Fields(Read(root+"/proc/cpuinfo"));struct utsname u;std::string machine;
 if(root.empty()&&uname(&u)==0)machine=u.machine;else machine=Read(root+"/uname_machine",128);
 std::string model=f["model name"],arch,bits,soc;
 const auto implementer=f["CPU implementer"],part=f["CPU part"];
 if(implementer=="0x41"){
  const std::map<std::string,std::string> arm={{"0xc05","Cortex-A5"},{"0xc07","Cortex-A7"},{"0xc08","Cortex-A8"},{"0xc09","Cortex-A9"},{"0xc0d","Cortex-A12"},{"0xc0e","Cortex-A17"},{"0xc0f","Cortex-A15"},{"0xd01","Cortex-A32"},{"0xd03","Cortex-A53"},{"0xd04","Cortex-A35"},{"0xd05","Cortex-A55"},{"0xd07","Cortex-A57"},{"0xd08","Cortex-A72"},{"0xd09","Cortex-A73"},{"0xd0a","Cortex-A75"},{"0xd0b","Cortex-A76"},{"0xd0c","Neoverse-N1"},{"0xd41","Cortex-A78"}};
  auto a=arm.find(part);if(a!=arm.end()){model="ARM "+a->second;arch=part.substr(0,3)=="0xc"?"ARMv7-A":"ARMv8-A";bits=part.substr(0,3)=="0xc"||part=="0xd01"?"32":"64";}
 }
 if(model.empty())model=f["cpu model"];
 if(model.empty()&&!implementer.empty()&&!part.empty())model="ARM implementer "+implementer+" part "+part;
 if(arch.empty()){
  if(machine=="aarch64"||machine=="arm64"){arch="AArch64 (ARMv8+, generation unknown)";bits="64";}
  else if(f["CPU architecture"]=="8"||machine=="armv8l")arch="ARMv8-A";
  else if(f["CPU architecture"]=="7"||machine=="armv7l")arch="ARMv7-A";
  else if(machine=="armv6l")arch="ARMv6";
  else if(machine=="x86_64"){arch="x86-64";bits="64";}
  else if(machine=="i386"||machine=="i686"){arch="x86";bits=(" "+f["flags"]+" ").find(" lm ")!=std::string::npos?"64":"";}
  else if(machine.find("mips")!=std::string::npos)arch=machine;
 }
 // Device-tree compatible strings identify the SoC, not the board's model string.
 auto dt=Read(root+"/sys/firmware/devicetree/base/compatible",4096);if(dt.empty())dt=Read(root+"/proc/device-tree/compatible",4096);
 std::istringstream compatibles(dt);std::string c;while(std::getline(compatibles,c,'\0'))if(!c.empty())soc=c;
 if(soc.empty())soc=Read(root+"/sys/devices/soc0/soc_id",128);
 Add(out,"cpu_model","CPU 型号",model);Add(out,"cpu_soc","SoC 标识",soc);Add(out,"cpu_arch","CPU 架构",arch);Add(out,"cpu_hardware_bits","CPU 硬件位数",bits);Add(out,"kernel_arch","内核运行架构",machine);Add(out,"probe_bits","Probe 程序位数",Integer(sizeof(void*)*8));
 std::string kernel_bits=(machine=="aarch64"||machine=="arm64"||machine=="x86_64"||machine=="mips64"||machine=="mips64el")?"64":(machine=="armv7l"||machine=="armv8l"||machine=="armv6l"||machine=="i386"||machine=="i486"||machine=="i586"||machine=="i686")?"32":"";Add(out,"kernel_bits","内核运行位数",kernel_bits);
 double freq=Frequency(root,"cpuinfo_max_freq");Add(out,"cpu_max_mhz","CPU 最大频率",freq>0?Num(freq):"");return out;
}
void SystemSampler::StartHardware(unsigned seconds){
 if(hardware_worker_.joinable())return;
 {std::lock_guard<std::mutex> l(hardware_mutex_);hardware_=HardwareMetrics(root_);hardware_attempts_=1;}
 hardware_worker_=std::thread([this,seconds]{auto due=Clock::now()+std::chrono::seconds(seconds?seconds:5);while(!hardware_stop_&&Clock::now()<due)std::this_thread::sleep_for(std::chrono::milliseconds(25));if(hardware_stop_)return;auto m=HardwareMetrics(root_);std::lock_guard<std::mutex>l(hardware_mutex_);hardware_=m;hardware_attempts_=2;});
}
SystemSampler::~SystemSampler(){hardware_stop_=true;if(hardware_worker_.joinable())hardware_worker_.join();}
Metrics SystemSampler::Hardware(){std::lock_guard<std::mutex>l(hardware_mutex_);return hardware_;}
unsigned SystemSampler::HardwareAttempts(){std::lock_guard<std::mutex>l(hardware_mutex_);return hardware_attempts_;}
Metrics SystemSampler::StaticProperties(const std::string&bytes){std::lock_guard<std::mutex>l(template_mutex_);if(static_template_!=bytes){static_template_=bytes;static_properties_.clear();}return static_properties_;}
void SystemSampler::SaveStaticProperty(const std::string&bytes,const std::string&key,const Metric&m){std::lock_guard<std::mutex>l(template_mutex_);if(static_template_==bytes)static_properties_[key]=m;}
Metrics SystemSampler::Sample(const std::string&group){
 Metrics out;
 if(group=="cpu"){
  std::istringstream in(Read(root_+"/proc/stat"));std::string label;std::uint64_t v[10]={};in>>label;bool valid=label=="cpu";for(int n=0;n<4;++n)valid=valid&&static_cast<bool>(in>>v[n]);for(int n=4;n<10;++n)if(!(in>>v[n]))break;
  std::uint64_t total=0;for(int n=0;n<8;++n)total+=v[n];const auto idle=v[3]+v[4];std::string usage,status="unknown";
  if(valid){status="waiting";if(cpu_valid_&&total>cpu_total_&&idle>=cpu_idle_&&idle-cpu_idle_<=total-cpu_total_){usage=Num(100.0*(total-cpu_total_-(idle-cpu_idle_))/(total-cpu_total_));status="ok";}cpu_total_=total;cpu_idle_=idle;}cpu_valid_=valid;
  Add(out,"cpu_usage","CPU 使用率",usage,"",status);double freq=Frequency(root_,"scaling_cur_freq");if(freq==0)freq=Frequency(root_,"cpuinfo_cur_freq");if(freq==0){auto f=Fields(Read(root_+"/proc/cpuinfo"));Number(f["cpu MHz"],&freq);}Add(out,"cpu_current_mhz","CPU 当前频率",freq>0?Num(freq):"");
 }else if(group=="memory"){
  auto fields=Fields(Read(root_+"/proc/meminfo"));std::map<std::string,std::uint64_t> n;for(const auto&p:fields){std::istringstream in(p.second);std::string number,unit;in>>number>>unit;std::uint64_t x;if(unit=="kB"&&UInt(number,&x)&&x<=static_cast<std::uint64_t>(INT64_MAX)/1024)n[p.first]=x*1024;}
  const auto total=n["MemTotal"];bool exact=n.count("MemAvailable")!=0;std::uint64_t available=0;
  if(exact)available=n["MemAvailable"];else {available=n["MemFree"]+n["Buffers"]+n["Cached"]+n["SReclaimable"];available=available>n["Shmem"]?available-n["Shmem"]:0;}available=std::min(total,available);
  Add(out,"memory_usage","内存使用率",total?Num(100.0*(total-available)/total):"");Add(out,"memory_used_bytes","已用内存",total?Integer(total-available):"");Add(out,"memory_total_bytes","总内存",total?Integer(total):"");Add(out,"memory_available_bytes","可用内存",total?Integer(available):"");Add(out,"memory_method","内存统计口径",exact?"MemAvailable":"旧内核缓存回收估算");
 }else if(group=="network"){
  std::istringstream in(Read(root_+"/proc/net/dev"));std::string line;std::map<std::string,Net> next;unsigned count=0;
  while(std::getline(in,line)&&count<48){auto split=line.find(':');if(split==std::string::npos)continue;auto name=Trim(line.substr(0,split));if((name=="lo"&&names_.empty())||name.size()>15||name.find('/')!=std::string::npos)continue;if(!names_.empty()&&std::find(names_.begin(),names_.end(),name)==names_.end())continue;std::istringstream nums(line.substr(split+1));std::uint64_t v[16];bool valid=true;for(int j=0;j<16;++j)if(!(nums>>v[j])){valid=false;break;}if(!valid)continue;++count;
   const auto base=root_+"/sys/class/net/"+name;Net n;n.rx=v[0];n.tx=v[8];n.index=Read(base+"/ifindex",32);n.at=Clock::now();auto old=networks_.find(name);double seconds=old==networks_.end()?0:std::chrono::duration<double>(n.at-old->second.at).count();bool delta=old!=networks_.end()&&n.index==old->second.index&&n.rx>=old->second.rx&&n.tx>=old->second.tx&&seconds>0;
   n.start_rx=delta?old->second.start_rx:n.rx;n.start_tx=delta?old->second.start_tx:n.tx;n.start=delta?old->second.start:n.at;
   auto key="net_"+Hex(name);
   Add(out,key+"_rx_bytes","累计接收",Integer(n.rx-n.start_rx),name);Add(out,key+"_tx_bytes","累计发送",Integer(n.tx-n.start_tx),name);
   Add(out,key+"_elapsed_seconds","统计时长",Integer(std::chrono::duration_cast<std::chrono::seconds>(n.at-n.start).count()),name);
Add(out,key+"_rx_bytes_per_sec","接收速率",delta?Num((n.rx-old->second.rx)/seconds):"",name,delta?"ok":"waiting");Add(out,key+"_tx_bytes_per_sec","发送速率",delta?Num((n.tx-old->second.tx)/seconds):"",name,delta?"ok":"waiting");Add(out,key+"_state","接口状态",Read(base+"/operstate",32),name);
   char link[1024];auto len=readlink(base.c_str(),link,sizeof(link)-1);std::string target=len>0?std::string(link,len):"";std::string kind=access((base+"/bridge").c_str(),F_OK)==0?"网桥":target.find("/virtual/")!=std::string::npos?"虚拟接口":access((base+"/device").c_str(),F_OK)==0?"物理接口":"未识别";Add(out,key+"_kind","接口类型",kind,name);next[name]=n;
  }
  for(const auto& name:names_)if(!next.count(name)){
   const auto key="net_"+Hex(name);for(const auto& field:{"rx_bytes_per_sec","tx_bytes_per_sec","rx_bytes","tx_bytes","elapsed_seconds","state","kind"}){Add(out,key+"_"+field,field,"",name);out[key+"_"+field].reason="interface_missing";}
  }
  networks_=next;
 }else if(group=="disk"){
  std::istringstream in(Read(root_+"/proc/self/mountinfo"));std::string line;std::set<std::string> seen,ids;unsigned count=0;
  while(std::getline(in,line)&&count<40){auto sep=line.find(" - ");if(sep==std::string::npos)continue;std::istringstream left(line.substr(0,sep)),right(line.substr(sep+3));std::string id,parent,dev,root,mount,opts,type,source;left>>id>>parent>>dev>>root>>mount>>opts;right>>type>>source;
   const std::set<std::string> types={"ext2","ext3","ext4","xfs","btrfs","f2fs","jffs2","ubifs","squashfs","overlay","overlayfs","vfat","exfat","ntfs","ntfs3","tmpfs","ramfs"};if(!types.count(type))continue;mount=UnescapeMount(mount);if(mount.size()>512||mount.find('\0')!=std::string::npos||mount.find("/dev")==0)continue;if(!seen.insert(dev+":"+root).second)continue;
   std::string key="disk_"+MountKey(mount);if(!ids.insert(key).second)continue;++count;struct statvfs stat;bool ok=statvfs((root_+mount).c_str(),&stat)==0;std::uint64_t total=0,used=0,avail=0;
   if(ok){std::uint64_t size=stat.f_frsize?stat.f_frsize:stat.f_bsize;ok=size>0&&stat.f_blocks<=static_cast<std::uint64_t>(INT64_MAX)/size&&stat.f_bfree<=stat.f_blocks&&stat.f_bavail<=stat.f_blocks;if(ok){total=stat.f_blocks*size;used=(stat.f_blocks-stat.f_bfree)*size;avail=stat.f_bavail*size;}}
   Add(out,key+"_usage","空间使用率",ok&&total?Num(100.0*used/total):"",mount);Add(out,key+"_used_bytes","已用空间",ok?Integer(used):"",mount);Add(out,key+"_total_bytes","总空间",ok?Integer(total):"",mount);Add(out,key+"_available_bytes","可用空间",ok?Integer(avail):"",mount);Add(out,key+"_kind","文件系统",type+(opts.find("ro")==0?" · 只读":"")+((type=="tmpfs"||type=="ramfs")?" · 内存盘":""),mount);
  }
 }
 return out;
}
TelemetryCollector::TelemetryCollector(const ClientConfig&config,SystemSampler* sampler)
 :config_(config),stop_(false),revision_(config.config_revision),network_seconds_(config.monitoring.at("network")),owned_sampler_("",config.network_interfaces),sampler_(sampler?sampler:&owned_sampler_){
 if(!sampler_->HardwareAttempts())sampler_->StartHardware(config_.monitoring.at("cpu"));
 if(!config_.monitoring.at("egress")){
  Metrics values;
  for(const auto& key:{"egress_ipv4","egress_ipv6"}){Add(values,key,key,"","","unknown");values[key].reason="collection_disabled";}
  Publish("egress",values);
 }
 if(!config_.cellular_json.empty()){CellularPlan p;if(ParseCellularPlan(config_.cellular_json,&p))cellular_.reset(new CellularCollector(p,revision_));}
 StartNetwork();
 builtin_=std::thread(&TelemetryCollector::Builtins,this);
 if(!config.collection_json.empty())templates_=std::thread(&TelemetryCollector::Templates,this);
 if(config_.monitoring.at("egress"))egress_=std::thread(&TelemetryCollector::Egress,this);
}
TelemetryCollector::~TelemetryCollector(){
 stop_=true;network_stop_=true;
 cellular_.reset();
 if(builtin_.joinable())builtin_.join();
 if(network_.joinable())network_.join();
 if(templates_.joinable())templates_.join();
 if(egress_.joinable())egress_.join();
 if(switches_.joinable())switches_.join();
}
void TelemetryCollector::Publish(const std::string&group,const Metrics&values){
 std::lock_guard<std::mutex> lock(mutex_);
 if(group=="template"||group=="egress"){
  for(const auto& item:values){pending_[group][item.first]=item.second;latest_[group][item.first]=item.second;}
 }else{pending_[group]=values;latest_[group]=values;}
}
void TelemetryCollector::StartNetwork(){
 network_stop_=false;
 if(network_seconds_){
  network_=std::thread(&TelemetryCollector::Network,this);
  if(!config_.switch_json.empty())switches_=std::thread(&TelemetryCollector::Switches,this);
 }else{
  Metrics values;Add(values,"net_collection_status","接口采集","","","unknown");
  values["net_collection_status"].reason="collection_disabled";Publish("network",values);
 }
}
void TelemetryCollector::ReconfigureNetwork(const ClientConfig&next){
 if(cellular_)cellular_->SetRevision(next.config_revision);
 network_stop_=true;
 if(network_.joinable())network_.join();
 if(switches_.joinable())switches_.join();
 sampler_->SetInterfaces(next.network_interfaces);
 network_seconds_=next.monitoring.at("network");
 if(!network_seconds_)sampler_->DisableSwitchCounters();
 {
  std::lock_guard<std::mutex> lock(mutex_);
  latest_.erase("network");latest_.erase("switch");pending_=latest_;
  revision_=next.config_revision;
 }
 StartNetwork();
}
void TelemetryCollector::Network(){
 while(!network_stop_){
  auto values=sampler_->Sample("network");
  for(auto&item:values)item.second.interval=network_seconds_;
  Publish("network",values);
  auto due=Clock::now()+std::chrono::seconds(network_seconds_);
  while(!network_stop_&&Clock::now()<due)std::this_thread::sleep_for(std::chrono::milliseconds(50));
 }
}
void TelemetryCollector::Builtins(){
 unsigned hardwareCount=0;std::map<std::string,Clock::time_point> due;
 while(!stop_){
  if(sampler_->HardwareAttempts()!=hardwareCount){hardwareCount=sampler_->HardwareAttempts();auto hardware=sampler_->Hardware();for(const auto&key:{"model","firmware"}){auto value=config_.properties.find(key);Add(hardware,key,key==std::string("model")?"设备型号":"固件版本",value==config_.properties.end()?"":value->second);hardware[key].reason=config_.builtin_errors[key];}Publish("hardware",hardware);}
  for(const auto&p:config_.monitoring){if(stop_)break;if(p.first!="egress"&&p.first!="network"&&p.second&&Clock::now()>=due[p.first]){auto m=sampler_->Sample(p.first);for(auto&v:m)v.second.interval=p.second;Publish(p.first,m);due[p.first]=Clock::now()+std::chrono::seconds(p.second);}}
  std::this_thread::sleep_for(std::chrono::milliseconds(50));
 }
}
Metrics SystemSampler::SampleSwitch(const std::string&json,const std::atomic<bool>*stop){if(!ports_)ports_.reset(new PortCounterSampler());return ports_->Collect(json,stop);}
void SystemSampler::DisableSwitchCounters(){if(ports_)ports_->Clear();}
void TelemetryCollector::Switches(){
 while(!network_stop_){
  auto values=sampler_->SampleSwitch(config_.switch_json,&network_stop_);
  for(auto&item:values)item.second.interval=network_seconds_;
  Publish("switch",values);
  auto due=Clock::now()+std::chrono::seconds(network_seconds_);
  while(!network_stop_&&Clock::now()<due)std::this_thread::sleep_for(std::chrono::milliseconds(50));
 }
}
void TelemetryCollector::Egress(){
 while(!stop_){
  for(int family:{4,6}){
   if(stop_)return;
   const std::string key=family==4?"egress_ipv4":"egress_ipv6";
   const auto result=NativeEgress(family,&stop_);if(stop_)return;
   Metrics m;Add(m,key,family==4?"出口 IPv4":"出口 IPv6",result.value,"",result.reason.empty()?"ok":"error");
   m[key].reason=result.reason;
   m[key].interval=config_.monitoring["egress"];Publish("egress",m);
  }
  auto due=Clock::now()+std::chrono::seconds(config_.monitoring["egress"]);while(!stop_&&Clock::now()<due)std::this_thread::sleep_for(std::chrono::milliseconds(50));
 }
}
void TelemetryCollector::Templates(){
 CollectionTemplate t;std::string error;if(!ParseCollectionTemplate(config_.collection_json,&t,&error))return;
 std::map<std::string,Clock::time_point> due;std::vector<std::string> keys;
 for(const auto&p:t.properties)if((p.second.interval||config_.config_revision)&&!(p.first=="hostname"&&config_.explicit_hostname)){keys.push_back(p.first);due[p.first]=Clock::now();}
 const auto cacheKey=t.id+":"+std::to_string(t.version)+":"+std::to_string(config_.template_generation);
 auto cached=sampler_->StaticProperties(cacheKey);
 for(const auto&p:cached)if(t.properties.count(p.first)&&t.properties[p.first].interval==0){Metrics m;m[p.first]=p.second;Publish("template",m);due[p.first]=Clock::time_point::max();}
 std::size_t cursor=0;
 while(!stop_&&!keys.empty()) {
  auto budget=Clock::now()+std::chrono::seconds(60);
  for(std::size_t visited=0;visited<keys.size();++visited) {
   if(stop_)return;
   if(Clock::now()>=budget)break;
   const auto key=keys[cursor];cursor=(cursor+1)%keys.size();const auto&p=t.properties[key];if(Clock::now()<due[key])continue;
   ExecTask task;task.command=p.command;task.timeout=std::min(p.timeout,static_cast<unsigned>(std::max<long long>(1,std::chrono::duration_cast<std::chrono::seconds>(budget-Clock::now()).count())));
   if(!p.config.empty()){task.type="router_config";task.config=p.config;}
   auto result=ExecuteExec(task,&stop_);if(stop_)return;auto value=Trim(result.stdout_text);JsonObject check;
   std::size_t limit=key=="libc"?64:key=="hostname"?255:key=="serial"||key=="model"||key=="firmware"||key=="kernel"?128:4096;
   bool valid=result.status=="success"&&!result.truncated&&!value.empty()&&value.size()<=limit&&value.find('\0')==std::string::npos&&ParseJsonObject("{\"v\":"+EscapeJsonString(value)+"}",&check,&error);
   if(key=="libc")for(unsigned char c:value)if(c>127)valid=false;
   Metrics m;Add(m,key,p.name,valid?value:"","",valid?"ok":"error");if(!valid)m[key].reason=result.status=="timeout"?"timeout":result.status!="success"?"command_failed":value.empty()?"empty":"invalid_output";m[key].interval=p.interval;if(p.interval==0)sampler_->SaveStaticProperty(cacheKey,key,m[key]);Publish("template",m);due[key]=p.interval?Clock::now()+std::chrono::seconds(p.interval):Clock::time_point::max();
  }
  std::this_thread::sleep_for(std::chrono::milliseconds(50));
 }
}
bool TelemetryCollector::Next(std::size_t max_payload,std::string*payload){
 if(cellular_&&cellular_->Next(max_payload,payload))return true;
 std::lock_guard<std::mutex>l(mutex_);if(pending_.empty())return false;
 auto group=pending_.begin();const std::size_t limit=std::min<std::size_t>(65536,max_payload);
 const std::string prefix="{\"config_revision\":"+Integer(revision_)+",\"event\":\"telemetry\",\"group\":"+EscapeJsonString(group->first)+",\"values\":{";
 std::string body;auto i=group->second.begin();
 while(i!=group->second.end()) {
  if(group->first!="template"&&group->second.size()>(group->first=="switch"?1024:256))break;
  const auto&p=*i;auto age=std::chrono::duration_cast<std::chrono::milliseconds>(Clock::now()-p.second.sampled).count();
  std::ostringstream item;item<<EscapeJsonString(p.first)<<":{\"name\":"<<EscapeJsonString(p.second.name)<<",\"value\":"<<EscapeJsonString(p.second.value)<<",\"unit\":"<<EscapeJsonString(p.second.unit)<<",\"status\":"<<EscapeJsonString(p.second.status)<<",\"reason\":"<<EscapeJsonString(p.second.reason)<<",\"entity\":"<<EscapeJsonString(p.second.entity)<<",\"interval_seconds\":"<<p.second.interval<<",\"age_ms\":"<<std::min<long long>(315360000000LL,std::max<long long>(0,age))<<'}';
  if(prefix.size()+body.size()+item.str().size()+3>limit){if(body.empty()&&group->first=="template"&&!i->second.value.empty()){i->second.value.clear();i->second.status="error";continue;}break;}
  if(!body.empty())body+=',';
  body+=item.str();++i;
 }
 if(i!=group->second.end()&&group->first!="template") {
  *payload=prefix+EscapeJsonString((group->first=="hardware"?"cpu":group->first=="network"?"net":group->first)+"_collection_status")+":{\"reason\":\"payload_limit\",\"name\":\"Collection exceeds payload capacity\",\"value\":\"\",\"unit\":\"text\",\"status\":\"error\"}}}";
  pending_.erase(group);return payload->size()<=limit;
 }
 *payload=prefix+body+"}}";group->second.erase(group->second.begin(),i);
 if(group->second.empty())pending_.erase(group);
 return true;
}
}
