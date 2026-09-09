#include "rmp/json.h"

#include <cerrno>
#include <cstdlib>
#include <iomanip>
#include <limits>
#include <sstream>

namespace rmp {
namespace {

bool IsValidUtf8(const std::string& input) {
    std::size_t index = 0;
    while (index < input.size()) {
        const unsigned char first = static_cast<unsigned char>(input[index]);
        if (first <= 0x7F) {
            ++index;
            continue;
        }
        std::size_t continuation = 0;
        std::uint32_t codepoint = 0;
        if (first >= 0xC2 && first <= 0xDF) {
            continuation = 1;
            codepoint = first & 0x1FU;
        } else if (first >= 0xE0 && first <= 0xEF) {
            continuation = 2;
            codepoint = first & 0x0FU;
        } else if (first >= 0xF0 && first <= 0xF4) {
            continuation = 3;
            codepoint = first & 0x07U;
        } else {
            return false;
        }
        if (index + continuation >= input.size()) {
            return false;
        }
        for (std::size_t offset = 1; offset <= continuation; ++offset) {
            const unsigned char next = static_cast<unsigned char>(input[index + offset]);
            if ((next & 0xC0U) != 0x80U) {
                return false;
            }
            codepoint = (codepoint << 6) | (next & 0x3FU);
        }
        if ((continuation == 2 && codepoint < 0x800U) ||
            (continuation == 3 && codepoint < 0x10000U) ||
            codepoint > 0x10FFFFU ||
            (codepoint >= 0xD800U && codepoint <= 0xDFFFU)) {
            return false;
        }
        index += continuation + 1;
    }
    return true;
}

void AppendUtf8(std::uint32_t codepoint, std::string* output) {
    if (codepoint <= 0x7FU) {
        output->push_back(static_cast<char>(codepoint));
    } else if (codepoint <= 0x7FFU) {
        output->push_back(static_cast<char>(0xC0U | (codepoint >> 6)));
        output->push_back(static_cast<char>(0x80U | (codepoint & 0x3FU)));
    } else if (codepoint <= 0xFFFFU) {
        output->push_back(static_cast<char>(0xE0U | (codepoint >> 12)));
        output->push_back(static_cast<char>(0x80U | ((codepoint >> 6) & 0x3FU)));
        output->push_back(static_cast<char>(0x80U | (codepoint & 0x3FU)));
    } else {
        output->push_back(static_cast<char>(0xF0U | (codepoint >> 18)));
        output->push_back(static_cast<char>(0x80U | ((codepoint >> 12) & 0x3FU)));
        output->push_back(static_cast<char>(0x80U | ((codepoint >> 6) & 0x3FU)));
        output->push_back(static_cast<char>(0x80U | (codepoint & 0x3FU)));
    }
}

class Parser {
public:
    Parser(const std::string& input, std::string* error)
        : input_(input), position_(0), error_(error) {}

    bool Parse(JsonObject* object) {
        if (!IsValidUtf8(input_)) {
            return Fail("JSON is not valid UTF-8");
        }
        SkipWhitespace();
        if (!ParseObject(object, 0)) {
            return false;
        }
        SkipWhitespace();
        if (position_ != input_.size()) {
            return Fail("unexpected data after JSON object");
        }
        return true;
    }

private:
    bool ParseObject(JsonObject* object, unsigned depth) {
        if (depth > 64) {
            return Fail("JSON nesting is too deep");
        }
        if (!Consume('{')) {
            return Fail("top-level JSON value must be an object");
        }
        SkipWhitespace();
        if (Consume('}')) {
            return true;
        }
        while (true) {
            std::string key;
            if (!ParseString(&key)) {
                return false;
            }
            SkipWhitespace();
            if (!Consume(':')) {
                return Fail("expected ':' after object key");
            }
            SkipWhitespace();
            JsonValue value;
            if (!ParseValue(&value, depth + 1)) {
                return false;
            }
            if (object != NULL && !object->insert(std::make_pair(key, value)).second) {
                return Fail("duplicate JSON object key");
            }
            SkipWhitespace();
            if (Consume('}')) {
                return true;
            }
            if (!Consume(',')) {
                return Fail("expected ',' or '}' in object");
            }
            SkipWhitespace();
        }
    }

    bool ParseArray(unsigned depth) {
        if (depth > 64) {
            return Fail("JSON nesting is too deep");
        }
        if (!Consume('[')) {
            return false;
        }
        SkipWhitespace();
        if (Consume(']')) {
            return true;
        }
        while (true) {
            JsonValue ignored;
            if (!ParseValue(&ignored, depth + 1)) {
                return false;
            }
            SkipWhitespace();
            if (Consume(']')) {
                return true;
            }
            if (!Consume(',')) {
                return Fail("expected ',' or ']' in array");
            }
            SkipWhitespace();
        }
    }

    bool ParseValue(JsonValue* value, unsigned depth) {
        if (position_ >= input_.size()) {
            return Fail("unexpected end of JSON value");
        }
        const std::size_t start = position_;
        const char next = input_[position_];
        if (next == '"') {
            value->type = JsonType::kString;
            if (!ParseString(&value->string_value)) {
                return false;
            }
            value->raw_value = input_.substr(start, position_ - start);
            return true;
        }
        if (next == '{') {
            value->type = JsonType::kOther;
            if (!ParseObject(NULL, depth)) {
                return false;
            }
            value->raw_value = input_.substr(start, position_ - start);
            return true;
        }
        if (next == '[') {
            value->type = JsonType::kOther;
            if (!ParseArray(depth)) {
                return false;
            }
            value->raw_value = input_.substr(start, position_ - start);
            return true;
        }
        if (MatchLiteral("true")) {
            value->type = JsonType::kBoolean;
            value->bool_value = true;
            value->raw_value = input_.substr(start, position_ - start);
            return true;
        }
        if (MatchLiteral("false")) {
            value->type = JsonType::kBoolean;
            value->bool_value = false;
            value->raw_value = input_.substr(start, position_ - start);
            return true;
        }
        if (MatchLiteral("null")) {
            value->type = JsonType::kOther;
            value->raw_value = input_.substr(start, position_ - start);
            return true;
        }
        if (!ParseNumber(value)) {
            return false;
        }
        value->raw_value = input_.substr(start, position_ - start);
        return true;
    }

    bool ParseNumber(JsonValue* value) {
        const std::size_t start = position_;
        bool negative = false;
        bool integer = true;
        if (Consume('-')) {
            negative = true;
        }
        if (position_ >= input_.size()) {
            return Fail("invalid JSON number");
        }
        if (input_[position_] == '0') {
            ++position_;
            if (position_ < input_.size() && input_[position_] >= '0' && input_[position_] <= '9') {
                return Fail("invalid leading zero in JSON number");
            }
        } else if (input_[position_] >= '1' && input_[position_] <= '9') {
            while (position_ < input_.size() && input_[position_] >= '0' && input_[position_] <= '9') {
                ++position_;
            }
        } else {
            return Fail("invalid JSON number");
        }
        if (position_ < input_.size() && input_[position_] == '.') {
            integer = false;
            ++position_;
            if (position_ >= input_.size() || input_[position_] < '0' || input_[position_] > '9') {
                return Fail("invalid JSON fraction");
            }
            while (position_ < input_.size() && input_[position_] >= '0' && input_[position_] <= '9') {
                ++position_;
            }
        }
        if (position_ < input_.size() && (input_[position_] == 'e' || input_[position_] == 'E')) {
            integer = false;
            ++position_;
            if (position_ < input_.size() && (input_[position_] == '+' || input_[position_] == '-')) {
                ++position_;
            }
            if (position_ >= input_.size() || input_[position_] < '0' || input_[position_] > '9') {
                return Fail("invalid JSON exponent");
            }
            while (position_ < input_.size() && input_[position_] >= '0' && input_[position_] <= '9') {
                ++position_;
            }
        }

        value->type = JsonType::kOther;
        if (negative || !integer) {
            return true;
        }
        const std::string text = input_.substr(start, position_ - start);
        errno = 0;
        char* end = NULL;
        const unsigned long long parsed = std::strtoull(text.c_str(), &end, 10);
        if (errno == ERANGE || end == NULL || *end != '\0') {
            // Valid JSON can carry larger numbers in unknown extension fields.
            // Known integer fields require kUnsignedInteger and reject this value.
            return true;
        }
        value->type = JsonType::kUnsignedInteger;
        value->unsigned_value = static_cast<std::uint64_t>(parsed);
        return true;
    }

    bool ParseString(std::string* output) {
        if (!Consume('"')) {
            return Fail("expected JSON string");
        }
        output->clear();
        while (position_ < input_.size()) {
            const unsigned char current = static_cast<unsigned char>(input_[position_++]);
            if (current == '"') {
                return true;
            }
            if (current < 0x20U) {
                return Fail("unescaped control character in JSON string");
            }
            if (current != '\\') {
                output->push_back(static_cast<char>(current));
                continue;
            }
            if (position_ >= input_.size()) {
                return Fail("unterminated JSON escape");
            }
            const char escaped = input_[position_++];
            switch (escaped) {
            case '"': output->push_back('"'); break;
            case '\\': output->push_back('\\'); break;
            case '/': output->push_back('/'); break;
            case 'b': output->push_back('\b'); break;
            case 'f': output->push_back('\f'); break;
            case 'n': output->push_back('\n'); break;
            case 'r': output->push_back('\r'); break;
            case 't': output->push_back('\t'); break;
            case 'u': {
                std::uint32_t codepoint = 0;
                if (!ParseHex4(&codepoint)) {
                    return false;
                }
                if (codepoint >= 0xD800U && codepoint <= 0xDBFFU) {
                    if (position_ + 2 > input_.size() || input_[position_] != '\\' || input_[position_ + 1] != 'u') {
                        return Fail("high surrogate must be followed by low surrogate");
                    }
                    position_ += 2;
                    std::uint32_t low = 0;
                    if (!ParseHex4(&low)) {
                        return false;
                    }
                    if (low < 0xDC00U || low > 0xDFFFU) {
                        return Fail("invalid low surrogate");
                    }
                    codepoint = 0x10000U + ((codepoint - 0xD800U) << 10) + (low - 0xDC00U);
                } else if (codepoint >= 0xDC00U && codepoint <= 0xDFFFU) {
                    return Fail("unexpected low surrogate");
                }
                AppendUtf8(codepoint, output);
                break;
            }
            default:
                return Fail("invalid JSON escape");
            }
        }
        return Fail("unterminated JSON string");
    }

    bool ParseHex4(std::uint32_t* value) {
        if (position_ + 4 > input_.size()) {
            return Fail("incomplete Unicode escape");
        }
        std::uint32_t parsed = 0;
        for (int count = 0; count < 4; ++count) {
            const char current = input_[position_++];
            parsed <<= 4;
            if (current >= '0' && current <= '9') {
                parsed |= static_cast<std::uint32_t>(current - '0');
            } else if (current >= 'a' && current <= 'f') {
                parsed |= static_cast<std::uint32_t>(current - 'a' + 10);
            } else if (current >= 'A' && current <= 'F') {
                parsed |= static_cast<std::uint32_t>(current - 'A' + 10);
            } else {
                return Fail("invalid Unicode escape");
            }
        }
        *value = parsed;
        return true;
    }

    bool MatchLiteral(const char* literal) {
        const std::size_t start = position_;
        while (*literal != '\0') {
            if (position_ >= input_.size() || input_[position_] != *literal) {
                position_ = start;
                return false;
            }
            ++position_;
            ++literal;
        }
        return true;
    }

    void SkipWhitespace() {
        while (position_ < input_.size()) {
            const char current = input_[position_];
            if (current != ' ' && current != '\t' && current != '\r' && current != '\n') {
                break;
            }
            ++position_;
        }
    }

    bool Consume(char expected) {
        if (position_ < input_.size() && input_[position_] == expected) {
            ++position_;
            return true;
        }
        return false;
    }

    bool Fail(const std::string& message) {
        if (error_ != NULL) {
            std::ostringstream stream;
            stream << message << " at byte " << position_;
            *error_ = stream.str();
        }
        return false;
    }

    const std::string& input_;
    std::size_t position_;
    std::string* error_;
};

bool Required(const JsonObject& object,
              const std::string& name,
              JsonType type,
              const JsonValue** value,
              std::string* error) {
    const JsonObject::const_iterator found = object.find(name);
    if (found == object.end()) {
        if (error != NULL) {
            *error = name + " is required";
        }
        return false;
    }
    if (found->second.type != type) {
        if (error != NULL) {
            *error = name + " has the wrong type";
        }
        return false;
    }
    *value = &found->second;
    return true;
}

bool Optional(const JsonObject& object,
              const std::string& name,
              JsonType type,
              const JsonValue** value,
              std::string* error) {
    const JsonObject::const_iterator found = object.find(name);
    if (found == object.end()) {
        *value = NULL;
        return true;
    }
    if (found->second.type != type) {
        if (error != NULL) {
            *error = name + " has the wrong type";
        }
        return false;
    }
    *value = &found->second;
    return true;
}

bool IsKnownRegisterError(const std::string& error_code) {
    return error_code == "INVALID_REGISTER" ||
           error_code == "DEVICE_REJECTED" ||
           error_code == "UNSUPPORTED_PROBE" ||
           error_code == "SERVER_BUSY";
}

}  // namespace

bool ParseJsonObject(const std::string& input, JsonObject* object, std::string* error) {
    object->clear();
    Parser parser(input, error);
    return parser.Parse(object);
}

std::string EscapeJsonString(const std::string& input) {
    std::ostringstream output;
    output << '"';
    for (std::size_t index = 0; index < input.size(); ++index) {
        const unsigned char current = static_cast<unsigned char>(input[index]);
        switch (current) {
        case '"': output << "\\\""; break;
        case '\\': output << "\\\\"; break;
        case '\b': output << "\\b"; break;
        case '\f': output << "\\f"; break;
        case '\n': output << "\\n"; break;
        case '\r': output << "\\r"; break;
        case '\t': output << "\\t"; break;
        default:
            if (current < 0x20U) {
                output << "\\u" << std::hex << std::uppercase << std::setw(4)
                       << std::setfill('0') << static_cast<unsigned>(current)
                       << std::dec;
            } else {
                output << static_cast<char>(current);
            }
        }
    }
    output << '"';
    return output.str();
}

bool ParseRegisterAck(const std::string& input, RegisterAck* ack, std::string* error) {
    JsonObject object;
    if (!ParseJsonObject(input, &object, error)) {
        return false;
    }
    const JsonValue* value = NULL;
    if (!Required(object, "reply_to", JsonType::kUnsignedInteger, &value, error)) {
        return false;
    }
    ack->reply_to = value->unsigned_value;
    if (!Required(object, "success", JsonType::kBoolean, &value, error)) {
        return false;
    }
    ack->success = value->bool_value;
    if (ack->success) {
        if (!Required(object, "session_id", JsonType::kString, &value, error) ||
            value->string_value.empty() || value->string_value.size() > 128) {
            if (error != NULL && value != NULL && value->type == JsonType::kString) {
                *error = "session_id must be 1-128 bytes";
            }
            return false;
        }
        ack->session_id = value->string_value;
        if (!Required(object, "heartbeat_interval", JsonType::kUnsignedInteger, &value, error) ||
            value->unsigned_value < 10 || value->unsigned_value > 300) {
            if (error != NULL && value != NULL && value->type == JsonType::kUnsignedInteger) {
                *error = "heartbeat_interval must be 10-300";
            }
            return false;
        }
        ack->heartbeat_interval = static_cast<std::uint32_t>(value->unsigned_value);
        if (!Required(object, "server_time", JsonType::kUnsignedInteger, &value, error)) {
            return false;
        }
        ack->server_time = value->unsigned_value;
        if (!Required(object, "max_control_payload", JsonType::kUnsignedInteger, &value, error) ||
            value->unsigned_value < 65536 || value->unsigned_value > 1024U * 1024U) {
            if (error != NULL && value != NULL && value->type == JsonType::kUnsignedInteger) {
                *error = "max_control_payload must be 65536-1048576";
            }
            return false;
        }
        ack->max_control_payload = static_cast<std::uint32_t>(value->unsigned_value);
        if (!Required(object, "file_chunk_size", JsonType::kUnsignedInteger, &value, error) ||
            value->unsigned_value < 1024 || value->unsigned_value > 512U * 1024U) {
            if (error != NULL && value != NULL && value->type == JsonType::kUnsignedInteger) {
                *error = "file_chunk_size must be 1024-524288";
            }
            return false;
        }
        ack->file_chunk_size = static_cast<std::uint32_t>(value->unsigned_value);
        for (const auto& capability : {"telemetry_v2", "managed_config_v1"}) {
            if (!Required(object,capability,JsonType::kBoolean,&value,error) || !value->bool_value) {
                if(error)*error="current server capabilities are required";
                return false;
            }
        }
        return true;
    }

    if (!Required(object, "error_code", JsonType::kString, &value, error) ||
        !IsKnownRegisterError(value->string_value)) {
        if (error != NULL && value != NULL && value->type == JsonType::kString) {
            *error = "error_code is not a Protocol v1 register failure code";
        }
        return false;
    }
    ack->error_code = value->string_value;
    if (!Optional(object, "message", JsonType::kString, &value, error)) {
        return false;
    }
    if (value != NULL) {
        if (value->string_value.size() > 512) {
            if (error != NULL) {
                *error = "message must be at most 512 bytes";
            }
            return false;
        }
        ack->message = value->string_value;
    }
    if (!Optional(object, "retry_after", JsonType::kUnsignedInteger, &value, error)) {
        return false;
    }
    if (value != NULL) {
        if (value->unsigned_value < 1 || value->unsigned_value > 3600) {
            if (error != NULL) {
                *error = "retry_after must be 1-3600";
            }
            return false;
        }
        ack->retry_after = static_cast<std::uint32_t>(value->unsigned_value);
    }
    return true;
}

bool ParseHeartbeatAck(const std::string& input, HeartbeatAck* ack, std::string* error) {
    JsonObject object;
    if (!ParseJsonObject(input, &object, error)) {
        return false;
    }
    const JsonValue* value = NULL;
    if (!Required(object, "reply_to", JsonType::kUnsignedInteger, &value, error)) {
        return false;
    }
    ack->reply_to = value->unsigned_value;
    if (!Required(object, "server_time", JsonType::kUnsignedInteger, &value, error)) {
        return false;
    }
    ack->server_time = value->unsigned_value;
    return true;
}

}  // namespace rmp
