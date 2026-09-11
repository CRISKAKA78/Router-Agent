#ifndef RMP_DEVICE_LOGS_H
#define RMP_DEVICE_LOGS_H
#include "rmp/task.h"
#include <condition_variable>
#include <deque>
#include <thread>
namespace rmp {
bool ParseDeviceLogTask(const std::string&, RouterConfigParams*, std::string*);
ExecResult ExecuteDeviceLogTask(const ExecTask&, const std::atomic<bool>*);
// Session-local, bounded read-only worker. No sockets, no persistent capture.
class DeviceLogReader {
public:
 DeviceLogReader();
 ~DeviceLogReader();
 bool Submit(const std::string&, std::string*);
 bool Next(std::string*);
private:
 void Run();
 std::atomic<bool> stop_;
 std::mutex mutex_;
 std::condition_variable changed_;
 std::deque<std::string> requests_, replies_;
 std::thread worker_;
};
// Exposed for filesystem regression tests; production uses /tmp/.systemlog.
std::string ReadDeviceLog(const std::string& path,const std::string& generation,std::uint64_t offset);
std::string ListDeviceLogs(const std::string& directory);
}
#endif
