#ifndef RMP_JSON_H
#define RMP_JSON_H

#include <cstdint>
#include <map>
#include <string>

namespace rmp {

enum class JsonType {
    kString,
    kBoolean,
    kUnsignedInteger,
    kOther
};

struct JsonValue {
    JsonType type;
    std::string string_value;
    std::string raw_value;
    bool bool_value;
    std::uint64_t unsigned_value;

    JsonValue()
        : type(JsonType::kOther), bool_value(false), unsigned_value(0) {}
};

typedef std::map<std::string, JsonValue> JsonObject;

bool ParseJsonObject(const std::string& input, JsonObject* object, std::string* error);
std::string EscapeJsonString(const std::string& input);

struct RegisterAck {
    std::uint64_t reply_to;
    bool success;
    std::string session_id;
    std::uint32_t heartbeat_interval;
    std::uint64_t server_time;
    std::uint32_t max_control_payload;
    std::uint32_t file_chunk_size;
    std::string error_code;
    std::string message;
    std::uint32_t retry_after;

    RegisterAck()
        : reply_to(0),
          success(false),
          heartbeat_interval(0),
          server_time(0),
          max_control_payload(0),
          file_chunk_size(0),
          retry_after(0) {}
};

bool ParseRegisterAck(const std::string& input, RegisterAck* ack, std::string* error);

struct HeartbeatAck {
    std::uint64_t reply_to;
    std::uint64_t server_time;

    HeartbeatAck() : reply_to(0), server_time(0) {}
};

bool ParseHeartbeatAck(const std::string& input, HeartbeatAck* ack, std::string* error);

}  // namespace rmp

#endif
