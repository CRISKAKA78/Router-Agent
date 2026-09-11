#include "rmp/network_agent.h"
#include "rmp/json.h"
#include <algorithm>
#include <cerrno>
#include <climits>
#include <cstring>
#include <ctime>
#include <fcntl.h>
#include <fstream>
#include <mutex>
#include <poll.h>
#include <signal.h>
#include <chrono>
#include <sstream>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
extern char** environ;
namespace rmp {
namespace {
std::mutex network_mutex;
bool UUID(const std::string& s){if(s.size()!=36)return false;for(std::size_t i=0;i<s.size();++i){if(i==8||i==13||i==18||i==23){if(s[i]!='-')return false;}else if(!((s[i]>='0'&&s[i]<='9')||(s[i]>='a'&&s[i]<='f')))return false;}return true;}
bool Directory(const std::string& s){if(s.size()<8||s.size()>160||s[0]!='/'||s.back()=='/'||s=="/tmp/root"||s.find("//")!=std::string::npos)return false;std::istringstream parts(s);std::string p;while(std::getline(parts,p,'/'))if(p=="."||p=="..")return false;for(char c:s)if(!((c>='a'&&c<='z')||(c>='A'&&c<='Z')||(c>='0'&&c<='9')||c=='/'||c=='-'||c=='_'||c=='.'))return false;return true;}
bool SafeParents(const std::string& root,bool create){std::size_t i=1;do{auto next=root.find('/',i);auto p=root.substr(0,next);struct stat st;if(lstat(p.c_str(),&st)!=0){if(!create||errno!=ENOENT||mkdir(p.c_str(),0700)!=0)return false;if(lstat(p.c_str(),&st)!=0)return false;}if(!S_ISDIR(st.st_mode))return false;if(next==std::string::npos){if(st.st_uid!=geteuid()||(st.st_mode&0022)!=0)return false;return true;}i=next+1;}while(true);}
std::string ExistingCore(const std::string& root){const std::string paths[]={root+"/easytier-core","/usr/sbin/easytier-core","/usr/bin/easytier-core","/opt/easytier/easytier-core"};for(auto& p:paths){char real[PATH_MAX];struct stat st;if(realpath(p.c_str(),real)&&stat(real,&st)==0&&S_ISREG(st.st_mode)&&access(real,X_OK)==0)return real;}return "";}
std::string StartTime(pid_t pid){std::ifstream f(("/proc/"+std::to_string(pid)+"/stat").c_str());std::string line;std::getline(f,line);auto end=line.rfind(')');if(end==std::string::npos)return "";std::istringstream fields(line.substr(end+2));std::string v;for(int i=3;i<=22;++i)if(!(fields>>v))return "";return v;}
bool Running(const std::string& root,const std::string& machine,std::string* saved_machine){struct stat st;auto file=root+"/agent.pid";if(lstat(file.c_str(),&st)!=0||!S_ISREG(st.st_mode)||st.st_size>1024)return false;std::ifstream f(file.c_str());long pid=0;std::string started,owner;f>>pid>>started>>owner;if(pid<=1||pid>INT_MAX||!UUID(owner)||StartTime(static_cast<pid_t>(pid))!=started)return false;int status=0;if(waitpid(static_cast<pid_t>(pid),&status,WNOHANG)==pid)return false;*saved_machine=owner;return owner==machine;}
bool SavePID(const std::string& root,pid_t pid,const std::string& machine){std::string started=StartTime(pid);if(started.empty())return false;auto tmp=root+"/agent.pid.new";int fd=open(tmp.c_str(),O_WRONLY|O_CREAT|O_EXCL|O_NOFOLLOW,0600);if(fd<0)return false;std::string v=std::to_string(pid)+" "+started+" "+machine+"\n";ssize_t n=write(fd,v.data(),v.size());bool ok=n==static_cast<ssize_t>(v.size())&&fsync(fd)==0;close(fd);if(ok)ok=rename(tmp.c_str(),(root+"/agent.pid").c_str())==0;if(!ok)unlink(tmp.c_str());return ok;}
ExecResult Program(const ExecTask& task,const std::vector<std::string>& args,const std::atomic<bool>* stop){ExecTask p=task;p.type="internal_program";p.arguments=args;p.timeout=5;p.command.clear();return ExecuteExec(p,stop);}
bool VersionOK(const std::string& s){return s.find("easytier-core 2.6.4")==0&&(s.size()==19||s[19]=='-'||s[19]=='\n'||s[19]=='\r'||s[19]==' ');}
bool Install(const std::string& root,const ExecTask& task,const std::atomic<bool>* stop){
 auto source=root+"/package";int in=open(source.c_str(),O_RDONLY|O_NOFOLLOW);if(in<0)return false;struct stat st;unsigned char header[4]={};bool valid=fstat(in,&st)==0&&S_ISREG(st.st_mode)&&st.st_size>4&&st.st_size<=64*1024*1024&&read(in,header,4)==4&&header[0]==0x7f&&header[1]=='E'&&header[2]=='L'&&header[3]=='F';if(!valid){close(in);return false;}lseek(in,0,SEEK_SET);auto tmp=root+"/easytier-core.new";int out=open(tmp.c_str(),O_WRONLY|O_CREAT|O_EXCL|O_NOFOLLOW,0700);if(out<0){close(in);return false;};char buffer[16384];ssize_t n;bool ok=true;while((n=read(in,buffer,sizeof(buffer)))>0){ssize_t used=0;while(used<n){ssize_t w=write(out,buffer+used,n-used);if(w<0&&errno==EINTR)continue;if(w<=0){ok=false;break;}used+=w;}if(!ok)break;}if(n<0)ok=false;if(fsync(out)!=0)ok=false;close(in);close(out);if(ok){auto checked=Program(task,{tmp,"--version"},stop);ok=checked.status=="success"&&VersionOK(checked.stdout_text);}if(ok)ok=rename(tmp.c_str(),(root+"/easytier-core").c_str())==0;if(!ok)unlink(tmp.c_str());return ok;
}
// Double fork + exec: the engine is independent of the Probe control session.
// All arguments and descriptor limits are prepared before fork; child path is
// async-signal-safe, with an exec-status pipe so missing binaries are not success.
// Separate PID and exec-status pipes avoid a race between parent/grandchild
// writes. No shell is involved, and pipes have bounded reads.
ssize_t ReadLaunch(int fd, void* value, std::size_t bytes) {
 const auto deadline=std::chrono::steady_clock::now()+std::chrono::seconds(10);
 while(std::chrono::steady_clock::now()<deadline){pollfd p={fd,POLLIN|POLLHUP,0};int n=poll(&p,1,100);if(n<0&&errno==EINTR)continue;if(n<0)return -1;if(n>0){ssize_t got=read(fd,value,bytes);if(got<0&&errno==EINTR)continue;return got;}}return -1;
}
bool Start(const std::string& root,const std::string& binary,const std::string& machine,const std::string& server){
 std::vector<std::string> values={binary,"--config-server",server,"--machine-id",machine,"--config-dir",root+"/networks","--rpc-portal","127.0.0.1:15888"};
 std::vector<char*> args;for(auto& s:values)args.push_back(const_cast<char*>(s.c_str()));args.push_back(NULL);
 if(!SafeParents(root+"/networks",true))return false;
 long max_fd=sysconf(_SC_OPEN_MAX);if(max_fd<0||max_fd>65536)max_fd=65536;
 std::unique_lock<std::mutex> lock(ExecForkMutex());int report[2],exec_status[2];
 if(pipe(report)!=0)return false;
 if(pipe(exec_status)!=0){close(report[0]);close(report[1]);return false;}
 for(int fd:{report[0],report[1],exec_status[0],exec_status[1]})fcntl(fd,F_SETFD,FD_CLOEXEC);
 pid_t intermediate=fork();
 if(intermediate==0){
  close(report[0]);close(exec_status[0]);if(setsid()<0)_exit(125);
  pid_t child=fork();if(child<0)_exit(125);
  if(child>0){write(report[1],&child,sizeof(child));_exit(0);}
  close(report[1]);int nullfd=open("/dev/null",O_RDWR);
  if(nullfd>=0 && dup2(nullfd,0)>=0 && dup2(nullfd,1)>=0 && dup2(nullfd,2)>=0){
   for(int fd=3;fd<max_fd;++fd)if(fd!=exec_status[1])close(fd);
   execve(binary.c_str(),args.data(),environ);
  }
  int failed=errno;write(exec_status[1],&failed,sizeof(failed));_exit(127);
 }
 lock.unlock();close(report[1]);close(exec_status[1]);
 if(intermediate<0){close(report[0]);close(exec_status[0]);return false;}
 pid_t child=0;ssize_t got=ReadLaunch(report[0],&child,sizeof(child));close(report[0]);
 int status=0;while(waitpid(intermediate,&status,0)<0&&errno==EINTR){}
 int failed=0;ssize_t extra=ReadLaunch(exec_status[0],&failed,sizeof(failed));close(exec_status[0]);
 if(!WIFEXITED(status)||WEXITSTATUS(status)!=0||got!=sizeof(child)||child<=1)return false;
 // Track an ambiguous launch too, so a later inspect cannot start a duplicate.
 if(!SavePID(root,child,machine)){kill(child,SIGTERM);return false;}
 return extra==0;

}
}
bool ParseNetworkAgent(const std::string& input,std::map<std::string,std::string>* p,std::string* error){
 JsonObject o;if(!ParseJsonObject(input,&o,error))return false;p->clear();for(auto& entry:o){if(entry.second.type!=JsonType::kString||(entry.first!="action"&&entry.first!="directory"&&entry.first!="machine_id"&&entry.first!="config_server"))return false;(*p)[entry.first]=entry.second.string_value;}
 const auto action=(*p)["action"];if((action!="prepare"&&action!="inspect"&&action!="install"&&action!="start")||!Directory((*p)["directory"])||!UUID((*p)["machine_id"]))return false;
 const auto server=(*p)["config_server"];if(action!="start")return server.empty();if(server.size()>512||server.find_first_of(" \r\n\t@?#")!=std::string::npos||server.find('\0')!=std::string::npos)return false;auto sep=server.find("://");if(sep==std::string::npos)return false;auto proto=server.substr(0,sep);auto slash=server.find('/',sep+3);return (proto=="tcp"||proto=="udp"||proto=="ws"||proto=="wss")&&slash!=std::string::npos&&slash+1<server.size()&&server.find(':',sep+3)<slash;
}
ExecResult ExecuteNetworkAgent(const ExecTask& task,const std::atomic<bool>* stop){
 std::lock_guard<std::mutex> guard(network_mutex);ExecResult result;result.task_id=task.task_id;result.started_at=static_cast<std::uint64_t>(std::time(NULL));result.status="failed";result.exit_code=1;
 auto finish=[&](const std::string& error){result.stderr_text=error;result.finished_at=static_cast<std::uint64_t>(std::time(NULL));return result;};if(stop&&stop->load())return finish("cancelled");
 const auto& p=task.config;auto root=p.at("directory"),machine=p.at("machine_id"),action=p.at("action");struct stat st;bool exists=lstat(root.c_str(),&st)==0;if((exists&&!SafeParents(root,false))||(!exists&&action!="inspect"&&!SafeParents(root,true)))return finish("unsafe_install_directory");
 std::string owner;bool running=exists&&Running(root,machine,&owner);if(!owner.empty()&&owner!=machine)return finish("another_managed_instance");auto core=ExistingCore(root);
 if(action=="install"&&core.empty()){if(!Install(root,task,stop))return finish("package_requires_extracted_elf_core");core=ExistingCore(root);}
 std::string version;if(!core.empty()){auto check=Program(task,{core,"--version"},stop);if(check.status!="success")return finish("engine_executable_failed");version=check.stdout_text;while(!version.empty()&&(version.back()=='\r'||version.back()=='\n'))version.pop_back();if(!VersionOK(version))return finish("engine_version_mismatch");}
 bool tun=access("/dev/net/tun",R_OK|W_OK)==0;
 if(action=="start"&&!running){if(core.empty())return finish("engine_not_installed");if(!tun)return finish("tun_unavailable");if(!Start(root,core,machine,p.at("config_server")))return finish("engine_start_failed");running=true;}
 result.status="success";result.exit_code=0;result.stdout_text="{\"installed\":"+std::string(core.empty()?"false":"true")+",\"running\":"+(running?"true":"false")+",\"tun\":"+(tun?"true":"false")+",\"version\":"+EscapeJsonString(version)+",\"machine_id\":"+EscapeJsonString(machine)+"}";return finish("");
}
}
