#include "rmp/device_logs.h"
#include "rmp/json.h"
#include <algorithm>
#include <cerrno>
#include <climits>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <ctime>
#include <dirent.h>
#include <fcntl.h>
#include <set>
#include <sstream>
#include <stdexcept>
#include <sys/stat.h>
#include <sys/statvfs.h>
#include <unistd.h>
namespace rmp {
namespace {
std::string Q(const std::string& s){return EscapeJsonString(s);}
void Need(bool ok,const char* why){if(!ok)throw std::runtime_error(why);}
std::string Num(std::uint64_t n){std::ostringstream s;s<<n;return s.str();}
bool Path(const std::string& s){return !s.empty()&&s[0]=='/'&&s.size()<=1024&&s.find('\0')==std::string::npos;}
bool Name(const std::string& s){
 if(s.size()!=24&&s.size()!=27)return false;
 if(s.compare(0,10,"FF_BKDATA_")!=0)return false;
 for(unsigned i=10;i<20;++i){if(i==14||i==17){if(s[i]!='-')return false;}else if(s[i]<'0'||s[i]>'9')return false;}
 return s.substr(20)==".txt"||s.substr(20)==".txt.gz";
}
// Names contain 10-character FF_BKDATA_ prefix and ISO date (10), followed by .txt[.gz].
struct FD{int n;explicit FD(int f):n(f){}~FD(){if(n>=0)close(n);}private:FD(const FD&);FD& operator=(const FD&);};
std::string Identity(const struct stat& st){return Num(st.st_dev)+":"+Num(st.st_ino);}
std::string Hex(const char* p,std::size_t n){static const char* h="0123456789abcdef";std::string s;s.reserve(n*2);for(std::size_t i=0;i<n;++i){unsigned char c=p[i];s+=h[c>>4];s+=h[c&15];}return s;}
std::string Trim(std::string s){while(!s.empty()&&(s.back()=='\n'||s.back()=='\r'))s.pop_back();return s;}
ExecResult Nvram(const ExecTask& original,const std::string& op,const std::string& key,const std::string& value,const std::atomic<bool>* stop){
 ExecTask t;t.task_id=original.task_id;t.type="router_config";t.timeout=op=="commit"?5:2;t.config["backend"]="nvram";t.config["operation"]=op;
 if(op!="commit")t.config["key"]=key;if(op=="set")t.config["value"]=value;return ExecuteExec(t,stop);
}
std::string Get(const std::string& key,const std::atomic<bool>* stop){ExecTask t;auto r=Nvram(t,"get",key,"",stop);Need(r.status=="success"&&r.exit_code==0&&!r.truncated,"nvram_read_failed");return Trim(r.stdout_text);}
std::string Status(const std::atomic<bool>* stop){
 std::string debug=Get("debuglog_enable",stop),syslog=Get("syslogd_enable",stop),history=Get("log_save_en",stop),interval=Get("log_save_itv",stop);
 return "{\"debuglog_enable\":"+Q(debug)+",\"syslogd_enable\":"+Q(syslog)+",\"log_save_en\":"+Q(history)+",\"log_save_itv\":"+Q(interval)+"}";
}
struct Snapshot {std::string directory,path;std::chrono::steady_clock::time_point created;std::uint64_t size;};
class Snapshots {
 std::mutex mutex;std::map<std::string,Snapshot> entries;
 std::condition_variable changed;bool stopping;std::thread cleaner;
 void Run(){std::unique_lock<std::mutex> lock(mutex);while(!stopping){changed.wait_for(lock,std::chrono::seconds(30));if(stopping)break;for(auto i=entries.begin();i!=entries.end();){auto old=i++;if(std::chrono::steady_clock::now()-old->second.created>std::chrono::seconds(3600))Remove(old);}}}
 void Remove(std::map<std::string,Snapshot>::iterator i){unlink(i->second.path.c_str());rmdir(i->second.directory.c_str());entries.erase(i);}
public:
 Snapshots():stopping(false),cleaner(&Snapshots::Run,this){}
 ~Snapshots(){{std::lock_guard<std::mutex> lock(mutex);stopping=true;}changed.notify_all();cleaner.join();for(auto i=entries.begin();i!=entries.end();){auto old=i++;Remove(old);}}
 void Release(const std::string& path){std::lock_guard<std::mutex> lock(mutex);auto i=entries.find(path);if(i!=entries.end())Remove(i);}
 std::string Copy(const std::string& source,const std::atomic<bool>* stop){
  std::lock_guard<std::mutex> lock(mutex);
  for(auto i=entries.begin();i!=entries.end();){auto old=i++;if(std::chrono::steady_clock::now()-old->second.created>std::chrono::seconds(3600))Remove(old);}
  Need(entries.size()<8,"snapshot_capacity");std::uint64_t total=0;for(auto i=entries.begin();i!=entries.end();++i)total+=i->second.size;
  FD in(open(source.c_str(),O_RDONLY|O_NONBLOCK|O_CLOEXEC));Need(in.n>=0,"source_unavailable");struct stat before,after;
  Need(fstat(in.n,&before)==0&&S_ISREG(before.st_mode),"source_not_regular");Need(before.st_size>=0&&before.st_size<=32*1024*1024&&total+before.st_size<=64*1024*1024,"snapshot_size_limit");
  struct statvfs fs;Need(statvfs("/tmp",&fs)==0&&static_cast<std::uint64_t>(fs.f_bavail)*fs.f_frsize>static_cast<std::uint64_t>(before.st_size)+1024*1024,"snapshot_no_space");
  char directory[]="/tmp/router-agent-log-XXXXXX";Need(mkdtemp(directory)!=NULL,"snapshot_create_failed");
  const std::string name=source.substr(source.find_last_of('/')+1),outpath=std::string(directory)+"/"+name;
  try{
   FD out(open(outpath.c_str(),O_WRONLY|O_CREAT|O_EXCL|O_CLOEXEC,0600));Need(out.n>=0,"snapshot_create_failed");
   char bytes[8192];off_t remaining=before.st_size;const auto started=std::chrono::steady_clock::now();
   while(remaining){Need(!stop||!stop->load(),"cancelled");Need(std::chrono::steady_clock::now()-started<std::chrono::seconds(25),"snapshot_timeout");ssize_t n=read(in.n,bytes,std::min<off_t>(sizeof(bytes),remaining));if(n<0&&errno==EINTR)continue;Need(n>0,"source_changed");ssize_t at=0;while(at<n){ssize_t wrote=write(out.n,bytes+at,n-at);if(wrote<0&&errno==EINTR)continue;Need(wrote>0,"snapshot_write_failed");at+=wrote;}remaining-=n;}
   Need(fstat(in.n,&after)==0&&before.st_size==after.st_size&&before.st_mtime==after.st_mtime&&before.st_ctime==after.st_ctime,"source_changed_retry_snapshot");
   entries[outpath]=Snapshot{directory,outpath,std::chrono::steady_clock::now(),static_cast<std::uint64_t>(before.st_size)};
  }catch(...){unlink(outpath.c_str());rmdir(directory);throw;}
  return "{\"remote_path\":"+Q(outpath)+",\"name\":"+Q(name)+",\"size\":"+Num(before.st_size)+",\"expires_in_seconds\":3600}";
 }
};
Snapshots& Store(){static Snapshots s;return s;}
}
bool ParseDeviceLogTask(const std::string& raw,RouterConfigParams* p,std::string* error){
 JsonObject o;if(!ParseJsonObject(raw,&o,error))return false;p->clear();for(auto i=o.begin();i!=o.end();++i){if(i->second.type!=JsonType::kString||i->second.string_value.find('\0')!=std::string::npos)return false;(*p)[i->first]=i->second.string_value;}
 if(!p->count("action"))return false;const std::string a=p->at("action");
 if(a=="enable_live")return p->size()==1;
 if(a=="snapshot"||a=="release")return p->size()==2&&p->count("path")&&Path(p->at("path"))&&(a=="release"||Name(p->at("path").substr(p->at("path").find_last_of('/')+1)));
 if(a!="history_settings"||p->size()!=4||!p->count("enabled")||!p->count("interval")||!p->count("persist"))return false;
 if((p->at("enabled")!="0"&&p->at("enabled")!="1")||(p->at("persist")!="0"&&p->at("persist")!="1"))return false;
 const std::string n=p->at("interval");if(n.empty()||n.size()>5)return false;for(char c:n)if(c<'0'||c>'9')return false;return std::strtoul(n.c_str(),NULL,10)>=1&&std::strtoul(n.c_str(),NULL,10)<=65535;
}
ExecResult ExecuteDeviceLogTask(const ExecTask& t,const std::atomic<bool>* stop){
 ExecResult r;r.task_id=t.task_id;r.started_at=std::time(NULL);r.exit_code=1;r.status="failed";
 try{
  const std::string a=t.config.at("action");
  auto set=[&](const std::string& key,const std::string& value){auto v=Nvram(t,"set",key,value,stop);Need(v.status=="success"&&v.exit_code==0,"nvram_set_failed_partial_configuration_possible");Need(Get(key,stop)==value,"nvram_readback_mismatch");};
  if(a=="snapshot")r.stdout_text=Store().Copy(t.config.at("path"),stop);
  else if(a=="release"){Store().Release(t.config.at("path"));r.stdout_text="{}";}
  else{
   if(a=="enable_live"){set("debuglog_enable","1");set("syslogd_enable","3");}
   else{if(t.config.at("enabled")=="1")set("debuglog_enable","1");set("log_save_itv",t.config.at("interval"));set("log_save_en",t.config.at("enabled"));}
   if(a=="enable_live"||t.config.at("persist")=="1"){auto v=Nvram(t,"commit","","",stop);Need(v.status=="success"&&v.exit_code==0,"nvram_commit_failed_runtime_values_may_have_changed");}
   r.stdout_text=Status(stop);
  }
  r.exit_code=0;r.status="success";
 }catch(const std::exception& e){r.stderr_text=e.what();}r.finished_at=std::time(NULL);return r;
}
std::string ReadDeviceLog(const std::string& path,const std::string& generation,std::uint64_t offset){
 FD fd(open(path.c_str(),O_RDONLY|O_NONBLOCK|O_CLOEXEC));if(fd.n<0){if(errno==ENOENT)return "{\"state\":\"waiting_for_file\",\"data_hex\":\"\",\"generation\":\"\",\"offset\":0,\"gap\":true}";throw std::runtime_error("log_read_denied");}
 struct stat st;Need(fstat(fd.n,&st)==0&&S_ISREG(st.st_mode)&&st.st_size>=0,"log_not_regular");std::string id=Identity(st);bool gap=!generation.empty()&&(id!=generation||offset>static_cast<std::uint64_t>(st.st_size));
 std::uint64_t at=offset;if(generation.empty()||gap)at=st.st_size>8192?st.st_size-8192:0;
 if(at>static_cast<std::uint64_t>(st.st_size))at=0;
 if(static_cast<std::uint64_t>(st.st_size)-at>65536){at=st.st_size>8192?st.st_size-8192:0;gap=true;}
 Need(at<=static_cast<std::uint64_t>(LONG_MAX),"log_offset_out_of_range");char bytes[8192];ssize_t n;
 do{n=pread(fd.n,bytes,sizeof(bytes),static_cast<off_t>(at));}while(n<0&&errno==EINTR);Need(n>=0,"log_read_failed");
 return "{\"state\":\"ok\",\"generation\":"+Q(id)+",\"start\":"+Num(at)+",\"offset\":"+Num(at+n)+",\"gap\":"+(gap?"true":"false")+",\"data_hex\":"+Q(Hex(bytes,n))+"}";
}
std::string ListDeviceLogs(const std::string& custom){
 std::vector<std::string> roots;if(custom.empty()){roots.push_back("/tmp/third_party/data");roots.push_back("/jffs");roots.push_back("/tmp/root");}else{Need(Path(custom),"invalid_directory");roots.push_back(custom);}
 std::set<std::string> directories,files;std::string entries="[",checked="[";unsigned count=0;bool limited=false;
 for(const auto& root:roots){struct stat ds;std::string state="ok";if(stat(root.c_str(),&ds)!=0||!S_ISDIR(ds.st_mode))state=errno==ENOENT?"missing":"unavailable";
  if(state=="ok"&&!directories.insert(Identity(ds)).second)state="alias";
  DIR* dir=NULL;if(state=="ok"){dir=opendir(root.c_str());if(!dir)state="unreadable";}
  if(checked.size()>1)checked+=",";checked+="{\"path\":"+Q(root)+",\"state\":"+Q(state)+"}";
  if(!dir)continue;unsigned scanned=0;while(struct dirent* ent=readdir(dir)){
   if(++scanned>2048){limited=true;break;}const std::string name=ent->d_name;if(!Name(name))continue;const std::string path=root+"/"+name;struct stat st;if(stat(path.c_str(),&st)!=0||!S_ISREG(st.st_mode)||st.st_size<0)continue;
   if(!files.insert(Identity(st)).second)continue;if(count>=128||entries.size()>44000){limited=true;break;}++count;if(entries.size()>1)entries+=",";
   entries+="{\"name\":"+Q(name)+",\"path\":"+Q(path)+",\"size\":"+Num(st.st_size)+",\"modified\":"+Num(st.st_mtime<0?0:st.st_mtime)+",\"cached\":"+(root=="/tmp/root"?"true":"false")+"}";
  }closedir(dir);
 }
 return "{\"files\":"+entries+"],\"directories\":"+checked+"],\"limited\":"+(limited?"true":"false")+"}";
}
DeviceLogReader::DeviceLogReader():stop_(false),worker_(&DeviceLogReader::Run,this){}
DeviceLogReader::~DeviceLogReader(){stop_=true;changed_.notify_all();worker_.join();}
bool DeviceLogReader::Submit(const std::string& raw,std::string* error){
 JsonObject o;if(raw.size()>4096||!ParseJsonObject(raw,&o,error))return false;
 if(o.size()!=6||o["event"].string_value!="device_log_query"||o["request_id"].type!=JsonType::kString||o["request_id"].string_value.size()!=32||o["operation"].type!=JsonType::kString||o["directory"].type!=JsonType::kString||o["generation"].type!=JsonType::kString||o["offset"].type!=JsonType::kUnsignedInteger)return false;
 for(char c:o["request_id"].string_value)if(!((c>='0'&&c<='9')||(c>='a'&&c<='f')))return false;
 const auto op=o["operation"].string_value;if(op!="live"&&op!="status"&&op!="history")return false;
 if(o["generation"].string_value.size()>64||(!o["directory"].string_value.empty()&&!Path(o["directory"].string_value)))return false;
 std::lock_guard<std::mutex> lock(mutex_);if(requests_.size()+replies_.size()>=4){*error="device_log_queue_full";return false;}requests_.push_back(raw);changed_.notify_one();return true;
}
bool DeviceLogReader::Next(std::string* out){std::lock_guard<std::mutex> lock(mutex_);if(replies_.empty())return false;*out=replies_.front();replies_.pop_front();return true;}
void DeviceLogReader::Run(){while(!stop_){std::string raw;{std::unique_lock<std::mutex> lock(mutex_);changed_.wait(lock,[this]{return stop_.load()||!requests_.empty();});if(stop_)return;raw=requests_.front();requests_.pop_front();}
 JsonObject o;std::string error;ParseJsonObject(raw,&o,&error);std::string data="null";
 try{const auto op=o["operation"].string_value;if(op=="live")data=ReadDeviceLog("/tmp/.systemlog",o["generation"].string_value,o["offset"].unsigned_value);else if(op=="history")data=ListDeviceLogs(o["directory"].string_value);else data=Status(&stop_);}catch(const std::exception& e){error=e.what();}
 const std::string reply="{\"event\":\"device_log_reply\",\"request_id\":"+Q(o["request_id"].string_value)+",\"data\":"+data+",\"error\":"+Q(error)+"}";
 {std::lock_guard<std::mutex> lock(mutex_);replies_.push_back(reply);}
}}
}
