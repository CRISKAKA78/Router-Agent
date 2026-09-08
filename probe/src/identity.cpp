#include "rmp/identity.h"
#include "rmp/json.h"
#include "rmp/task.h"

namespace rmp {
bool ResolveDeviceId(bool explicit_id, std::string* device_id, std::string* error) {
    if (!explicit_id) {
        ExecTask task;
        task.command = "nvram get SN";
        task.timeout = 5;
        const ExecResult result = ExecuteExec(task, NULL);
        if (result.status != "success" || result.truncated) {
            *error = "nvram get SN failed or timed out; supply --device-id explicitly";
            return false;
        }
        const std::size_t first = result.stdout_text.find_first_not_of(" \t\r\n");
        const std::size_t last = result.stdout_text.find_last_not_of(" \t\r\n");
        *device_id = first == std::string::npos ? "" : result.stdout_text.substr(first, last - first + 1);
    }
    JsonObject parsed;
    if (device_id->empty() || device_id->size() > 128 ||
        (!explicit_id && (device_id->find_first_of("\r\n") != std::string::npos ||
                         device_id->find('\0') != std::string::npos)) ||
        !ParseJsonObject("{\"id\":" + EscapeJsonString(*device_id) + "}", &parsed, error)) {
        *error = "device ID must be a non-empty single-line UTF-8 value of at most 128 bytes; supply --device-id explicitly";
        return false;
    }
    return true;
}
}
