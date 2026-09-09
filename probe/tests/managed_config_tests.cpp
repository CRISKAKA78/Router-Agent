#include "rmp/live_config.h"
#include "rmp/switch_probe.h"
#include "rmp/json.h"
#include <iostream>
#include <stdexcept>
#include <unistd.h>
#include <fstream>
#include <cstdio>
static int Lines(const std::string& path){std::ifstream file(path);int count=0;std::string line;while(std::getline(file,line))++count;return count;}
static void Check(bool ok,const char*why){if(!ok)throw std::runtime_error(why);}
static void NetworkOnlyChecks(){
 char path[]="/tmp/rmp-configuration-XXXXXX";int fd=mkstemp(path);Check(fd>=0,"counter file");close(fd);
 rmp::SystemSampler sampler;rmp::ClientConfig config;
 auto bytes=[&](unsigned revision,unsigned generation,unsigned network){
  auto command=rmp::EscapeJsonString(std::string("printf 'run\\n' >> ")+path+"; printf value");
  return std::string("{\"revision\":")+std::to_string(revision)+",\"template_generation\":"+std::to_string(generation)+",\"template\":{\"template_id\":\"repeat\",\"name\":\"T\",\"version\":1,\"monitoring\":{\"cpu_seconds\":0,\"memory_seconds\":0,\"disk_seconds\":0,\"egress_seconds\":0,\"network_seconds\":"+std::to_string(network)+",\"network_interfaces\":\"lo\"},\"properties\":{\"static\":{\"name\":\"Static\",\"timeout_seconds\":1,\"command\":"+command+"},\"periodic\":{\"name\":\"Periodic\",\"timeout_seconds\":1,\"command\":"+command+",\"interval_seconds\":60}}}}";
 };
 auto apply=[&](rmp::LiveTelemetry& live,unsigned rev,unsigned gen,unsigned network){std::string error,ack;Check(live.Apply(bytes(rev,gen,network),rev,&error),"apply");for(int n=0;n<200&&ack.empty();n++){live.NextAck(&ack);usleep(20000);}Check(ack.find("\"success\":true")!=std::string::npos,"configuration acknowledged");};
 {
  rmp::LiveTelemetry live(config,&sampler);apply(live,1,1,1);
  for(int n=0;n<200&&Lines(path)<2;n++)usleep(20000);Check(Lines(path)==2,"static and periodic first sample");
  apply(live,2,1,2);usleep(300000);Check(Lines(path)==2,"network-only update does not execute other properties");
  std::string event;bool cached=false;for(int n=0;n<100&&!cached;n++){if(live.Next(65536,&event)&&event.find("value")!=std::string::npos&&event.find("\"config_revision\":2")!=std::string::npos)cached=true;usleep(20000);}Check(cached,"cached observations are republished at interface revision");
  apply(live,3,2,2);for(int n=0;n<200&&Lines(path)<4;n++)usleep(20000);Check(Lines(path)==4,"explicit same-version application executes properties again");
  apply(live,3,2,2);usleep(200000);Check(Lines(path)==4,"idempotent retry does not execute again");
 }
 {
  rmp::LiveTelemetry reconnected(config,&sampler);apply(reconnected,3,2,2);for(int n=0;n<200&&Lines(path)<5;n++)usleep(20000);Check(Lines(path)==5,"reconnect preserves static cache while periodic collection resumes");
 }
 unlink(path);
}
int main(){
 NetworkOnlyChecks();
 rmp::SystemSampler sampler;sampler.StartHardware(1);Check(sampler.HardwareAttempts()==1,"initial hardware collection");usleep(1100000);Check(sampler.HardwareAttempts()==2,"second hardware collection");auto before=sampler.Hardware();sampler.StartHardware(1);usleep(1100000);Check(sampler.HardwareAttempts()==2&&sampler.Hardware().begin()->second.sampled==before.begin()->second.sampled,"hardware never recollected after second attempt");
 auto ports=rmp::ParseSwitchRows("lan1\tswitch0\t1\teth0\teth0\tup\tunknown\t1000\tfull\nlan2\tswitch0\t2\teth0\teth0\tdown\tup\t-\tunknown\n");Check(ports.size()==20&&ports["switch_lan1_state"].value=="up"&&ports["switch_lan1_admin"].status=="unknown"&&ports["switch_lan2_state"].value=="down","physical link separate from administrative state");Check(rmp::ParseSwitchRows("bad output").empty(),"malformed vendor output rejected");
 Check(rmp::ValidateSwitchProbe("{\"backend\":\"auto\",\"ports\":[]}"),"empty board mapping valid");
 Check(!rmp::ValidateSwitchProbe("{\"backend\":\"made_up\",\"ports\":[]}")&&!rmp::ValidateSwitchProbe("{\"backend\":\"dsa\",\"ports\":[3]}"),"invalid switch backend and port data rejected");
 rmp::ClientConfig c;c.monitoring={{"cpu",0},{"memory",0},{"disk",0},{"network",0},{"egress",0}};
 rmp::LiveTelemetry live(c,&sampler);std::string error,ack;Check(!live.Next(65536,&ack),"no telemetry before admission configuration");
 std::string config="{\"revision\":1,\"template_generation\":1,\"template\":{\"template_id\":\"t\",\"name\":\"T\",\"version\":1,\"monitoring\":{\"egress_seconds\":0},\"properties\":{\"x\":{\"name\":\"X\",\"command\":\"printf value\",\"timeout_seconds\":1}}}}";
 Check(live.Apply(config,2,&error),"configuration accepted");for(int n=0;n<100&&!live.NextAck(&ack);++n)usleep(20000);rmp::JsonObject a;Check(rmp::ParseJsonObject(ack,&a,&error)&&a["success"].bool_value&&a["reply_to"].unsigned_value==2,"applied confirmation");
 bool found=false;for(int n=0;n<100&&!found;++n){std::string event;if(live.Next(65536,&event)&&event.find("value")!=std::string::npos&&event.find("config_revision\":1")!=std::string::npos)found=true;usleep(20000);}Check(found,"startup-only property collected on application and revision stamped");
 Check(live.Apply(config,3,&error)&&live.NextAck(&ack),"duplicate configuration acknowledged without replacement");
 auto bad=config;auto pos=bad.find("printf value");bad.replace(pos,12,"printf other");live.Apply(bad,4,&error);Check(live.NextAck(&ack)&&ack.find("revision_conflict")!=std::string::npos,"same revision conflict rejected");
 std::cout<<"PASS managed configuration, static CPU cache and switch rows"<<std::endl;
}
