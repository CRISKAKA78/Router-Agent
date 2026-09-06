#include "rmp/tunnel.h"
#include "rmp/client.h"
#include "rmp/json.h"
#ifdef NDEBUG
#undef NDEBUG
#endif
#include <arpa/inet.h>
#include <cassert>
#include <chrono>
#include <csignal>
#include <iostream>
#include <stdexcept>
#include <sys/socket.h>
#include <unistd.h>

static rmp::Frame frame(unsigned char type,const std::string& p){rmp::Frame f;f.header.type=type;f.payload.assign(p.begin(),p.end());return f;}
static std::string connectPayload(const std::string& cid="22222222222222222222222222222222",const std::string& service="web",const std::string& session="session"){
 return "{\"session_id\":"+rmp::EscapeJsonString(session)+",\"maintenance_id\":\"11111111111111111111111111111111\",\"connection_id\":"+rmp::EscapeJsonString(cid)+",\"service\":"+rmp::EscapeJsonString(service)+",\"token\":\"3333333333333333333333333333333333333333333333333333333333333333\",\"data_host\":\"127.0.0.1\",\"data_port\":1,\"timeout_ms\":500,\"idle_ms\":500}";
}
int main(){
 assert(rmp::ClientConfig().tunnel_connections==8);
 for(std::size_t limit: {std::size_t(0),std::size_t(65)}) {
  bool rejected=false;
  try { rmp::TunnelManager invalid("session",limit,[](const std::string&){return true;}); }
  catch(const std::invalid_argument&) { rejected=true; }
  assert(rejected);
 }
 // Tunnel send must never change the process-wide signal disposition.
 std::signal(SIGPIPE,SIG_DFL);
 std::string error;int reports=0;
 const auto start=std::chrono::steady_clock::now();
 {
  rmp::TunnelManager m("session",1,[&](const std::string& p){rmp::JsonObject object;assert(rmp::ParseJsonObject(p,&object,&error));++reports;return true;});
  assert(!m.Feed(frame(rmp::kTypeTunnelConnect,connectPayload("22222222222222222222222222222222","socks")),&error));
  assert(!m.Feed(frame(rmp::kTypeTunnelConnect,connectPayload("22222222222222222222222222222222","web","old-session")),&error));
  assert(m.Feed(frame(rmp::kTypeTunnelConnect,connectPayload()),&error));
  assert(m.Feed(frame(rmp::kTypeTunnelConnect,connectPayload()),&error)); // duplicate has no new worker
  assert(m.Feed(frame(rmp::kTypeTunnelConnect,connectPayload("44444444444444444444444444444444")),&error));
  assert(reports==1); // capacity rejects without unbounded work
  assert(m.Feed(frame(rmp::kTypeTunnelClose,"{\"maintenance_id\":\"11111111111111111111111111111111\",\"connection_id\":\"\"}"),&error));
  assert(m.Feed(frame(rmp::kTypeTunnelClose,"{\"maintenance_id\":\"11111111111111111111111111111111\",\"connection_id\":\"\"}"),&error));
 }
 assert(std::chrono::steady_clock::now()-start<std::chrono::seconds(2));
 struct sigaction action;assert(sigaction(SIGPIPE,NULL,&action)==0);assert(action.sa_handler==SIG_DFL);
 std::cout<<"tunnel validation, bounded admission, duplicate, idempotent cancel, join, SIGPIPE disposition passed\n";
}
