#include "rmp/device_logs.h"
#include "rmp/task_manager.h"
#include "rmp/json.h"
#include <chrono>
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <sys/stat.h>
#include <thread>
#include <unistd.h>
static void Check(bool ok,const char* text){if(!ok)throw std::runtime_error(text);}
static std::string Read(const std::string& p){std::ifstream f(p.c_str());return std::string((std::istreambuf_iterator<char>(f)),std::istreambuf_iterator<char>());}
int main(){try{
 std::string error;rmp::RouterConfigParams params;
 Check(rmp::ParseDeviceLogTask("{\"action\":\"enable_live\"}",&params,&error),"enable parse");
 Check(!rmp::ParseDeviceLogTask("{\"action\":\"enable_live\",\"command\":\"reboot\"}",&params,&error),"reject shell fields");
 Check(rmp::ParseDeviceLogTask("{\"action\":\"snapshot\",\"path\":\"/jffs/FF_BKDATA_2025-09-25.txt.gz\"}",&params,&error),"archive filename parse");
 Check(!rmp::ParseDeviceLogTask("{\"action\":\"history_settings\",\"enabled\":\"1\",\"interval\":\"0\",\"persist\":\"1\"}",&params,&error),"interval bound");
 char directory[]="/tmp/rmp-log-test-XXXXXX";Check(mkdtemp(directory)!=NULL,"temp directory");std::string root=directory;
 const std::string log=root+"/FF_BKDATA_2025-09-25.txt";std::ofstream(log.c_str())<<"first\n";
 rmp::JsonObject batch;Check(rmp::ParseJsonObject(rmp::ReadDeviceLog(log,"",0),&batch,&error),"batch json");Check(batch["offset"].unsigned_value==6&&batch["data_hex"].string_value=="66697273740a","initial bytes");
 std::string gen=batch["generation"].string_value;std::ofstream(log.c_str(),std::ios::app)<<"next\n";
 Check(rmp::ParseJsonObject(rmp::ReadDeviceLog(log,gen,6),&batch,&error)&&batch["offset"].unsigned_value==11&&!batch["gap"].bool_value,"append cursor");
 std::ofstream(log.c_str())<<"short";Check(rmp::ParseJsonObject(rmp::ReadDeviceLog(log,gen,11),&batch,&error)&&batch["gap"].bool_value,"truncate gap");
 const std::string listing=rmp::ListDeviceLogs(root);Check(listing.find("FF_BKDATA_2025-09-25.txt")!=std::string::npos,"discover txt cache");
 const std::string fake=root+"/nvram";
 std::ofstream(fake.c_str())<<"#!/bin/sh\ncase \"$1\" in\nset) key=${2%%=*}; value=${2#*=}; printf '%s' \"$value\" > \"$LOG_TEST_DIR/$key\"; printf 'set %s\\n' \"$2\" >> \"$LOG_TEST_DIR/order\";;\nget) [ ! -f \"$LOG_TEST_DIR/$2\" ] || cat \"$LOG_TEST_DIR/$2\";;\ncommit) printf 'commit\\n' >> \"$LOG_TEST_DIR/order\";;\nesac\nexit 0\n";
 Check(chmod(fake.c_str(),0700)==0,"fake executable");Check(setenv("PATH",(root+":/usr/bin:/bin").c_str(),1)==0&&setenv("LOG_TEST_DIR",root.c_str(),1)==0,"isolated nvram path");
 rmp::ExecTask task;task.task_id="log-enable";task.type="device_logs";task.timeout=30;task.config["action"]="enable_live";
 {rmp::TaskManager manager(2,16,256*1024);Check(manager.Submit(task,true,128,4096)=="queued","admit enable");std::string result;const auto end=std::chrono::steady_clock::now()+std::chrono::seconds(10);while(!manager.CachedResult(task.task_id,4096,&result)&&std::chrono::steady_clock::now()<end)std::this_thread::sleep_for(std::chrono::milliseconds(20));
 Check(result.find("\"status\":\"success\"")!=std::string::npos,"enable success");Check(Read(root+"/order")=="set debuglog_enable=1\nset syslogd_enable=3\ncommit\n","enable automatically commits once");Check(manager.Submit(task,true,128,4096)=="success","duplicate result cached");Check(Read(root+"/order")=="set debuglog_enable=1\nset syslogd_enable=3\ncommit\n","duplicate does not recommit");}
 Check(Read(root+"/debuglog_enable")=="1"&&Read(root+"/syslogd_enable")=="3","stopping does not restore persistent settings");
 task.task_id="history";task.config={{"action","history_settings"},{"enabled","1"},{"interval","300"},{"persist","0"}};
 auto history=rmp::ExecuteDeviceLogTask(task,NULL);Check(history.status=="success"&&Read(root+"/log_save_en")=="1"&&Read(root+"/log_save_itv")=="300","history settings readback");
 task.task_id="snapshot";task.config={{"action","snapshot"},{"path",log}};auto snap=rmp::ExecuteDeviceLogTask(task,NULL);Check(snap.status=="success","snapshot creation");rmp::JsonObject info;Check(rmp::ParseJsonObject(snap.stdout_text,&info,&error),"snapshot json");const std::string remote=info["remote_path"].string_value;Check(Read(remote)=="short","snapshot bytes");
 task.config={{"action","release"},{"path",remote}};Check(rmp::ExecuteDeviceLogTask(task,NULL).status=="success"&&access(remote.c_str(),F_OK)!=0,"owned snapshot cleanup");
 for(const auto& file:{"nvram","order","debuglog_enable","syslogd_enable","log_save_en","log_save_itv","FF_BKDATA_2025-09-25.txt"})unlink((root+"/"+file).c_str());rmdir(root.c_str());std::cout<<"device log checks passed\n";return 0;
 }catch(const std::exception& e){std::cerr<<e.what()<<'\n';return 1;}}
