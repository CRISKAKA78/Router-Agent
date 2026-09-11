#include "rmp/network_agent.h"
#include "rmp/json.h"
#include <atomic>
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <sys/stat.h>
#include <unistd.h>
static void Check(bool v,const char* message){if(!v)throw std::runtime_error(message);}
int main(){try{
 const std::string machine="12345678-1234-1234-1234-123456789abc";std::string error;std::map<std::string,std::string> p;
 auto json=[&](const std::string& action,const std::string& dir,const std::string& extra){return "{\"action\":"+rmp::EscapeJsonString(action)+",\"directory\":"+rmp::EscapeJsonString(dir)+",\"machine_id\":\""+machine+"\""+extra+"}";};
 for(auto action:{"stop","exec","shell"})Check(!rmp::ParseNetworkAgent(json(action,"/tmp/rmp-network", ""),&p,&error),"unsafe action");
 for(auto dir:{"/tmp/root","/tmp/net/../etc","/tmp/a;b","/tmp/a b","/tmp//net"})Check(!rmp::ParseNetworkAgent(json("inspect",dir,""),&p,&error),"unsafe directory");
 Check(!rmp::ParseNetworkAgent(json("start","/tmp/rmp-network",",\"config_server\":\"tcp://user:pass@host:22020/test\""),&p,&error),"userinfo allowed");
 Check(rmp::ParseNetworkAgent(json("start","/tmp/rmp-network",",\"config_server\":\"tcp://example.com:22020/test\""),&p,&error),"valid start");
 rmp::ExecTask task;Check(rmp::ParseTask("{\"task_id\":\"network-test\",\"type\":\"network_agent\",\"timeout\":30,\"params\":"+json("inspect","/tmp/rmp-network","")+"}",&task,&error),"typed parser");
 Check(!rmp::ParseTask("{\"task_id\":\"network-test\",\"type\":\"network_agent\",\"timeout\":31,\"params\":"+json("inspect","/tmp/rmp-network","")+"}",&task,&error),"timeout bound");
 char tmp[]="/tmp/rmp-network-XXXXXX";Check(mkdtemp(tmp)!=NULL,"mkdtemp");std::string root=std::string(tmp)+"/engine";std::atomic<bool> stop(false);
 task.task_id="inspect";task.type="network_agent";task.timeout=30;task.config={{"action","inspect"},{"directory",root},{"machine_id",machine}};
 auto r=rmp::ExecuteNetworkAgent(task,&stop);Check(r.status=="success","inspect");Check(access(root.c_str(),F_OK)!=0,"inspect created directory");
 task.config["action"]="prepare";r=rmp::ExecuteNetworkAgent(task,&stop);Check(r.status=="success","prepare");struct stat st;Check(stat(root.c_str(),&st)==0&&(st.st_mode&0777)==0700,"private directory mode");
 std::ofstream((root+"/package").c_str())<<"not an ELF package";task.config["action"]="install";r=rmp::ExecuteNetworkAgent(task,&stop);Check(r.status=="failed","bad package accepted");Check(access((root+"/easytier-core").c_str(),F_OK)!=0,"bad package installed");
 unlink((root+"/package").c_str());Check(symlink("/bin/sh",(root+"/package").c_str())==0,"symlink setup");r=rmp::ExecuteNetworkAgent(task,&stop);Check(r.status=="failed","symlink package accepted");unlink((root+"/package").c_str());
 task.config["action"]="inspect";stop=true;r=rmp::ExecuteNetworkAgent(task,&stop);Check(r.status=="failed"&&r.stderr_text=="cancelled","cancelled bootstrap");
 stop=false;chmod(root.c_str(),0777);r=rmp::ExecuteNetworkAgent(task,&stop);Check(r.status=="failed","world writable directory accepted");chmod(root.c_str(),0700);rmdir(root.c_str());rmdir(tmp);
 std::cout<<"PASS network agent validation, inspect, package rejection and private directory\n";return 0;
}catch(const std::exception& e){std::cerr<<e.what()<<"\n";return 1;}}
