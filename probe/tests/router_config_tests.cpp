#include "rmp/task_manager.h"
#include "rmp/collection.h"
#include "rmp/json.h"
#include <chrono>
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <sys/stat.h>
#include <unistd.h>

static void Check(bool v,const char* message){if(!v)throw std::runtime_error(message);}
template<class F> static void Wait(F f){for(int i=0;i<600;++i){if(f())return;std::this_thread::sleep_for(std::chrono::milliseconds(10));}throw std::runtime_error("config wait timeout");}
static std::string Read(const std::string& path){std::ifstream f(path.c_str());return std::string(std::istreambuf_iterator<char>(f),std::istreambuf_iterator<char>());}
int main(){try {
 std::string error; rmp::RouterConfigParams parsed;
 for(const char* bad:{"{}","{\"backend\":\"nvram\",\"operation\":\"set\",\"key\":\"SN\"}","{\"backend\":\"uci\",\"operation\":\"delete\",\"key\":\"network.lan\"}","{\"backend\":\"nvram\",\"operation\":\"get\",\"key\":\"-x\"}","{\"backend\":\"nvram\",\"operation\":\"commit\",\"value\":\"\"}","{\"backend\":\"uci\",\"operation\":\"get\",\"key\":\"system.@system[].hostname\"}"})
  Check(!rmp::ParseRouterConfig(bad,&parsed,&error),"invalid parameters accepted");
 Check(rmp::ParseRouterConfig("{\"backend\":\"uci\",\"operation\":\"get\",\"key\":\"system.@system[-1].hostname\"}",&parsed,&error),"anonymous UCI section");
 rmp::ExecTask invalid;
 Check(!rmp::ParseTask("{\"task_id\":\"x\",\"type\":\"router_config\",\"timeout\":0,\"params\":{\"backend\":\"nvram\",\"operation\":\"commit\"}}",&invalid,&error),"zero timeout accepted");
 char tmp[]="/tmp/rmp-config-XXXXXX";Check(mkdtemp(tmp)!=NULL,"mkdtemp");const std::string dir=tmp;
 const std::string script=dir+"/nvram";
 std::ofstream(script.c_str()) << "#!/bin/sh\ncase \"$1\" in\nset) printf '%s' \"$2\" > \"$CONFIG_DIR/value\"; printf 'set\\n' >> \"$CONFIG_DIR/order\"; touch \"$CONFIG_DIR/started\"; while [ ! -f \"$CONFIG_DIR/gate\" ]; do sleep 0.02; done;;\nget) case \"$2\" in slow) sleep 10;; missing) exit 4;; *) printf '%s' \"$2\";; esac;;\ncommit) printf 'commit\\n' >> \"$CONFIG_DIR/order\";;\nesac\n";
 Check(chmod(script.c_str(),0700)==0,"chmod");
 rmp::ExecTask write;write.task_id="set";write.type="router_config";write.timeout=5;
 write.env["PATH"]=dir+":/bin:/usr/bin";write.env["CONFIG_DIR"]=dir;
 const std::string special=" a'\";$(touch "+dir+"/injected)\n中文 ";
 write.config={{"backend","nvram"},{"operation","set"},{"key","SN"},{"value",special}};
 {
  rmp::TaskManager manager(4,3,65536);
  Check(manager.Submit(write,true,500,4096)=="queued","submit set");
  Wait([&]{return access((dir+"/started").c_str(),F_OK)==0;});
  Check(manager.Submit(write,true,500,4096)=="running","running duplicate");
  rmp::ExecTask commit=write;commit.task_id="commit";commit.config={{"backend","nvram"},{"operation","commit"}};
  Check(manager.Submit(commit,true,500,4096)=="queued","submit commit");
  rmp::ExecTask other=write;other.task_id="exec";other.type="exec";other.command="printf independent";
  Check(manager.Submit(other,true,500,4096)=="queued","submit independent exec");
  std::string payload;Wait([&]{return manager.CachedResult("exec",4096,&payload);});
  Check(Read(dir+"/order")=="set\n","commit ran before write finished");
  Check(manager.Submit(commit,true,500,4096)=="queued","queued duplicate");
  rmp::ExecTask conflict=write;conflict.config["value"]="different";
  Check(manager.Submit(conflict,true,500,4096)=="conflict","write conflict not rejected");
  rmp::ExecTask extra=write;extra.task_id="full";
  Check(manager.Submit(extra,true,500,4096)=="rejected","capacity not enforced");
  std::ofstream((dir+"/gate").c_str())<<"go";
  Wait([&]{return manager.CachedResult("commit",4096,&payload);});
  Check(Read(dir+"/order")=="set\ncommit\n","configuration ordering");
  Check(Read(dir+"/value")=="SN="+special,"argument not preserved");
  Check(access((dir+"/injected").c_str(),F_OK)!=0,"shell injection");
  std::string original;Check(manager.CachedResult("set",4096,&original),"set result");
  manager.BeginSession();Check(manager.Submit(write,true,500,4096)=="success","completed duplicate");
  Check(manager.CachedResult("set",4096,&payload)&&payload==original,"changed replay result");
  int count=0;while(manager.NextResult(4096,&payload))++count;Check(count==2,"reconnect replay count");
  Check(Read(dir+"/order")=="set\ncommit\n","duplicate side effect");
 }
 write.config["value"]="";rmp::ExecResult result=rmp::ExecuteExec(write,NULL);Check(result.status=="success"&&Read(dir+"/value")=="SN=","empty value");
 write.config={{"backend","nvram"},{"operation","get"},{"key","missing"}};
 result=rmp::ExecuteExec(write,NULL);Check(result.status=="failed"&&result.exit_code==4,"command failure");
 write.config["key"]="slow";write.timeout=1;
 result=rmp::ExecuteExec(write,NULL);Check(result.status=="timeout","timeout");
 write.env["PATH"]=dir+"/no-program";write.config["key"]="SN";
 result=rmp::ExecuteExec(write,NULL);Check(result.status=="failed"&&result.exit_code==127,"missing program");
 const char* oldPath=getenv("PATH");const std::string savedPath=oldPath?oldPath:"";
 setenv("PATH",(dir+":/bin:/usr/bin").c_str(),1);
 rmp::CollectionTemplate collection;
 Check(rmp::ParseCollectionTemplate("{\"template_id\":\"t\",\"name\":\"t\",\"version\":1,\"properties\":{\"serial\":{\"name\":\"SN\",\"source\":\"nvram\",\"key\":\"SN\",\"timeout_seconds\":5}}}",&collection,&error),"source parsing");
 rmp::ClientConfig config;rmp::CollectProperties(collection,&config);Check(config.properties["serial"]=="SN","config source collection");
 setenv("PATH",savedPath.c_str(),1);
 for(const char* name:{"nvram","value","order","started","gate"})unlink((dir+"/"+name).c_str());rmdir(dir.c_str());
 std::cout<<"router configuration tests passed\n";return 0;
}catch(const std::exception& e){std::cerr<<e.what()<<'\n';return 1;}}
