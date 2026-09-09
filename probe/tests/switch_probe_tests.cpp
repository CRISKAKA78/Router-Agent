#include "rmp/switch_probe.h"
#include "rmp/collection.h"
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <cstdlib>
#include <sys/stat.h>
#include <unistd.h>
static void Check(bool ok,const char*why){if(!ok)throw std::runtime_error(why);}
static void Put(const std::string&p,const std::string&s){std::ofstream f(p.c_str());f<<s;}
int main(){
 char tmp[]="/tmp/rmp-switch-XXXXXX";Check(mkdtemp(tmp)!=NULL,"fixture directory");std::string root=tmp;
 const char*dirs[]={"/sys","/sys/class","/sys/class/net","/sys/class/net/lan1","/sys/class/net/eth0","/proc","/proc/device-tree"};for(auto d:dirs)mkdir((root+d).c_str(),0700);
 Put(root+"/sys/class/net/lan1/phys_switch_id","aabb");Put(root+"/sys/class/net/lan1/phys_port_name","p1");Put(root+"/sys/class/net/lan1/carrier","1");Put(root+"/sys/class/net/lan1/flags","0x0");Put(root+"/sys/class/net/lan1/ifindex","3");Put(root+"/sys/class/net/lan1/iflink","2");Put(root+"/sys/class/net/eth0/ifindex","2");
 auto dsa=rmp::CollectSwitchPorts("{\"backend\":\"dsa\",\"ports\":[{\"id\":\"lan1\",\"port\":1,\"system_name\":\"lan1\",\"display_name\":\"LAN1\",\"uplink\":\"eth0\",\"role\":\"external\"}]}",NULL,root);
 Check(dsa["switch_lan1_state"].value=="up"&&dsa["switch_lan1_admin"].value=="down"&&dsa["switch_lan1_uplink"].value=="eth0","native physical carrier and admin flag separate with board mapping");
 auto optional=rmp::CollectSwitchPorts(R"({"backend":"dsa","ports":[{"id":"optional","system_name":"lan1"}]})",NULL,root);
 Check(rmp::ValidateSwitchProbe(R"({"backend":"dsa","ports":[{"id":"optional","system_name":"lan1"}]})"),"missing numeric port accepted for system matching");
 Check(optional["switch_optional_port"].value=="1"&&optional["switch_optional_state"].value=="up","omitted port preserves detected number rather than inventing zero");
 Check(!rmp::ValidateSwitchProbe(R"({"backend":"swconfig","ports":[{"id":"p1","switch_id":"switch0"}]})"),"chip matching needs a port number");
 Check(rmp::ValidateSwitchProbe(R"({"backend":"swconfig","ports":[{"id":"p1","switch_id":"switch0","port":0}]})"),"zero is a real port");
 auto missing=rmp::CollectSwitchPorts(R"({"backend":"dsa","ports":[{"id":"missing","system_name":"absent"}]})",NULL,root);
 Check(missing["switch_missing_port"].status=="unknown"&&missing["switch_missing_state"].status=="unknown","unmatched record does not invent a zero port or DOWN");
 auto path=std::string(getenv("PATH")?getenv("PATH"):"");setenv("PATH",root.c_str(),1);
 Put(root+"/swconfig","#!/bin/sh\ncase \"$1\" in list) printf 'Found: switch0 - fixture\\n';; dev) printf 'Port 1:\\n  link: port:1 link:up speed:1000baseT full-duplex\\nPort 2:\\n  link: port:2 link:down\\nPort 6:\\n  link: port:6 link:up speed:1000baseT full-duplex\\n';; esac\n");chmod((root+"/swconfig").c_str(),0700);
 auto sw=rmp::CollectSwitchPorts("{\"backend\":\"swconfig\",\"ports\":[{\"id\":\"lan1\",\"switch_id\":\"switch0\",\"port\":1,\"system_name\":\"vlan3\",\"uplink\":\"eth0\",\"display_name\":\"LAN1\",\"role\":\"external\"},{\"id\":\"cpu\",\"switch_id\":\"switch0\",\"port\":6,\"role\":\"cpu\"}]}",NULL,root);
 Check(sw.size()==30&&sw["switch_lan1_state"].value=="up"&&sw["switch_lan1_speed"].value=="1000"&&sw["switch_cpu_role"].value=="cpu"&&sw["switch_lan1_admin"].status=="unknown","swconfig driver maps real switch ports and internal CPU uplink without duplicates");
 auto vendor=rmp::CollectSwitchPorts(R"({"backend":"command","command":"printf 'v1\t-\t-\teth0\t-\tup\tunknown\t1000\tfull\n'","ports":[{"id":"v1","system_name":"eth0","display_name":"LAN1"}]})",NULL,root);
 Check(vendor["switch_v1_state"].value=="up"&&vendor["switch_v1_label"].value=="LAN1","vendor record ID matching is not replaced by a sysfs lookup");
 auto failed=rmp::CollectSwitchPorts("{\"backend\":\"command\",\"command\":\"exit 1\",\"ports\":[{\"id\":\"lan4\",\"switch_id\":\"switch0\",\"port\":4}]}",NULL,root);
 Check(failed["switch_lan4_state"].status=="unknown"&&failed["switch_collection_status"].status=="error","unavailable hardware never fabricated as DOWN");
 Check(!rmp::ValidateSwitchProbe("{\"backend\":\"auto\",\"ports\":[[{\"id\":\"x\",\"port\":1}]]}"),"nested port arrays rejected");
 Put(root+"/proc/device-tree/model","Neutral Board");rmp::ClientConfig c;rmp::CollectFirmware(&c,root);Check(c.properties["model"]=="Neutral Board"&&c.builtin_errors.count("model")==0,"board model discovery when nvram is unavailable");
 setenv("PATH",path.c_str(),1);
 for(auto f:{"/sys/class/net/lan1/phys_switch_id","/sys/class/net/lan1/phys_port_name","/sys/class/net/lan1/carrier","/sys/class/net/lan1/flags","/sys/class/net/lan1/ifindex","/sys/class/net/lan1/iflink","/sys/class/net/eth0/ifindex","/proc/device-tree/model","/swconfig"})unlink((root+f).c_str());
 for(int n=6;n>=0;--n)rmdir((root+dirs[n]).c_str());rmdir(root.c_str());
 std::cout<<"PASS native ports, swconfig, board mapping, failure state and model fallback fixtures"<<std::endl;
}
