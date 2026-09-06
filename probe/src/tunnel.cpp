#include "rmp/tunnel.h"
#include "rmp/json.h"
#include "rmp/task.h"

#include <arpa/inet.h>
#include <cerrno>
#include <chrono>
#include <cstring>
#include <fcntl.h>
#include <mutex>
#include <system_error>
#include <poll.h>
#include <sys/socket.h>
#include <unistd.h>

namespace rmp {
namespace {
typedef std::chrono::steady_clock Clock;
struct Socket {
    int fd;
    Socket():fd(-1){}
    ~Socket(){if(fd>=0)close(fd);}
    Socket(const Socket&)=delete;
    Socket& operator=(const Socket&)=delete;
};
bool Hex(const std::string& s,std::size_t n){if(s.size()!=n)return false;for(char c:s)if(!((c>='0'&&c<='9')||(c>='a'&&c<='f')))return false;return true;}
bool String(const JsonObject& o,const char* key,std::string* v){auto i=o.find(key);if(i==o.end()||i->second.type!=JsonType::kString)return false;*v=i->second.string_value;return true;}
bool Number(const JsonObject& o,const char* key,std::uint64_t max,std::uint64_t* v){auto i=o.find(key);if(i==o.end()||i->second.type!=JsonType::kUnsignedInteger||i->second.unsigned_value==0||i->second.unsigned_value>max)return false;*v=i->second.unsigned_value;return true;}
std::string Status(const std::string& mid,const std::string& cid,const std::string& state){return "{\"maintenance_id\":"+EscapeJsonString(mid)+",\"connection_id\":"+EscapeJsonString(cid)+",\"state\":"+EscapeJsonString(state)+"}";}
bool Wait(int fd,short events,const std::atomic<bool>& stop,Clock::time_point until){
    while(!stop.load()&&Clock::now()<until){pollfd p={fd,events,0};int n=poll(&p,1,20);if(n>0)return true;if(n<0&&errno!=EINTR)return false;}return false;
}
bool Dial(const std::string& host,int port,Socket* s,const std::atomic<bool>& stop,Clock::time_point until){
    sockaddr_storage storage;std::memset(&storage,0,sizeof(storage));socklen_t length;
    sockaddr_in* v4=reinterpret_cast<sockaddr_in*>(&storage);sockaddr_in6* v6=reinterpret_cast<sockaddr_in6*>(&storage);
    if(inet_pton(AF_INET,host.c_str(),&v4->sin_addr)==1){v4->sin_family=AF_INET;v4->sin_port=htons(port);length=sizeof(*v4);}
    else if(inet_pton(AF_INET6,host.c_str(),&v6->sin6_addr)==1){v6->sin6_family=AF_INET6;v6->sin6_port=htons(port);length=sizeof(*v6);}else return false;
    {std::lock_guard<std::mutex> guard(ExecForkMutex());s->fd=socket(storage.ss_family,SOCK_STREAM,0);
    if(s->fd<0)return false;
    if(fcntl(s->fd,F_SETFD,FD_CLOEXEC)<0||fcntl(s->fd,F_SETFL,O_NONBLOCK)<0)return false;}
    int size=32768;setsockopt(s->fd,SOL_SOCKET,SO_SNDBUF,&size,sizeof(size));setsockopt(s->fd,SOL_SOCKET,SO_RCVBUF,&size,sizeof(size));
    if(stop.load()||Clock::now()>=until)return false;
    if(connect(s->fd,reinterpret_cast<sockaddr*>(&storage),length)==0)return true;
    if(errno!=EINPROGRESS)return false;
    if(!Wait(s->fd,POLLOUT,stop,until))return false;
    int error=0;socklen_t len=sizeof(error);return getsockopt(s->fd,SOL_SOCKET,SO_ERROR,&error,&len)==0&&error==0&&!stop.load();
}
bool Handshake(int fd,const std::string& h,const std::atomic<bool>& stop,Clock::time_point until){
    std::size_t offset=0;
    while(offset<h.size()&&!stop.load()&&Clock::now()<until){
        ssize_t n=send(fd,h.data()+offset,h.size()-offset,MSG_NOSIGNAL);
        if(n>0){offset+=n;continue;}
        if(n<0&&errno==EINTR)continue;
        if(n<0&&(errno==EAGAIN||errno==EWOULDBLOCK)&&Wait(fd,POLLOUT,stop,until))continue;
        return false;
    }
    if(offset!=h.size())return false;
    while(!stop.load()&&Clock::now()<until){unsigned char ack=0;ssize_t n=recv(fd,&ack,1,0);if(n==1)return ack==1;if(n==0)return false;if(errno==EINTR)continue;if((errno==EAGAIN||errno==EWOULDBLOCK)&&Wait(fd,POLLIN,stop,until))continue;return false;}return false;
}
struct Pipe {
    char bytes[16384];std::size_t offset=0,size=0;bool eof=false,shut=false;
    bool Step(int src,int dst,bool* progress){
        if(size==0&&!eof){ssize_t n=recv(src,bytes,sizeof(bytes),0);if(n>0){offset=0;size=n;*progress=true;}else if(n==0){eof=true;*progress=true;}else if(errno!=EINTR&&errno!=EAGAIN&&errno!=EWOULDBLOCK)return false;}
        if(size>0){ssize_t n=send(dst,bytes+offset,size,MSG_NOSIGNAL);if(n>0){offset+=n;size-=n;*progress=true;}else if(n==0)return false;else if(errno!=EINTR&&errno!=EAGAIN&&errno!=EWOULDBLOCK)return false;}
        if(eof&&size==0&&!shut){if(shutdown(dst,SHUT_WR)!=0)return false;shut=true;*progress=true;}return true;
    }
};
void Relay(int a,int b,const std::atomic<bool>& stop,std::chrono::milliseconds idle){
    Pipe ab,ba;Clock::time_point last=Clock::now();
    while(!stop.load()&&Clock::now()-last<idle){
        bool progress=false;if(!ab.Step(a,b,&progress)||!ba.Step(b,a,&progress))return;if(ab.shut&&ba.shut)return;
        if(progress){last=Clock::now();continue;}
        short ae=static_cast<short>((!ab.eof&&ab.size==0?POLLIN:0)|(ba.size>0?POLLOUT:0));
        short be=static_cast<short>((!ba.eof&&ba.size==0?POLLIN:0)|(ab.size>0?POLLOUT:0));
        pollfd p[2]={{ae?a:-1,ae,0},{be?b:-1,be,0}};if(poll(p,2,20)<0&&errno!=EINTR)return;
    }
}
}

struct TunnelManager::Worker {
    std::string mid,cid,token,host,result;
    int target,port;
    std::uint64_t timeout,idle;
    std::atomic<bool> stop,done;
    std::thread thread;
    Worker():target(0),port(0),timeout(0),idle(0),stop(false),done(false){}
    void Run(){
        { // Close sockets before publishing completion to Tick or destructor.
            Socket local,data;const Clock::time_point until=Clock::now()+std::chrono::milliseconds(timeout);
            if(!Dial("127.0.0.1",target,&local,stop,until))result="local_unavailable";
            else if(!Dial(host,port,&data,stop,until)||!Handshake(data.fd,"RMT1"+mid+cid+token,stop,until))result="data_failed";
            else Relay(local.fd,data.fd,stop,std::chrono::milliseconds(idle));
        }
        done.store(true);
    }
};
TunnelManager::TunnelManager(const std::string& session,std::size_t limit,Report report):session_(session),limit_(limit),report_(report){}
TunnelManager::~TunnelManager(){for(auto& w:workers_)w->stop.store(true);for(auto& w:workers_)if(w->thread.joinable())w->thread.join();}
bool TunnelManager::Feed(const Frame& frame,std::string* error){
    JsonObject object;std::string mid,cid;
    if(frame.header.flags!=0||frame.payload.size()>4096||!ParseJsonObject(std::string(frame.payload.begin(),frame.payload.end()),&object,error)||!String(object,"maintenance_id",&mid)||!Hex(mid,32)||!String(object,"connection_id",&cid)){*error="invalid tunnel control";return false;}
    if(frame.header.type==kTypeTunnelClose){if(!cid.empty()&&!Hex(cid,32))return false;for(auto& w:workers_)if(w->mid==mid&&(cid.empty()||w->cid==cid))w->stop.store(true);return true;}
    std::string session,service,token,host;std::uint64_t port,timeout,idle;
    if(frame.header.type!=kTypeTunnelConnect||!Hex(cid,32)||!String(object,"session_id",&session)||session!=session_||!String(object,"service",&service)||!String(object,"token",&token)||!Hex(token,64)||!String(object,"data_host",&host)||!Number(object,"data_port",65535,&port)||!Number(object,"timeout_ms",60000,&timeout)||!Number(object,"idle_ms",86400000,&idle)){*error="invalid tunnel connect";return false;}
    int target=service=="web"?80:service=="ssh"?22:service=="telnet"?23:0;in6_addr address;
    if(target==0||(inet_pton(AF_INET,host.c_str(),&address)!=1&&inet_pton(AF_INET6,host.c_str(),&address)!=1)){*error="invalid tunnel target";return false;}
    for(auto& w:workers_)if(w->cid==cid)return true; // no second worker for an in-flight identity
    if(workers_.size()>=limit_)return report_(Status(mid,cid,"busy"));
    std::shared_ptr<Worker> w(new Worker);w->mid=mid;w->cid=cid;w->token=token;w->host=host;w->port=port;w->target=target;w->timeout=timeout;w->idle=idle;
    workers_.push_back(w);
    try{w->thread=std::thread([w]{w->Run();});}catch(const std::system_error&){workers_.pop_back();return report_(Status(mid,cid,"busy"));}
    return true;
}
bool TunnelManager::Tick(){
    for(auto i=workers_.begin();i!=workers_.end();){auto w=*i;if(!w->done.load()){++i;continue;}w->thread.join();i=workers_.erase(i);if(!w->stop.load()&&!w->result.empty()&&!report_(Status(w->mid,w->cid,w->result)))return false;}return true;
}
}
