#include "rmp/cellular.h"
#include "rmp/json.h"
#include <algorithm>
#include <cerrno>
#include <climits>
#include <cstring>
#include <dirent.h>
#include <fcntl.h>
#include <fstream>
#include <map>
#include <poll.h>
#include <set>
#include <sstream>
#include <sys/file.h>
#include <sys/ioctl.h>
#include <sys/stat.h>
#include <sys/sysmacros.h>
#include <termios.h>
#include <unistd.h>
namespace rmp {
namespace {
typedef std::chrono::steady_clock Clock;
bool Stopped(const std::atomic<bool>* s){return s&&s->load();}
std::string Trim(const std::string& s){auto a=s.find_first_not_of(" \t\r\n");return a==std::string::npos?"":s.substr(a,s.find_last_not_of(" \t\r\n")-a+1);}
bool Digits(const std::string& s){return !s.empty()&&s.find_first_not_of("0123456789")==std::string::npos;}
std::string ReadSmall(const std::string& path){std::ifstream f(path.c_str());char b[257]={};f.read(b,256);return f.gcount()<256?Trim(std::string(b,static_cast<std::size_t>(f.gcount()))):"";}
std::string Real(const std::string& path){char b[PATH_MAX];return realpath(path.c_str(),b)?b:"";}
bool CandidateName(const std::string& s){for(const std::string prefix:{"ttyUSB","ttyACM"})if(s.compare(0,prefix.size(),prefix)==0&&Digits(s.substr(prefix.size())))return true;return false;}
struct Candidate {CellularPort port;std::string node;dev_t dev;ino_t inode;dev_t filesystem;};
std::vector<Candidate> Discover(const std::string& root,bool* limited,std::string* reason){
 std::vector<Candidate> out;DIR* d=opendir((root+"/sys/class/tty").c_str());
 if(!d){*reason="sysfs_unavailable";return out;}
 std::vector<std::string> names;std::size_t seen=0;
 while(dirent* e=readdir(d)){if(++seen>4096){*limited=true;break;}if(CandidateName(e->d_name))names.push_back(e->d_name);}
 closedir(d);std::sort(names.begin(),names.end());
 for(const auto& name:names){
  const auto cls=root+"/sys/class/tty/"+name;
  std::string parent=Real(cls+"/device"),key;bool excluded=false;
  for(unsigned i=0;i<12&&!parent.empty()&&parent!="/";++i){
   std::string label=ReadSmall(parent+"/interface");std::transform(label.begin(),label.end(),label.begin(),[](unsigned char c){return c>='A'&&c<='Z'?static_cast<char>(c-'A'+'a'):static_cast<char>(c);});
   for(const std::string token:{"gps","gnss","nmea","diag","qdss","debug","download"})if(label.find(token)!=std::string::npos)excluded=true;
   if(label=="dm")excluded=true;
   if(!ReadSmall(parent+"/idVendor").empty()&&!ReadSmall(parent+"/idProduct").empty()){
    key=parent;if(!root.empty()&&key.compare(0,root.size(),root)==0)key.erase(0,root.size());break;
   }
   const auto slash=parent.find_last_of('/');if(slash==std::string::npos)break;parent.resize(slash);
  }
  // USB-backed TTY only; skip explicit diagnostic/GNSS interface descriptions.
  if(excluded||key.empty()||key.size()>256)continue;
  Candidate c;c.port.path="/dev/"+name;c.port.device_key=key;c.node=root+c.port.path;
  struct stat st;if(stat(c.node.c_str(),&st)!=0||!S_ISCHR(st.st_mode))continue;
  if(ReadSmall(cls+"/dev")!=std::to_string(major(st.st_rdev))+":"+std::to_string(minor(st.st_rdev)))continue;
  c.dev=st.st_rdev;c.inode=st.st_ino;c.filesystem=st.st_dev;
  if(out.size()==16){*limited=true;break;}out.push_back(c);
 }
 return out;
}
// A lock alone cannot detect existing users. Inspect descriptor metadata, fail
// closed on unreadable/over-limit process tables; do not read command lines/data.
std::string Occupied(const std::string& root,dev_t device,int own_fd,const std::atomic<bool>* stop,const Clock::time_point& end){
 DIR* processes=opendir((root+"/proc").c_str());if(!processes)return "occupancy_unknown";
 unsigned processes_seen=0,descriptors=0;std::string result;
 while(dirent* p=readdir(processes)){
  if(!Digits(p->d_name))continue;
  if(Stopped(stop)){result="cancelled";break;}
  if(++processes_seen>4096){result="occupancy_limit";break;}
  const auto directory=root+"/proc/"+p->d_name+"/fd";
  DIR* fds=opendir(directory.c_str());
  if(!fds){if(errno!=ENOENT&&errno!=ESRCH){result="occupancy_unknown";break;}continue;}
  while(dirent* entry=readdir(fds)){
   if(!Digits(entry->d_name))continue;
   if(Stopped(stop)||Clock::now()>=end){result=Stopped(stop)?"cancelled":"occupancy_timeout";break;}
   if(++descriptors>65536){result="occupancy_limit";break;}
   if(std::string(p->d_name)==std::to_string(getpid())&&std::string(entry->d_name)==std::to_string(own_fd))continue;
   struct stat st;if(stat((directory+"/"+entry->d_name).c_str(),&st)!=0){if(errno!=ENOENT&&errno!=ESRCH){result="occupancy_unknown";break;}continue;}
   if(S_ISCHR(st.st_mode)&&st.st_rdev==device){result="port_busy";break;}
  }
  closedir(fds);if(!result.empty())break;
 }
 closedir(processes);return result;
}
class PortLease {
public:
 int fd=-1;bool exclusive=false,configured=false;struct termios original;
 ~PortLease(){if(fd>=0){if(configured)tcsetattr(fd,TCSANOW,&original);if(exclusive)ioctl(fd,TIOCNXCL);flock(fd,LOCK_UN);close(fd);}}
 std::string Open(const Candidate& c,const std::string& root,const std::atomic<bool>* stop,const Clock::time_point& end){
  auto busy=Occupied(root,c.dev,-1,stop,end);if(!busy.empty())return busy;
  fd=open(c.node.c_str(),O_RDWR|O_NOCTTY|O_NONBLOCK|O_CLOEXEC);if(fd<0)return errno==EBUSY?"port_busy":errno==EACCES?"permission_denied":"open_failed";
  struct stat st;if(fstat(fd,&st)!=0||st.st_rdev!=c.dev||st.st_ino!=c.inode||st.st_dev!=c.filesystem)return "port_changed";
#ifdef TIOCGEXCL
  int held=0;if(ioctl(fd,TIOCGEXCL,&held)==0&&held)return "port_busy";
#endif
  if(flock(fd,LOCK_EX|LOCK_NB)!=0)return "port_busy";
  if(ioctl(fd,TIOCEXCL)!=0)return "exclusive_unavailable";
  exclusive=true;
  busy=Occupied(root,c.dev,fd,stop,end);if(!busy.empty())return busy;
  if(tcgetattr(fd,&original)!=0)return "not_tty";
  struct termios t=original;
  // Preserve speed and modem-control flags: no baud sweep, DTR/RTS or HUPCL changes.
  t.c_iflag&=~(IGNBRK|BRKINT|PARMRK|ISTRIP|INLCR|IGNCR|ICRNL|IXON|IXOFF);
  t.c_oflag&=~OPOST;t.c_lflag&=~(ECHO|ECHONL|ICANON|ISIG|IEXTEN);
  t.c_cc[VMIN]=0;t.c_cc[VTIME]=0;
  if(tcsetattr(fd,TCSANOW,&t)!=0)return "configure_failed";
  configured=true;return "";
 }
};
bool Pause(int fd,short events,const Clock::time_point& end,const std::atomic<bool>* stop){
 while(!Stopped(stop)&&Clock::now()<end){struct pollfd p={fd,events,0};auto left=std::chrono::duration_cast<std::chrono::milliseconds>(end-Clock::now()).count();
  const int n=poll(&p,1,static_cast<int>(std::min<long long>(50,std::max<long long>(1,left))));
  if(n>0)return (p.revents&events)!=0;
  if(n<0&&errno!=EINTR)return false;
 }return false;
}
bool Quiet(int fd,const std::atomic<bool>* stop,const Clock::time_point& deadline){
 auto quiet=Clock::now();std::size_t count=0;
 while(!Stopped(stop)&&Clock::now()<deadline){
  char b[256];auto n=read(fd,b,sizeof b);
  if(n>0){count+=static_cast<std::size_t>(n);quiet=Clock::now();if(count>4096)return false;}
  else if(n<0&&errno!=EAGAIN&&errno!=EWOULDBLOCK&&errno!=EINTR)return false;
  if(Clock::now()-quiet>=std::chrono::milliseconds(100))return true;
  std::this_thread::sleep_for(std::chrono::milliseconds(10));
 }return false;
}
bool Unsolicited(const std::string& line){return line=="RING"||line=="RDY"||line=="SMS Ready"||line=="Call Ready"||line=="PB DONE"||line=="NO CARRIER"||(!line.empty()&&(line[0]=='+'||line[0]=='^'||line[0]=='%'));}
AtIdentity Query(int fd,const std::string& command,unsigned timeout,const std::atomic<bool>* stop,const Clock::time_point& round,const std::string& expected="",std::size_t capacity=1024){
 AtIdentity r;r.command=command;r.status="timeout";
 auto end=std::min(round,Clock::now()+std::chrono::milliseconds(timeout));const std::string wire=command+"\r";std::size_t sent=0,received=0;
 while(sent<wire.size()&&!Stopped(stop)&&Clock::now()<end){
  auto n=write(fd,wire.data()+sent,wire.size()-sent);if(n>0)sent+=static_cast<std::size_t>(n);
  else if(n<0&&errno!=EAGAIN&&errno!=EWOULDBLOCK&&errno!=EINTR){r.status="io_error";return r;}
  else if(!Pause(fd,POLLOUT,end,stop))break;
 }
 if(sent!=wire.size()){if(Stopped(stop))r.status="cancelled";return r;}
 std::string line,body;
 while(!Stopped(stop)&&Clock::now()<end){
  if(!Pause(fd,POLLIN,end,stop))break;
  char b[256];auto n=read(fd,b,sizeof b);
  if(n<0&&(errno==EAGAIN||errno==EWOULDBLOCK||errno==EINTR))continue;
  if(n<=0){r.status="io_error";return r;}
  received+=static_cast<std::size_t>(n);if(received>4096){r.status="overflow";return r;}
  for(ssize_t i=0;i<n;++i){unsigned char ch=b[i];
   if(ch=='\r'||ch=='\n'){
    auto text=Trim(line);line.clear();if(text.empty()||text==command)continue;
    if(text=="OK"){r.status="ok";r.value=body;return r;}
    if(text=="ERROR"||text.compare(0,11,"+CME ERROR:")==0||text.compare(0,11,"+CMS ERROR:")==0){r.status="rejected";return r;}
    const bool imeiPrefix=text.compare(0,6,"+CGSN:")==0||text.compare(0,5,"+GSN:")==0;
    if(Unsolicited(text)&&!(command!="ATI"&&imeiPrefix)&&!(expected.size()&&text.compare(0,expected.size(),expected)==0))continue;
    if(body.size()+text.size()+1>capacity){r.status="overflow";return r;}
    if(!body.empty())body+='\n';
    body+=text;
   }else if((ch>=32&&ch<=126)||ch=='\t'){line+=static_cast<char>(ch);}
   else{r.status="invalid_response";return r;}
  }
 }
 if(Stopped(stop))r.status="cancelled";
 return r;
}
bool CanContinue(const AtIdentity& q){return q.status=="ok"||q.status=="rejected"||q.status=="invalid_value";}
void ProbePort(const Candidate& c,CellularPort* p,const std::string& root,const std::atomic<bool>* stop,unsigned timeout,const Clock::time_point& end,bool telemetry,bool details){
 PortLease lease;auto failure=lease.Open(c,root,stop,end);
 if(!failure.empty()){p->status=failure=="port_busy"?"busy":"error";p->reason=failure;return;}
 if(!Quiet(lease.fd,stop,std::min(end,Clock::now()+std::chrono::milliseconds(500)))){p->status="error";p->reason="unsolicited_data";return;}
 const auto handshake=Query(lease.fd,"AT",timeout,stop,end);
 if(handshake.status!="ok"){p->status="not_at";p->reason=handshake.status;return;}
 p->ati=Query(lease.fd,"ATI",timeout,stop,end);
 if(p->ati.status=="ok"&&p->ati.value.empty())p->ati.status="invalid_value";
 if(CanContinue(p->ati))for(const std::string command:{"AT+CGSN","AT+CGSN=1","AT+GSN"}){
  p->imei=Query(lease.fd,command,timeout,stop,end);
  if(p->imei.status=="ok"){p->imei.value=ParseIMEI(p->imei.value);if(p->imei.value.empty())p->imei.status="invalid_value";else break;}
  if(!CanContinue(p->imei))break;
 }
 p->status=p->ati.status=="ok"&&p->imei.status=="ok"?"ok":"partial";
 if(p->status=="partial")p->reason="identity_incomplete";
 if(telemetry && p->status=="ok" && p->ati.value.find("Manufacturer: Fibocom Wireless Inc.")!=std::string::npos
    && ("\n"+p->ati.value+"\n").find("\nModel: FM160-CN\n")!=std::string::npos){
  p->profile="fibocom-fm160-v1";
  const std::pair<const char*,const char*> commands[]={
   {"AT+CPIN?","+CPIN:"},{"AT+CCID","+CCID:"},{"AT+CIMI","+CIMI:"},
   {"AT+COPS?","+COPS:"},{"AT+CEREG?","+CEREG:"},{"AT+C5GREG?","+C5GREG:"},
   {"AT+CSQ","+CSQ:"},{"AT+CESQ","+CESQ:"}};
  bool proceed=true;
  for(const auto& entry:commands){AtIdentity q;q.command=entry.first;
   if(proceed){q=Query(lease.fd,entry.first,timeout,stop,end,entry.second);proceed=CanContinue(q);}
   p->queries.push_back(q);
  }
  if(details){
   p->profile="fibocom-fm160-details-v1";
   const std::pair<const char*,const char*> extra[]={{"AT+CBC","+CBC:"},{"AT+MTSM?","+MTSM:"},{"AT+MTSM=1","+MTSM:"},{"AT+MTSM=6","+MTSM:"},{"AT+MTSM=7","+MTSM:"},{"AT+CGATT?","+CGATT:"},{"AT+CGACT?","+CGACT:"},{"AT+CGDCONT?","+CGDCONT:"},{"AT+CGPADDR","+CGPADDR:"},{"AT+CGCONTRDP","+CGCONTRDP:"},{"AT+GTACT?","+GTACT:"},{"AT+GTACT=?","+GTACT:"},{"AT+GTCELLLOCK?","+GTCELLLOCK:"},{"AT+GTCAINFO?","+GTCAINFO:"},{"AT+GTCELLINFO?","+GTCELLINFO:"},{"AT+GTCCINFO?","+GTCCINFO:"}};
   bool temperatureSafe=false;
   for(const auto& entry:extra){AtIdentity q;q.command=entry.first;
    const bool temp=q.command.compare(0,8,"AT+MTSM=")==0;
    if(proceed&&(!temp||temperatureSafe)){
     const unsigned budget=q.command=="AT+GTCCINFO?"?15000:q.command=="AT+GTCAINFO?"?3000:timeout;
     q=Query(lease.fd,entry.first,budget,stop,end,entry.second,4096);proceed=CanContinue(q);
     if(q.command=="AT+MTSM?")temperatureSafe=q.status=="ok"&&(q.value=="+MTSM: 0"||q.value=="+MTSM: 1"||q.value=="+MTSM: 6"||q.value=="+MTSM: 7");
    }
    p->queries.push_back(q);
   }
  }
 }
 p->sampled=Clock::now();
}
std::uint64_t Age(Clock::time_point at){auto n=std::chrono::duration_cast<std::chrono::milliseconds>(Clock::now()-at).count();return static_cast<std::uint64_t>(std::max<long long>(0,std::min<long long>(315360000000LL,n)));}
std::string IdentityJson(const AtIdentity& q){return "{\"command\":"+EscapeJsonString(q.command)+",\"status\":"+EscapeJsonString(q.status)+",\"value\":"+EscapeJsonString(q.value)+"}";}
}
bool ParseCellularPlan(const std::string& bytes,CellularPlan* plan){
 JsonObject v;std::string e;if(!ParseJsonObject(bytes,&v,&e))return false;
 CellularPlan p;for(const auto& item:v){if(item.first=="details"){if(item.second.type!=JsonType::kBoolean)return false;p.details=item.second.bool_value;continue;}if(item.first=="telemetry"){if(item.second.type!=JsonType::kBoolean)return false;p.telemetry=item.second.bool_value;continue;}if(item.first!="interval_seconds"||item.second.type!=JsonType::kUnsignedInteger||item.second.unsigned_value<10||item.second.unsigned_value>86400)return false;p.interval=static_cast<unsigned>(item.second.unsigned_value);}
 if(p.details&&!p.telemetry)return false;
 *plan=p;return true;
}
std::string ParseIMEI(const std::string& text){
 std::istringstream lines(text);std::string line,value;
 while(std::getline(lines,line)){
  line=Trim(line);if(line.empty())continue;
  for(const std::string prefix:{"+CGSN:","+GSN:","IMEI:"})if(line.compare(0,prefix.size(),prefix)==0){line=Trim(line.substr(prefix.size()));break;}
  if(line.size()>=2&&line.front()=='"'&&line.back()=='"')line=line.substr(1,line.size()-2);
  if(line.size()!=15||!Digits(line)||!value.empty())return "";
  value=line;
 }return value;
}
CellularObservation SampleCellular(const std::string& root,const std::atomic<bool>* stop,unsigned* cursor,unsigned timeout,unsigned round_ms,bool telemetry,bool details){
 CellularObservation observation;observation.telemetry=telemetry;observation.details=details;auto candidates=Discover(root,&observation.limited,&observation.reason);
 if(candidates.empty()){if(!observation.reason.empty())observation.status="error";return observation;}
 const auto deadline=Clock::now()+std::chrono::milliseconds(round_ms);std::set<std::string> complete;
 for(auto& c:candidates){c.port.ati.command="ATI";observation.ports.push_back(c.port);}
 const unsigned start=*cursor%candidates.size();unsigned attempted=0;
 for(unsigned n=0;n<candidates.size();++n){
  const unsigned i=(start+n)%candidates.size();auto& p=observation.ports[i];
  if(complete.count(p.device_key)){p.status="alternate";p.reason="device_port_selected";continue;}
  if(Stopped(stop)||Clock::now()>=deadline){observation.limited=true;continue;}
  ProbePort(candidates[i],&p,root,stop,timeout,deadline,telemetry,details);p.sampled=Clock::now();attempted=n+1;
  if(p.status=="ok")complete.insert(p.device_key);
 }
 *cursor=(start+std::max(1u,attempted))%candidates.size();
 // One selected AT endpoint per USB device; independent USB devices remain separate.
 std::map<std::string,std::size_t> best;
 for(std::size_t i=0;i<observation.ports.size();++i){const auto& p=observation.ports[i];if(p.status!="ok"&&p.status!="partial")continue;
  auto old=best.find(p.device_key);if(old==best.end()||(observation.ports[old->second].status!="ok"&&p.status=="ok"))best[p.device_key]=i;
 }
 observation.status=best.empty()?"unavailable":"ok";
 for(const auto& selected:best){auto& p=observation.ports[selected.second];p.selected=true;if(p.status!="ok")observation.status="partial";}
 if(observation.limited&&observation.status=="ok")observation.status="partial";
 observation.sampled=Clock::now();return observation;
}
std::string CellularEvent(const CellularObservation& v,std::uint64_t revision,unsigned interval,std::size_t limit){
 std::string prefix="{\"event\":"+EscapeJsonString(v.details?"cellular_details":v.telemetry?"cellular_telemetry":"cellular")+",\"config_revision\":"+std::to_string(revision)+",\"interval_seconds\":"+std::to_string(interval)+",\"age_ms\":"+std::to_string(Age(v.sampled));
 std::string s=prefix+",\"status\":"+EscapeJsonString(v.status)+",\"reason\":"+EscapeJsonString(v.reason)+",\"limited\":"+(v.limited?"true":"false")+",\"ports\":[";
 for(const auto& p:v.ports){if(&p!=&v.ports.front())s+=',';
  s+="{\"path\":"+EscapeJsonString(p.path)+",\"device_key\":"+EscapeJsonString(p.device_key)+",\"status\":"+EscapeJsonString(p.status)+",\"reason\":"+EscapeJsonString(p.reason)+",\"selected\":"+(p.selected?"true":"false")+",\"age_ms\":"+std::to_string(Age(p.sampled))+",\"ati\":"+IdentityJson(p.ati)+",\"imei\":"+IdentityJson(p.imei);
  if(v.telemetry){s+=",\"profile\":"+EscapeJsonString(p.profile)+",\"queries\":[";for(std::size_t i=0;i<p.queries.size();++i){if(i)s+=',';s+=IdentityJson(p.queries[i]);}s+="]";}s+="}";
 }
 s+="]}";if(s.size()<=std::min<std::size_t>(65536,limit))return s;
 s=prefix+",\"status\":\"error\",\"reason\":\"payload_limit\",\"limited\":true,\"ports\":[]}";return s.size()<=limit?s:"";
}
CellularCollector::CellularCollector(const CellularPlan& p,std::uint64_t r):plan_(p),revision_(r){worker_=std::thread(&CellularCollector::Run,this);}
CellularCollector::~CellularCollector(){stop_=true;if(worker_.joinable())worker_.join();}
void CellularCollector::SetRevision(std::uint64_t r){std::lock_guard<std::mutex> lock(mutex_);revision_=r;pending_=observed_;}
void CellularCollector::Run(){unsigned cursor=0;while(!stop_){
 auto v=SampleCellular("",&stop_,&cursor,1500,15000,plan_.telemetry,plan_.details);if(stop_)break;
 {std::lock_guard<std::mutex> lock(mutex_);observation_=std::move(v);observed_=true;pending_=true;}
 const auto next=Clock::now()+std::chrono::seconds(plan_.interval);while(!stop_&&Clock::now()<next)std::this_thread::sleep_for(std::chrono::milliseconds(50));
}}
bool CellularCollector::Next(std::size_t limit,std::string* out){std::lock_guard<std::mutex> lock(mutex_);if(!pending_)return false;*out=CellularEvent(observation_,revision_,plan_.interval,limit);if(out->empty())return false;pending_=false;return true;}
}
