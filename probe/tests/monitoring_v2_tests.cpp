#include "template_fixture.h"
#include "rmp/json.h"
#include <fstream>
#include <stdexcept>
#include <iostream>
#include <cstdlib>
#include <sys/stat.h>
#include <unistd.h>
static void Check(bool b,const char* m){if(!b)throw std::runtime_error(m);}
static void Put(const std::string& p,const std::string& s){std::ofstream f(p.c_str());f<<s;}
int main(){
 char dir[]="/tmp/rmp-v2-XXXXXX";Check(mkdtemp(dir)!=NULL,"temp");const std::string root=dir;
 const std::string path=getenv("PATH")?getenv("PATH"):"";setenv("PATH",root.c_str(),1);
 Put(root+"/nvram","#!/bin/sh\nprintf 'FNR100 v1.1 (Jan  7 2026 11:51:01) std\\n'\n");chmod((root+"/nvram").c_str(),0700);
 rmp::ClientConfig cfg;rmp::CollectFirmware(&cfg);Check(cfg.properties["model"]=="FNR100"&&cfg.properties["firmware"]=="FNR100 v1.1 (Jan  7 2026 11:51:01) std","firmware full value and prefix");
 rmp::CollectionTemplate tpl;tpl.id="t";tpl.name="T";tpl.version=1;tpl.properties["model"]=rmp::CollectionProperty("型号","exit 1");auto templateMetrics=CollectCurrent(tpl,cfg);
 Check(templateMetrics["model"].status=="error"&&cfg.properties["firmware"].find("FNR100")==0&&templateMetrics["model"].reason=="command_failed","independent template priority and failure");
 Put(root+"/nvram","#!/bin/sh\nprintf 'release-string'\n");rmp::ClientConfig bad;rmp::CollectFirmware(&bad);Check(bad.properties["firmware"]=="release-string"&&bad.builtin_errors["model"]=="model_prefix_missing","missing delimiter");
 Put(root+"/nvram","#!/bin/sh\nexit 1\n");rmp::ClientConfig failed;rmp::CollectFirmware(&failed);Check(failed.properties.empty()&&failed.builtin_errors.size()==2,"command failure");
 std::vector<std::string> names;Check(rmp::ParseNetworkInterfaces("eth0,br0",&names)&&names.size()==2,"whitelist");for(const char* s:{"eth0,eth0","eth*","../x","eth0,","abcdefghijklmnop"})Check(!rmp::ParseNetworkInterfaces(s,&names),"invalid whitelist");
 mkdir((root+"/proc").c_str(),0700);mkdir((root+"/proc/net").c_str(),0700);
 Put(root+"/proc/net/dev","eth0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\nbr0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\n");
 rmp::SystemSampler sampler(root,{"eth0"});auto first=sampler.Sample("network");Check(first.size()==7&&first["net_65746830_rx_bytes"].value=="0","filter before report and baseline");
 usleep(1100000);Put(root+"/proc/net/dev","eth0: 400 0 0 0 0 0 0 0 700 0 0 0 0 0 0 0\n");auto next=sampler.Sample("network");Check(next["net_65746830_rx_bytes"].value=="300"&&next["net_65746830_tx_bytes"].value=="500"&&next["net_65746830_elapsed_seconds"].value=="1","counter difference and monotonic duration");
 Put(root+"/proc/net/dev","eth0: 1 0 0 0 0 0 0 0 1 0 0 0 0 0 0 0\n");next=sampler.Sample("network");Check(next["net_65746830_rx_bytes"].value=="0"&&next["net_65746830_rx_bytes_per_sec"].status=="waiting","rollback resets");
 Put(root+"/proc/net/dev","");next=sampler.Sample("network");Check(next["net_65746830_rx_bytes_per_sec"].reason=="interface_missing","missing whitelist interface reason");
 rmp::ClientConfig c;for(auto& p:c.monitoring)p.second=0;
 bool disabled=false;{
  rmp::TelemetryCollector collector(c);std::string wire;
  for(int n=0;n<100&&!disabled;n++){if(collector.Next(65536,&wire)&&wire.find("collection_disabled")!=std::string::npos&&wire.find("egress_ipv4")!=std::string::npos&&wire.find("egress_ipv6")!=std::string::npos)disabled=true;usleep(10000);}
 }Check(disabled,"disabled native egress reports both address families");
 unlink((root+"/nvram").c_str());unlink((root+"/proc/net/dev").c_str());rmdir((root+"/proc/net").c_str());rmdir((root+"/proc").c_str());rmdir(root.c_str());setenv("PATH",path.c_str(),1);
 std::cout<<"PASS monitoring v2 firmware, filters, counters, egress"<<std::endl;
}
