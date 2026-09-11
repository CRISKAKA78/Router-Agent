#include "rmp/neighbors.h"
#include "rmp/json.h"
#include <algorithm>
#include <arpa/inet.h>
#include <dirent.h>
#include <fcntl.h>
#include <fstream>
#include <sstream>
#include <set>
#include <cstring>
#include <cstdio>
#include <ctime>
#include <linux/rtnetlink.h>
#include <net/if.h>
#include <poll.h>
#include <unistd.h>
namespace rmp {
namespace {
std::string Read(const std::string&p){std::ifstream f(p.c_str());std::string s(16384,'\0');f.read(&s[0],s.size());s.resize(static_cast<std::size_t>(f.gcount()));return s;}
std::vector<std::string> Names(const std::string&p){std::vector<std::string> v;DIR*d=opendir(p.c_str());if(!d)return v;while(auto*e=readdir(d)){std::string n=e->d_name;if(n!="."&&n!=".."&&v.size()<64)v.push_back(n);}closedir(d);std::sort(v.begin(),v.end());return v;}
std::string Master(const std::string&p){char b[512];auto n=readlink(p.c_str(),b,sizeof(b)-1);if(n<=0)return "";b[n]=0;std::string s=b;return s.substr(s.find_last_of('/')+1);}
std::string Array(const std::vector<std::string>&v){std::string out="[";for(const auto&s:v){if(out.size()>1)out+=",";out+=EscapeJsonString(s);}return out+"]";}
// RTM_GETADDR reports every IPv4 prefix (including aliases); no shell or getifaddrs dependency.
std::map<std::string,std::vector<std::string>> Addresses(){
 std::map<std::string,std::vector<std::string>> out;int fd;
 {std::lock_guard<std::mutex> g(ExecForkMutex());fd=socket(AF_NETLINK,SOCK_RAW,NETLINK_ROUTE);if(fd<0)return out;if(fcntl(fd,F_SETFD,FD_CLOEXEC)<0||fcntl(fd,F_SETFL,O_NONBLOCK)<0){close(fd);return out;}}
 struct {nlmsghdr h; ifaddrmsg a;} req={};req.h.nlmsg_len=NLMSG_LENGTH(sizeof(ifaddrmsg));req.h.nlmsg_type=RTM_GETADDR;req.h.nlmsg_flags=NLM_F_REQUEST|NLM_F_DUMP;req.h.nlmsg_seq=1;req.a.ifa_family=AF_INET;
 sockaddr_nl kernel={};kernel.nl_family=AF_NETLINK;
 if(sendto(fd,&req,req.h.nlmsg_len,0,reinterpret_cast<sockaddr*>(&kernel),sizeof(kernel))<0){close(fd);return out;}
 auto until=std::chrono::steady_clock::now()+std::chrono::milliseconds(500);bool done=false;unsigned bytes=0;
 while(!done&&std::chrono::steady_clock::now()<until){pollfd p={fd,POLLIN,0};if(poll(&p,1,20)<=0)continue;alignas(nlmsghdr) char b[32768];sockaddr_nl from={};socklen_t fl=sizeof(from);int len=recvfrom(fd,b,sizeof(b),MSG_TRUNC,reinterpret_cast<sockaddr*>(&from),&fl);if(len<=0||len>static_cast<int>(sizeof(b))||from.nl_pid!=0)break;bytes+=len;if(bytes>1048576)break;
 for(auto*h=reinterpret_cast<nlmsghdr*>(b);NLMSG_OK(h,len);h=NLMSG_NEXT(h,len)){if(h->nlmsg_seq!=1)continue;if(h->nlmsg_type==NLMSG_DONE){done=(h->nlmsg_flags&NLM_F_DUMP_INTR)==0;break;}if(h->nlmsg_type==NLMSG_ERROR){close(fd);return {};}
 if(h->nlmsg_type!=RTM_NEWADDR||h->nlmsg_len<NLMSG_LENGTH(sizeof(ifaddrmsg)))continue;auto*a=reinterpret_cast<ifaddrmsg*>(NLMSG_DATA(h));if(a->ifa_family!=AF_INET||a->ifa_prefixlen>32)continue;char name[IF_NAMESIZE];if(!if_indextoname(a->ifa_index,name))continue;
 in_addr local={};bool found=false;int left=h->nlmsg_len-NLMSG_LENGTH(sizeof(ifaddrmsg));for(auto*r=IFA_RTA(a);RTA_OK(r,left);r=RTA_NEXT(r,left)){if(r->rta_type==IFA_LOCAL&&RTA_PAYLOAD(r)==4){std::memcpy(&local,RTA_DATA(r),4);found=true;}}
 if(found&&out[name].size()<8){char ip[INET_ADDRSTRLEN];if(inet_ntop(AF_INET,&local,ip,sizeof(ip)))out[name].push_back(std::string(ip)+"/"+std::to_string(a->ifa_prefixlen));}
 }}close(fd);return done?out:std::map<std::string,std::vector<std::string>>{};
}
}
std::vector<NeighborNetwork> DiscoverNeighborNetworks(const std::string&root){
 auto addresses=Addresses();std::vector<NeighborNetwork> out;
 for(const auto&name:Names(root+"/sys/class/net")){NeighborNetwork n;n.interface=name;auto base=root+"/sys/class/net/"+name;n.master=Master(base+"/master");n.bridge=access((base+"/bridge").c_str(),F_OK)==0;n.vlan=access((root+"/proc/net/vlan/"+name).c_str(),F_OK)==0;n.ipv4=addresses[name];n.ports=Names(base+"/brif");auto type=Read(base+"/type");
 n.reason=!n.master.empty()?"bridge_member_use_master":(type!="1\n"&&type!="1")?"not_ethernet":Read(base+"/bridge/vlan_filtering").find('1')==0?"vlan_bridge_requires_l3_interface":"";
 n.eligible=n.reason.empty();if(n.eligible&&n.ipv4.empty())n.reason="interface_ipv4_unavailable";out.push_back(n);
 }return out;
}
bool FNR100Environment(const std::vector<NeighborNetwork>&networks){for(const auto&n:networks)if(n.interface=="br0"&&n.bridge&&n.master.empty()&&n.eligible){for(const auto*p:{"eth0","vlan3","ath0","ath1"})if(std::find(n.ports.begin(),n.ports.end(),p)==n.ports.end())return false;return true;}return false;}
bool ParseFNR100ARL(const std::string&text,std::vector<NeighborRow>*out){
 out->clear();if(text.empty()||text.size()>32768)return false;std::istringstream lines(text);std::string line;unsigned count=0;bool seen=false;
 while(std::getline(lines,line)){if(line.find_first_not_of(" \t\r")==std::string::npos)continue;if(++count>512)return false;
 unsigned m[6],bits,vid,status;int used=0;
 if(std::sscanf(line.c_str()," MAC: %2x:%2x:%2x:%2x:%2x:%2x PORTMAP: 0x%x VID: 0x%x STATUS: 0x%x %n",&m[0],&m[1],&m[2],&m[3],&m[4],&m[5],&bits,&vid,&status,&used)!=9||used==0||line.substr(used).find_first_not_of(" \t\r")!=std::string::npos||bits==0||(bits&~0x3fU)||vid>4094){out->clear();return false;}
 seen=true;if((m[0]&1)||!(m[0]|m[1]|m[2]|m[3]|m[4]|m[5]))continue;
 // Only the verified br0 VLAN3 path. Other VLANs are not leaked into this domain.
 if(vid!=3)continue;char mac[18];::snprintf(mac,sizeof(mac),"%02x:%02x:%02x:%02x:%02x:%02x",m[0],m[1],m[2],m[3],m[4],m[5]);
 const char*ports[]={"lan1","lan2","lan3","lan4","wan"};for(unsigned i=0;i<5;++i)if(bits&(2U<<i)){NeighborRow r;r.mac=mac;r.port=ports[i];r.source="fdb";r.state="mac_only";r.interface="br0";out->push_back(r);}
 }return seen;
}
bool ReadFNR100(const std::string&root,const std::atomic<bool>*cancel,std::vector<NeighborRow>*rows,std::string*raw){
 if(!FNR100Environment(DiscoverNeighborNetworks(root)))return false;
 ExecTask t;t.type="exec";t.timeout=2;t.command="swconfig list";auto list=ExecuteExec(t,cancel);bool found=false;std::istringstream ls(list.stdout_text);std::string line;while(std::getline(ls,line)){std::istringstream f(line);std::string a,b;f>>a>>b;if(a=="Found:"&&b=="switch0")found=true;}
 if(list.status!="success"||list.truncated||!found)return false;t.timeout=3;t.command="swconfig dev switch0 get dump_arl";auto r=ExecuteExec(t,cancel);*raw=r.stdout_text.substr(0,4096);return r.status=="success"&&!r.truncated&&ParseFNR100ARL(r.stdout_text,rows);
}
bool ParseNeighborInspect(const std::string&json,RouterConfigParams*out){JsonObject o;std::string e;if(!ParseJsonObject(json,&o,&e)||o.size()!=3||o["vendor_test"].type!=JsonType::kBoolean||o["session_id"].type!=JsonType::kString||o["session_id"].string_value.empty()||o["session_id"].string_value.size()>128||o["config_revision"].type!=JsonType::kUnsignedInteger)return false;(*out)["session_id"]=o["session_id"].string_value;(*out)["config_revision"]=std::to_string(o["config_revision"].unsigned_value);(*out)["vendor_test"]=o["vendor_test"].bool_value?"true":"false";return true;}
ExecResult InspectNeighbors(const ExecTask&t,const std::atomic<bool>*cancel){
 ExecResult r;r.task_id=t.task_id;r.started_at=static_cast<std::uint64_t>(std::time(NULL));auto networks=DiscoverNeighborNetworks();std::string items;
 for(const auto&n:networks){if(!items.empty())items+=",";items+="{\"interface\":"+EscapeJsonString(n.interface)+",\"bridge\":"+(n.bridge?"true":"false")+",\"vlan\":"+(n.vlan?"true":"false")+",\"master\":"+EscapeJsonString(n.master)+",\"eligible\":"+(n.eligible?"true":"false")+",\"reason\":"+EscapeJsonString(n.reason)+",\"ipv4\":"+Array(n.ipv4)+",\"ports\":"+Array(n.ports)+"}";}
 std::vector<NeighborRow> rows;std::string raw,status="not_tested";bool candidate=FNR100Environment(networks);if(t.config.at("vendor_test")=="true")status=ReadFNR100("",cancel,&rows,&raw)?"verified":"failed";
 std::set<std::string> ports;for(const auto&row:rows)ports.insert(row.port);std::vector<std::string> portList(ports.begin(),ports.end());r.status=cancel->load()?"failed":"success";r.exit_code=r.status=="success"?0:1;
 r.stdout_text="{\"networks\":["+items+"],\"preset\":"+EscapeJsonString(candidate?"fnr100":"")+",\"preset_status\":"+EscapeJsonString(status)+",\"raw_summary\":"+EscapeJsonString(raw)+",\"ports\":"+Array(portList)+"}";r.finished_at=static_cast<std::uint64_t>(std::time(NULL));return r;
}
} // namespace rmp
