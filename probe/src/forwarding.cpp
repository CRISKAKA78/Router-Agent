#include "rmp/forwarding.h"
#include "rmp/json.h"
#include "rmp/task.h"
#include <cerrno>
#include <cstdlib>
#include <fcntl.h>
#include <poll.h>
#include <pthread.h>
#include <signal.h>
#include <sys/prctl.h>
#include <sys/wait.h>
#include <unistd.h>
#include <chrono>
namespace rmp {
std::string ForwardingManager::Executable(){const char* configured=getenv("RMP_FORWARDING_AGENT");if(configured&&configured[0]=='/')return configured;char path[4096];ssize_t n=readlink("/proc/self/exe",path,sizeof(path)-1);if(n<=0)return "";path[n]=0;std::string p(path);return p.substr(0,p.find_last_of('/')+1)+"router-forwarding-agent";}
bool ForwardingManager::Available(){std::string p=Executable();return !p.empty()&&access(p.c_str(),X_OK)==0;}
ForwardingManager::ForwardingManager(const std::string& session,Report report):session_(session),report_(report),stop_(false){if(Available())worker_=std::thread(&ForwardingManager::Run,this);}
ForwardingManager::~ForwardingManager(){stop_=true;if(worker_.joinable())worker_.join();}
bool ForwardingManager::Feed(const Frame& frame,std::string* error){
 if(frame.header.flags!=0||frame.payload.size()>65536){*error="invalid forwarding payload";return false;}
 std::string data(frame.payload.begin(),frame.payload.end());JsonObject object;if(!ParseJsonObject(data,&object,error))return false;
 if(object["session_id"].string_value!=session_){*error="forwarding session mismatch";return false;}
 // JSON framing is a single line; newlines inside strings are already escaped.
 if(data.find('\n')!=std::string::npos||data.find('\r')!=std::string::npos){*error="forwarding requires compact JSON";return false;}
 std::lock_guard<std::mutex> lock(mu_);if(stop_||!worker_.joinable()||queue_.size()>=16){return report_("{\"id\":"+EscapeJsonString(object["id"].string_value)+",\"session_id\":"+EscapeJsonString(session_)+",\"state\":\"failed\",\"reason\":\"helper_unavailable\"}");}queue_.push_back(data+"\n");return true;
}
void ForwardingManager::Run(){
 sigset_t blocked;sigemptyset(&blocked);sigaddset(&blocked,SIGPIPE);pthread_sigmask(SIG_BLOCK,&blocked,NULL);
 int input[2]={-1,-1},output[2]={-1,-1};pid_t pid=-1;std::string executable=Executable();pid_t parent=getpid();
 {std::lock_guard<std::mutex> forkLock(ExecForkMutex());
 if(pipe(input)!=0){stop_=true;return;}if(pipe(output)!=0){close(input[0]);close(input[1]);stop_=true;return;}
 for(int n=0;n<2;n++){fcntl(input[n],F_SETFD,FD_CLOEXEC);fcntl(output[n],F_SETFD,FD_CLOEXEC);}
 pid=fork();if(pid==0){prctl(PR_SET_PDEATHSIG,SIGTERM);if(getppid()!=parent)_exit(126);dup2(input[0],0);dup2(output[1],1);int nullfd=open("/dev/null",O_WRONLY);if(nullfd>=0){dup2(nullfd,2);if(nullfd>2)close(nullfd);}close(input[0]);close(input[1]);close(output[0]);close(output[1]);execl(executable.c_str(),executable.c_str(),"--session",session_.c_str(),static_cast<char*>(NULL));_exit(127);}}
 close(input[0]);close(output[1]);if(pid<0){close(input[1]);close(output[0]);stop_=true;return;}
 fcntl(input[1],F_SETFL,O_NONBLOCK);fcntl(output[0],F_SETFL,O_NONBLOCK);std::string pending,received;std::size_t offset=0;
 while(!stop_){
  if(pending.empty()){std::lock_guard<std::mutex> lock(mu_);if(!queue_.empty()){pending.swap(queue_.front());queue_.pop_front();offset=0;}}
  pollfd fds[2]={{output[0],POLLIN,0},{input[1],static_cast<short>(pending.empty()?0:POLLOUT),0}};if(poll(fds,2,50)<0&&errno!=EINTR)break;
  if(!pending.empty()&&(fds[1].revents&POLLOUT)){ssize_t n=write(input[1],pending.data()+offset,pending.size()-offset);if(n>0){offset+=n;if(offset==pending.size())pending.clear();}else if(n<0&&errno!=EAGAIN&&errno!=EINTR)break;}
  if(fds[0].revents&POLLIN){char buf[4096];ssize_t n=read(output[0],buf,sizeof(buf));if(n<=0)break;received.append(buf,n);if(received.size()>65536)break;std::size_t end;while((end=received.find('\n'))!=std::string::npos){std::string row=received.substr(0,end);received.erase(0,end+1);if(!report_(row)){stop_=true;break;}}}
  if(fds[0].revents&(POLLHUP|POLLERR|POLLNVAL))break;
 }
 if(!stop_)report_("{\"id\":\"\",\"session_id\":"+EscapeJsonString(session_)+",\"state\":\"helper_failed\",\"reason\":\"helper_exited\"}");
 close(input[1]);close(output[0]);kill(pid,SIGTERM);bool reaped=false;for(int n=0;n<40;n++){pid_t r=waitpid(pid,NULL,WNOHANG);if(r==pid||(r<0&&errno==ECHILD)){reaped=true;break;}std::this_thread::sleep_for(std::chrono::milliseconds(50));}if(!reaped){kill(pid,SIGKILL);while(waitpid(pid,NULL,0)<0&&errno==EINTR){}}
 stop_=true;
}
}
