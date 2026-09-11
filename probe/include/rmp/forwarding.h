#ifndef RMP_FORWARDING_H
#define RMP_FORWARDING_H
#include "rmp/frame.h"
#include <atomic>
#include <deque>
#include <functional>
#include <mutex>
#include <string>
#include <thread>
namespace rmp {
class ForwardingManager {
public:
 typedef std::function<bool(const std::string&)> Report;
 static std::string Executable();
 static bool Available();
 ForwardingManager(const std::string& session,Report report);
 ~ForwardingManager();
 bool Feed(const Frame& frame,std::string* error);
private:
 void Run();
 std::string session_;Report report_;std::atomic<bool> stop_;std::mutex mu_;std::deque<std::string> queue_;std::thread worker_;
};
}
#endif
