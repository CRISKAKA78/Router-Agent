#include "rmp/live_config.h"
#include "rmp/json.h"
#include <sstream>
namespace rmp {
static std::string Ack(std::uint64_t reply,std::uint64_t revision,bool success,const std::string&error=""){
 return "{\"reply_to\":"+std::to_string(reply)+",\"revision\":"+std::to_string(revision)+",\"success\":"+(success?"true":"false")+",\"error\":"+EscapeJsonString(error)+"}";
}
LiveTelemetry::LiveTelemetry(const ClientConfig&c,SystemSampler*s):initial_(c),sampler_(s){
 worker_=std::thread(&LiveTelemetry::Run,this);
}
LiveTelemetry::~LiveTelemetry(){stop_=true;if(worker_.joinable())worker_.join();collector_.reset();sampler_->NeighborCollector()->Configure("",0);}
bool LiveTelemetry::Apply(const std::string&bytes,std::uint64_t reply,std::string*error){
 if(error)error->clear();
 JsonObject root;CollectionTemplate t;std::string why;
 if(bytes.size()>65536||!ParseJsonObject(bytes,&root,&why)||root["revision"].type!=JsonType::kUnsignedInteger||root["revision"].unsigned_value==0||root["template_generation"].type!=JsonType::kUnsignedInteger||!ParseCollectionTemplate(root["template"].raw_value,&t,&why)){std::lock_guard<std::mutex>l(mutex_);ack_=Ack(reply,root["revision"].unsigned_value,false,"invalid_configuration");return true;}
 auto revision=root["revision"].unsigned_value;
 std::lock_guard<std::mutex>l(mutex_);
 if(revision<revision_||revision<pending_revision_){ack_=Ack(reply,revision,false,"old_revision");return true;}
 if(revision==revision_){ack_=Ack(reply,revision,bytes==applied_bytes_,bytes==applied_bytes_?"":"revision_conflict");return true;}
 if(revision==pending_revision_){if(bytes!=pending_bytes_)ack_=Ack(reply,revision,false,"revision_conflict");else reply_=reply;return true;}
 ClientConfig c=initial_;c.config_revision=revision;c.template_generation=root["template_generation"].unsigned_value;c.collection_json=t.raw_json;c.explicit_hostname=false;
 c.monitoring={{"cpu",5},{"memory",5},{"disk",60},{"network",5},{"egress",600}};
 for(const auto&p:t.monitoring)c.monitoring[p.first]=p.second;
 if(t.has_network_interfaces)c.network_interfaces=t.network_interfaces;
 else c.network_interfaces=initial_.default_network_interfaces;
 c.switch_json=t.switch_json;c.neighbor_json=t.neighbor_json;c.cellular_json=t.cellular_json;
 pending_=c;pending_bytes_=bytes;pending_revision_=revision;reply_=reply;requested_=true;return true;
}
static std::string NonNetworkPlan(const ClientConfig&config){
 JsonObject root;std::string error,result;
 if(!ParseJsonObject(config.collection_json,&root,&error))return config.collection_json;
 for(const auto& item:root)if(item.first!="monitoring")result+=item.first+":"+item.second.raw_value+";";
 for(const auto& item:config.monitoring)if(item.first!="network")result+=item.first+":"+std::to_string(item.second)+";";
 return result;
}
void LiveTelemetry::Run(){
 while(!stop_){
  ClientConfig next,previous;std::string bytes;std::uint64_t rev=0,reply=0;std::unique_ptr<TelemetryCollector> old;
  {std::lock_guard<std::mutex> lock(mutex_);if(requested_){requested_=false;next=pending_;previous=applied_;bytes=pending_bytes_;rev=pending_revision_;reply=reply_;old=std::move(collector_);ready_=false;}}
  if(rev){
   std::unique_ptr<TelemetryCollector> fresh;
   if(old&&next.template_generation==previous.template_generation&&NonNetworkPlan(next)==NonNetworkPlan(previous)){
    old->ReconfigureNetwork(next);fresh=std::move(old);
   }else{
    old.reset();if(stop_)return;
    if(next.switch_json.empty()||!next.monitoring.at("network"))sampler_->DisableSwitchCounters();
    sampler_->SetInterfaces(next.network_interfaces);
    fresh.reset(new TelemetryCollector(next,sampler_));
   }
   std::lock_guard<std::mutex> lock(mutex_);
   sampler_->NeighborCollector()->Configure(next.neighbor_json,rev);
   collector_=std::move(fresh);applied_=next;revision_=rev;applied_bytes_=bytes;
   ack_=Ack(pending_revision_==rev?reply_:reply,rev,true);
  }
  std::this_thread::sleep_for(std::chrono::milliseconds(20));
 }
}
bool LiveTelemetry::NextAck(std::string*out){std::lock_guard<std::mutex>l(mutex_);if(ack_.empty())return false;*out=ack_;ack_.clear();ready_=true;return true;}
bool LiveTelemetry::Next(std::size_t limit,std::string*out){std::lock_guard<std::mutex>l(mutex_);return ready_&&collector_&&(sampler_->NeighborCollector()->Next(limit,out)||collector_->Next(limit,out));}
}
