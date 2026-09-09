#include "rmp/port_counters.h"
#include "rmp/switch_probe.h"
#include <iostream>
#include <stdexcept>

static void Check(bool ok,const char*why){if(!ok)throw std::runtime_error(why);}
int main(){
 std::uint64_t rx=0,tx=0;
 Check(rmp::ParseMibBytes("Port 1 MIB counters\nRxGoodByte  : 9307599594\nRxBadByte : 19968\nTxByte : 8118199201\n","RxGoodByte","TxByte",&rx,&tx)&&rx==9307599594ULL&&tx==8118199201ULL,"real FNR100 fields above 32 bits, bad bytes excluded");
 Check(rmp::ParseMibBytes("RxGoodByte: 18446744073709551615\nTxByte: 9007199254740993\n","RxGoodByte","TxByte",&rx,&tx)&&rx==UINT64_MAX&&tx==9007199254740993ULL,"exact uint64 parsing beyond double precision");
 for(auto text:{"RxGoodByte: 1\nRxGoodByte: 2\nTxByte: 3", "RxGoodByte: -1\nTxByte: 0", "RxGoodByte: 18446744073709551616\nTxByte: 0", "recv_good: 50\nTxByte: 5"})Check(!rmp::ParseMibBytes(text,"RxGoodByte","TxByte",&rx,&tx),"duplicate, negative, overflow and packet-only samples rejected");
 rmp::PortCounterSampler sampler;rmp::Metrics m;auto t=std::chrono::steady_clock::now();
 sampler.Observe(m,"lan1","hardware1",9007199254740993ULL,100,64,1000,t);
 Check(m["switch_lan1_rx_bytes_per_sec"].status=="waiting"&&m["switch_lan1_rx_bytes"].value=="0","first sample establishes baseline");
 sampler.Observe(m,"lan1","hardware1",9007199254740996ULL,120,64,1000,t+std::chrono::seconds(2));
 Check(m["switch_lan1_rx_bytes_per_sec"].value=="1.5"&&m["switch_lan1_rx_bytes"].value=="3"&&m["switch_lan1_tx_bytes"].value=="20","subtract exact integers before floating rate");
 sampler.Observe(m,"lan2","hardware2",100,500,64,1000,t);sampler.Observe(m,"lan2","hardware2",100,500,64,1000,t+std::chrono::seconds(2));
 Check(m["switch_lan2_rx_bytes_per_sec"].value=="0"&&m["switch_lan1_rx_bytes"].value=="3","ports isolated, actual zero preserved");
 sampler.Observe(m,"lan1","hardware1",1,1,64,1000,t+std::chrono::seconds(4));Check(m["switch_lan1_rx_bytes_per_sec"].status=="waiting","counter reset never creates negative or giant rate");
 sampler.Observe(m,"lan2","changed-source",200,600,64,1000,t+std::chrono::seconds(4));Check(m["switch_lan2_rx_bytes"].value=="0","changed source restarts statistics");
 sampler.Observe(m,"narrow","32",10,10,32,1000,t);sampler.Observe(m,"narrow","32",30,30,32,1000,t+std::chrono::seconds(40));Check(m["switch_narrow_rx_bytes_per_sec"].status=="waiting","32 bit long gaps cannot hide whole wraps");
 sampler.Observe(m,"narrow","32",4294967296ULL,30,32,1000,t+std::chrono::seconds(41));Check(m["switch_narrow_rx_raw_bytes"].status=="error","width overflow rejected");
 const std::string config=R"({"backend":"command","command":"printf 'lan1\t-\t-\teth1\t-\tup\tunknown\t1000\tfull\n'","ports":[{"id":"lan1","display_name":"LAN1","role":"external"}],"counters":{"backend":"command","command":"printf 'lan1\t9007199254740993\t9007199254740999\n'","bits":64,"basis":"raw cumulative bytes"}})";
 Check(rmp::ValidateSwitchProbe(config),"independent counter configuration accepted");auto collected=sampler.Collect(config,NULL);
 Check(collected["switch_lan1_rx_raw_bytes"].value=="9007199254740993"&&collected["switch_lan1_state"].value=="up","independent commands join by stable port id");
 auto failed=config;auto at=failed.find("printf 'lan1\\t900");failed.replace(at,failed.find('"',at)-at,"exit 1");auto errors=sampler.Collect(failed,NULL);
 Check(errors["switch_lan1_state"].value=="up"&&errors["switch_lan1_rx_bytes_per_sec"].status=="error","counter failure retains independent link status without interface fallback");
 sampler.Collect(config,NULL);std::atomic<bool> stop(true);sampler.Collect(config,&stop);stop=false;
 Check(sampler.Collect(config,&stop)["switch_lan1_rx_bytes_per_sec"].status=="ok","control cancellation preserves process-level counter baseline");
 Check(!rmp::ValidateSwitchProbe(R"({"backend":"swconfig","ports":[{"id":"lan1","switch_id":"switch0;bad","port":1}],"counters":{"backend":"swconfig_mib","rx_field":"RX","tx_field":"TX","bits":64,"basis":"bytes"}})"),"switch instance cannot inject shell command");
 std::cout<<"PASS exact physical counters, reset/width/source isolation and command collection"<<std::endl;
}
