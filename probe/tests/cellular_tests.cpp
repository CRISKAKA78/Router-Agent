#include "rmp/cellular.h"
#include "rmp/collection.h"
#include <atomic>
#include <chrono>
#include <cstdlib>
#include <cstring>
#include <fcntl.h>
#include <fstream>
#include <functional>
#include <iostream>
#include <mutex>
#include <stdexcept>
#include <sys/ioctl.h>
#include <sys/stat.h>
#include <sys/sysmacros.h>
#include <termios.h>
#include <thread>
#include <unistd.h>
using namespace rmp;
static void Check(bool b,const char* message){if(!b)throw std::runtime_error(message);}
static void Mkdir(const std::string& s){for(std::size_t i=1;i<=s.size();++i)if(i==s.size()||s[i]=='/')mkdir(s.substr(0,i).c_str(),0700);}
static void Write(const std::string& p,const std::string& s){std::ofstream f(p.c_str());f<<s;}
struct Fixture {
 std::string root;
 Fixture(){char p[]="/tmp/rmp-cellular-XXXXXX";char* d=mkdtemp(p);Check(d,"mkdtemp");root=d;Mkdir(root+"/dev");Mkdir(root+"/sys/class/tty");Check(symlink("/proc",(root+"/proc").c_str())==0,"proc link");}
};
class Modem {
public:
 int master;std::string slave,name,root,device;std::atomic<bool> stop{false};std::thread worker;std::mutex mutex;std::vector<std::string> commands;
 std::function<std::string(const std::string&)> respond;
 Modem(Fixture& f,std::string n,std::string group="1-1",std::function<std::string(const std::string&)> r={}):name(n),root(f.root),device(root+"/sys/devices/platform/usb1/"+group),respond(r){
  master=posix_openpt(O_RDWR|O_NOCTTY|O_NONBLOCK);Check(master>=0&&grantpt(master)==0&&unlockpt(master)==0,"pty");slave=ptsname(master);
  if(!respond)respond=[](const std::string& c){if(c=="AT")return "OK\r\n";if(c=="ATI")return "+CREG: 1\r\nGeneric multi-vendor modem\r\nRevision 1.0\r\nOK\r\n";if(c=="AT+CGSN")return "867123456789012\r\nOK\r\n";return "ERROR\r\n";};
  Link();worker=std::thread([this]{std::string line;while(!stop){char b[256];auto n=read(master,b,sizeof b);if(n<=0){std::this_thread::sleep_for(std::chrono::milliseconds(2));continue;}for(ssize_t i=0;i<n;++i){if(b[i]=='\r'){auto c=line;line.clear();{std::lock_guard<std::mutex> l(mutex);commands.push_back(c);}auto answer=respond(c);if(!answer.empty()){answer=c+"\r\n"+answer;auto half=answer.size()/2;write(master,answer.data(),half);std::this_thread::sleep_for(std::chrono::milliseconds(2));write(master,answer.data()+half,answer.size()-half);}}else if(b[i]!='\n')line+=b[i];}}});
 }
 void Link(){Mkdir(device+"/interface");Write(device+"/idVendor","1234");Write(device+"/idProduct","5678");Mkdir(root+"/sys/class/tty/"+name);Check(symlink((device+"/interface").c_str(),(root+"/sys/class/tty/"+name+"/device").c_str())==0,"sysfs link");Check(symlink(slave.c_str(),(root+"/dev/"+name).c_str())==0,"device link");struct stat st;Check(stat(slave.c_str(),&st)==0,"slave stat");Write(root+"/sys/class/tty/"+name+"/dev",std::to_string(major(st.st_rdev))+":"+std::to_string(minor(st.st_rdev)));}
 void Rename(const std::string& n){unlink((root+"/dev/"+name).c_str());unlink((root+"/sys/class/tty/"+name+"/dev").c_str());unlink((root+"/sys/class/tty/"+name+"/device").c_str());rmdir((root+"/sys/class/tty/"+name).c_str());name=n;Link();}
 std::vector<std::string> Seen(){std::lock_guard<std::mutex> l(mutex);return commands;}
 ~Modem(){stop=true;worker.join();close(master);}
};
static CellularObservation Sample(Fixture& f,unsigned* cursor=NULL,unsigned round=3000){unsigned c=0;return SampleCellular(f.root,NULL,cursor?cursor:&c,180,round);}
int main(){try{
 CellularPlan p;Check(ParseCellularPlan("{}",&p)&&p.interval==30,"default config");Check(!ParseCellularPlan("{\"interval_seconds\":0}",&p)&&!ParseCellularPlan("{\"command\":\"ATZ\"}",&p)&&!ParseCellularPlan("null",&p),"strict config");
 Check(ParseIMEI("+CGSN: \"867123456789012\"")=="867123456789012","quoted IMEI");Check(ParseIMEI("867123456789012\n867123456789012").empty()&&ParseIMEI("8671234567890123").empty()&&ParseIMEI("serial 867123456789012").empty(),"strict IMEI");
 {Fixture f;Check(Sample(f).status=="no_ports","no ports");Modem uart(f,"ttyS0");Check(Sample(f).ports.empty(),"UART excluded");}
 {Fixture f;Modem m(f,"ttyUSB2");auto v=Sample(f);Check(v.status=="ok"&&v.ports.size()==1&&v.ports[0].selected&&v.ports[0].ati.value=="Generic multi-vendor modem\nRevision 1.0"&&v.ports[0].imei.value=="867123456789012","identity roundtrip/echo/URC/fragments");auto key=v.ports[0].device_key;m.Rename("ttyUSB9");v=Sample(f);Check(v.ports.size()==1&&v.ports[0].path=="/dev/ttyUSB9"&&v.ports[0].device_key==key&&v.status=="ok","renumber retains physical group");Check(CellularEvent(v,7,30,65536).find("\"config_revision\":7")!=std::string::npos,"wire");Check(CellularEvent(v,7,30,250).find("payload_limit")!=std::string::npos,"bounded wire");int fd=open(m.slave.c_str(),O_RDWR|O_NOCTTY|O_NONBLOCK);Check(fd>=0,"exclusive lease released");close(fd);}
 {Fixture f;Modem busy(f,"ttyUSB0"),available(f,"ttyUSB1");int fd=open(busy.slave.c_str(),O_RDWR|O_NOCTTY|O_NONBLOCK);Check(fd>=0,"busy open");struct termios before,after;tcgetattr(fd,&before);auto v=Sample(f);tcgetattr(fd,&after);Check(v.ports[0].status=="busy"&&v.ports[1].selected&&busy.Seen().empty(),"occupied port skipped, alternate selected");Check(before.c_lflag==after.c_lflag&&before.c_cflag==after.c_cflag,"occupied terminal unchanged");close(fd);}
 {Fixture f;Modem a(f,"ttyUSB0","1-1"),b(f,"ttyACM0","1-2");auto v=Sample(f);Check(v.ports.size()==2&&v.ports[0].selected&&v.ports[1].selected,"multiple USB devices independent");}
 {Fixture f;Modem a(f,"ttyUSB0"),b(f,"ttyUSB1");auto v=Sample(f);Check(v.ports[0].selected&&v.ports[1].status=="alternate"&&b.Seen().empty(),"one successful endpoint per physical device");}
 {Fixture f;Modem m(f,"ttyUSB0","1-1",[](const std::string& c){if(c=="AT")return std::string("OK\r\n");if(c=="ATI")return std::string("Another vendor\r\nOK\r\n");if(c=="AT+CGSN=1")return std::string("+CGSN: \"867123456789012\"\r\nOK\r\n");return std::string("ERROR\r\n");});auto v=Sample(f);Check(v.status=="ok"&&v.ports[0].imei.command=="AT+CGSN=1","IMEI fallback");}
 {Fixture f;Modem m(f,"ttyUSB0","1-1",[](const std::string& c){if(c=="AT")return std::string("OK\r\n");if(c=="ATI")return std::string("Serial device\r\nOK\r\n");return std::string("SERIAL-NOT-IMEI\r\nOK\r\n");});auto v=Sample(f);Check(v.status=="partial"&&v.ports[0].imei.value.empty()&&v.ports[0].imei.status=="invalid_value","AT success does not invent IMEI");auto commands=m.Seen();Check(commands.size()==5&&commands.back()=="AT+GSN","bounded fixed queries");}
 {Fixture f;Modem m(f,"ttyUSB0","1-1",[](const std::string& c){return c=="AT"?std::string("OK\r\n"):std::string();});auto v=Sample(f);Check(v.status=="partial"&&v.ports[0].ati.status=="timeout"&&v.ports[0].imei.status=="not_queried"&&m.Seen().size()==2,"timeout stops transaction chain");}
 {Fixture f;Modem m(f,"ttyUSB0","1-1",[](const std::string&){return std::string();});std::atomic<bool> stop{false};unsigned cursor=0;auto start=std::chrono::steady_clock::now();std::thread cancel([&]{std::this_thread::sleep_for(std::chrono::milliseconds(80));stop=true;});SampleCellular(f.root,&stop,&cursor,2000,15000);cancel.join();Check(std::chrono::steady_clock::now()-start<std::chrono::milliseconds(500),"cancellation bounded");}
 {Fixture f;Modem slow(f,"ttyUSB0","1-1",[](const std::string&){return std::string();});Modem good(f,"ttyUSB1","1-2");unsigned cursor=0;auto first=Sample(f,&cursor,200);Check(first.limited,"round bounded");auto next=Sample(f,&cursor);Check(next.ports[1].selected,"round robin reaches later device");}
 {Check(ParseCellularPlan("{\"telemetry\":true}",&p)&&p.telemetry,"telemetry opt-in");Check(!ParseCellularPlan("{\"telemetry\":null}",&p),"telemetry strict bool");}
 {Fixture f;Modem m(f,"ttyUSB1","1-1",[](const std::string& c){
  if(c=="ATI")return std::string("Manufacturer: Fibocom Wireless Inc.\r\nModel: FM160-CN\r\nRevision: fixture\r\nOK\r\n");
  if(c=="AT+CGSN")return std::string("867123456789012\r\nOK\r\n");
  if(c=="AT+CPIN?")return std::string("+CEREG: 1\r\n+CPIN: READY\r\nOK\r\n");
  if(c=="AT+CESQ")return std::string("+CESQ: 99,99,255,255,14,33,255,255,255\r\nOK\r\n");
  return c=="AT"?std::string("OK\r\n"):std::string("ERROR\r\n");});
  unsigned cursor=0;auto v=SampleCellular(f.root,NULL,&cursor,180,5000,true);
  Check(v.telemetry&&v.ports[0].profile=="fibocom-fm160-v1"&&v.ports[0].queries.size()==8,"FM160 exact-profile selection");
  Check(v.ports[0].queries[0].value=="+CPIN: READY"&&v.ports[0].queries[7].value.find("+CESQ:")==0,"expected response prefix survives URC filter");
  Check(CellularEvent(v,1,30,65536).find("cellular_telemetry")!=std::string::npos,"versioned telemetry event");
 }
 {Fixture f;Modem m(f,"ttyUSB1");unsigned cursor=0;auto v=SampleCellular(f.root,NULL,&cursor,180,3000,true);
  Check(v.telemetry&&v.ports[0].profile.empty()&&v.ports[0].queries.empty()&&m.Seen().size()==3,"unknown module never receives vendor queries");}

 Check(!ParseCellularPlan("{\"details\":true}",&p),"details require telemetry");
 Check(ParseCellularPlan("{\"telemetry\":true,\"details\":true}",&p)&&p.details,"details opt in");
 for(const std::string mode:{"0","2"}){
  Fixture f;Modem m(f,"ttyUSB1","1-1",[&](const std::string& c){
   if(c=="ATI")return std::string("Manufacturer: Fibocom Wireless Inc.\r\nModel: FM160-CN\r\nRevision: fixture\r\nOK\r\n");
   if(c=="AT+CGSN")return std::string("867123456789012\r\nOK\r\n");
   if(c=="AT+MTSM?")return std::string("+MTSM: ")+mode+"\r\nOK\r\n";
   if(c=="AT+GTCCINFO?")return std::string("+GTCCINFO:\r\nNR service cell:\r\n1,9,460,11,010203,0000012345,99240,C6,5078,100,91,74,74,67\r\nOK\r\n");
   return c=="AT"?std::string("OK\r\n"):std::string("ERROR\r\n");});
  unsigned cursor=0;auto v=SampleCellular(f.root,NULL,&cursor,180,5000,true,true);
  Check(v.details&&v.ports[0].profile=="fibocom-fm160-details-v1"&&v.ports[0].queries.size()==24,"details profile and fixed query list");
  auto seen=m.Seen();bool temp=false;for(const auto& c:seen){if(c=="AT+MTSM=1")temp=true;Check(c!="AT+GTCELLINFO=1"&&c!="AT+GTCELLLOCK=1"&&c!="AT+GTACT=14","no network configuration writes");}
  Check(temp==(mode=="0"),"one-shot temperature preserves existing periodic reporting");
  Check(v.ports[0].queries.back().value.find("NR service cell:")!=std::string::npos,"multiline cell response preserved");
  Check(CellularEvent(v,1,30,65536).find("cellular_details")!=std::string::npos,"details event version");
 }
 std::cout<<"cellular native: configuration, safe enumeration, occupancy, grouping, ATI/IMEI, renumber, timeout, cancellation, bounded payload passed\n";return 0;
 }catch(const std::exception& e){std::cerr<<e.what()<<"\n";return 1;}}
