#include "rmp/telemetry.h"
#include "rmp/json.h"
#include <fstream>
#include <iostream>
#include <stdexcept>
#include <sys/stat.h>
#include <unistd.h>
#include <cstdlib>
static void Check(bool value,const char*message){if(!value)throw std::runtime_error(message);}
static void Put(const std::string&p,const std::string&v){std::ofstream f(p.c_str());f<<v;}
int main(){
 char path[]="/tmp/rmp-monitor-XXXXXX";char*made=mkdtemp(path);Check(made,"temp directory");std::string root=made;
 mkdir((root+"/proc").c_str(),0700);mkdir((root+"/proc/net").c_str(),0700);
 Put(root+"/uname_machine","armv7l");Put(root+"/proc/cpuinfo","processor : 0\nCPU implementer : 0x41\nCPU part : 0xd03\nCPU architecture : 8\n");
 auto hardware=rmp::HardwareMetrics(root);Check(hardware["cpu_model"].value=="ARM Cortex-A53","CPU core identification");Check(hardware["cpu_arch"].value=="ARMv8-A"&&hardware["cpu_hardware_bits"].value=="64"&&hardware["kernel_arch"].value=="armv7l"&&hardware["kernel_bits"].value=="32","64 bit hardware with 32 bit kernel");Check(hardware["cpu_max_mhz"].status=="unknown","frequency must not use BogoMIPS");
 Put(root+"/proc/cpuinfo","CPU implementer : 0x41\nCPU part : 0xc09\nCPU architecture : 7\n");hardware=rmp::HardwareMetrics(root);Check(hardware["cpu_model"].value=="ARM Cortex-A9"&&hardware["cpu_hardware_bits"].value=="32","ARMv7 detection");
 rmp::SystemSampler sampler(root);Put(root+"/proc/stat","cpu 10 0 10 80 0 0 0 0 0 0\n");Check(sampler.Sample("cpu")["cpu_usage"].status=="waiting","CPU first delta waits");Put(root+"/proc/stat","cpu 20 0 20 160 0 0 0 0 0 0\n");Check(sampler.Sample("cpu")["cpu_usage"].value=="20.00","CPU normalized whole-machine percent");Put(root+"/proc/stat","cpu 1 0 1 2 0 0 0 0\n");Check(sampler.Sample("cpu")["cpu_usage"].status=="waiting","CPU rollback reset");
 Put(root+"/proc/meminfo","MemTotal: 2097152 kB\nMemAvailable: 1048576 kB\nMemFree: 16 kB\n");auto mem=sampler.Sample("memory");Check(mem["memory_total_bytes"].value=="2147483648"&&mem["memory_usage"].value=="50.00","memory bytes and available calculation");Put(root+"/proc/meminfo","MemTotal: 1000 kB\nMemFree: 100 kB\nBuffers: 100 kB\nCached: 300 kB\nSReclaimable: 100 kB\nShmem: 100 kB\n");mem=sampler.Sample("memory");Check(mem["memory_usage"].value=="50.00"&&mem["memory_method"].value!="MemAvailable","old kernel estimate");
 Put(root+"/proc/net/dev","eth0: 100 0 0 0 0 0 0 0 200 0 0 0 0 0 0 0\nlo: 1 0 0 0 0 0 0 0 1 0 0 0 0 0 0 0\n");auto net=sampler.Sample("network");Check(net.size()==7&&net["net_65746830_rx_bytes_per_sec"].status=="waiting","interface keys and first sample");usleep(20000);Put(root+"/proc/net/dev","eth0: 200 0 0 0 0 0 0 0 400 0 0 0 0 0 0 0\n");net=sampler.Sample("network");Check(net["net_65746830_rx_bytes_per_sec"].status=="ok"&&net["net_65746830_rx_bytes_per_sec"].unit=="bytes_per_sec","interface delta and units");Put(root+"/proc/net/dev","");Check(sampler.Sample("network").empty(),"removed interface cleared");
 mkdir((root+"/proc/self").c_str(),0700);Put(root+"/proc/self/mountinfo","1 0 8:1 / / rw - ext4 /dev/root rw\n2 1 8:1 / /duplicate rw - ext4 /dev/root rw\n3 1 0:2 / /proc rw - proc proc rw\n");auto disks=sampler.Sample("disk");Check(disks.size()==5,"disk mounts filtered and deduplicated");bool capacity=false;for(const auto&p:disks)if(p.first.find("_total_bytes")!=std::string::npos&&p.second.status=="ok"&&p.second.entity=="/")capacity=true;Check(capacity,"real statvfs capacity");unlink((root+"/proc/self/mountinfo").c_str());rmdir((root+"/proc/self").c_str());
 rmp::CollectionTemplate t;std::string error;Check(rmp::ParseCollectionTemplate("{\"template_id\":\"t\",\"name\":\"T\",\"version\":1,\"monitoring\":{\"cpu_seconds\":1,\"disk_seconds\":0},\"properties\":{\"cpu_usage\":{\"name\":\"CPU\",\"command\":\"printf 50\",\"timeout_seconds\":5,\"interval_seconds\":3}}}",&t,&error)&&t.properties["cpu_usage"].interval==3&&t.monitoring["disk"]==0,"independent configuration");
 for(const char*f:{"/proc/net/dev","/proc/stat","/proc/meminfo","/proc/cpuinfo","/uname_machine"})unlink((root+f).c_str());rmdir((root+"/proc/net").c_str());rmdir((root+"/proc").c_str());rmdir(root.c_str());
 std::cout<<"PASS telemetry hardware, CPU, memory, network and configuration"<<std::endl;
}
