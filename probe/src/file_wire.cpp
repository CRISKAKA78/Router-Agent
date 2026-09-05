#include "rmp/file_wire.h"
#include <limits>
#include <sstream>
#include <stdexcept>
namespace rmp {
bool FileParams::operator==(const FileParams &b) const {
    return transfer_id == b.transfer_id && remote_path == b.remote_path && sha256 == b.sha256 &&
           mode == b.mode && result_name == b.result_name && size == b.size && overwrite == b.overwrite;
}
namespace {
int Hex(char c) {
    if (c >= '0' && c <= '9')
        return c - '0';
    if (c >= 'a' && c <= 'f')
        return c - 'a' + 10;
    return -1;
}
const JsonValue &Field(const JsonObject &o, const char *k, JsonType t) {
    JsonObject::const_iterator i = o.find(k);
    if (i == o.end() || i->second.type != t)
        throw std::runtime_error(std::string("invalid file field: ") + k);
    return i->second;
}
void Require(bool b, const char *e) {
    if (!b)
        throw std::runtime_error(e);
}
std::string Q(const std::string &s) { return EscapeJsonString(s); }
} // namespace
std::string FileString(const JsonObject &o, const char *k) {
    std::string s = Field(o, k, JsonType::kString).string_value;
    if (s.find('\0') != std::string::npos)
        throw std::runtime_error("NUL in file field");
    return s;
}
std::uint64_t FileUInt(const JsonObject &o, const char *k) {
    return Field(o, k, JsonType::kUnsignedInteger).unsigned_value;
}
bool FileBool(const JsonObject &o, const char *k) { return Field(o, k, JsonType::kBoolean).bool_value; }
bool FileUUID(const std::string &s, std::string *bytes) {
    if (s.size() != 36)
        return false;
    std::string b;
    for (std::size_t i = 0; i < s.size();) {
        if (i == 8 || i == 13 || i == 18 || i == 23) {
            if (s[i++] != '-')
                return false;
        } else {
            int a = Hex(s[i]), c = Hex(s[i + 1]);
            if (a < 0 || c < 0)
                return false;
            b += static_cast<char>((a << 4) | c);
            i += 2;
        }
    }
    if (bytes)
        *bytes = b;
    return true;
}
bool FileName(const std::string &s) {
    return !s.empty() && s.size() <= 255 && s != "." && s != ".." &&
           s.find_first_of("/\\") == std::string::npos && s.find('\0') == std::string::npos;
}
bool FileDigest(const std::string &s) {
    if (s.size() != 64)
        return false;
    for (std::size_t i = 0; i < s.size(); i++)
        if (Hex(s[i]) < 0)
            return false;
    return true;
}
bool ParseFileParams(const std::string &raw, const std::string &kind, FileParams *p, std::string *error) {
    try {
        JsonObject o;
        Require(ParseJsonObject(raw, &o, error), "invalid file params");
        p->transfer_id = FileString(o, "transfer_id");
        p->remote_path = FileString(o, "remote_path");
        Require(FileUUID(p->transfer_id), "invalid transfer UUID");
        Require(!p->remote_path.empty() && p->remote_path[0] == '/' && p->remote_path.size() <= 4096,
                "invalid remote_path");
        if (kind == "upload") {
            p->size = FileUInt(o, "size");
            p->sha256 = FileString(o, "sha256");
            p->mode = FileString(o, "mode");
            p->overwrite = FileBool(o, "overwrite");
            Require(p->size <= std::uint64_t(std::numeric_limits<std::int64_t>::max()) &&
                        FileDigest(p->sha256),
                    "invalid size/SHA-256");
            Require(p->mode.size() == 4 && p->mode[0] == '0' &&
                        p->mode.find_first_not_of("01234567") == std::string::npos,
                    "invalid mode");
        } else {
            p->result_name = FileString(o, "result_name");
            Require(FileName(p->result_name), "invalid result_name");
        }
        return true;
    } catch (const std::exception &e) {
        *error = e.what();
        return false;
    }
}
bool ParseFileBegin(const std::string &raw, std::uint32_t limit, FileBegin *b, std::string *error) {
    try {
        JsonObject o;
        Require(ParseJsonObject(raw, &o, error), "invalid FILE_BEGIN");
        b->direction = FileString(o, "direction");
        b->name = FileString(o, "name");
        b->task_id = FileString(o, "task_id");
        Require(!b->task_id.empty() && b->task_id.size() <= 128 && FileName(b->name),
                "invalid BEGIN identity");
        b->file.transfer_id = FileString(o, "transfer_id");
        b->file.remote_path = FileString(o, "remote_path");
        b->file.size = FileUInt(o, "size");
        b->file.sha256 = FileString(o, "sha256");
        std::uint64_t n = FileUInt(o, "chunk_size");
        Require(n > 0 && n <= limit, "invalid chunk_size");
        b->chunk_size = static_cast<std::uint32_t>(n);
        Require(FileUUID(b->file.transfer_id) && !b->file.remote_path.empty() &&
                    b->file.remote_path[0] == '/' && b->file.remote_path.size() <= 4096 &&
                    b->file.size <= std::uint64_t(std::numeric_limits<std::int64_t>::max()) &&
                    FileDigest(b->file.sha256),
                "invalid BEGIN metadata");
        if (b->direction == "server_to_device") {
            b->file.mode = FileString(o, "mode");
            b->file.overwrite = FileBool(o, "overwrite");
        } else {
            Require(b->direction == "device_to_server" && o.count("mode") == 0 && o.count("overwrite") == 0,
                    "invalid download BEGIN");
        }
        return true;
    } catch (const std::exception &e) {
        *error = e.what();
        return false;
    }
}
std::string FileEndPayload(const FileParams &p) {
    return "{\"transfer_id\":" + Q(p.transfer_id) + ",\"size\":" + std::to_string(p.size) +
           ",\"sha256\":" + Q(p.sha256) + "}";
}
std::string FileBeginPayload(const FileBegin &b) {
    std::string s = FileEndPayload(b.file);
    s.pop_back();
    s += ",\"task_id\":" + Q(b.task_id) + ",\"direction\":" + Q(b.direction) + ",\"name\":" + Q(b.name) +
         ",\"remote_path\":" + Q(b.file.remote_path) + ",\"chunk_size\":" + std::to_string(b.chunk_size);
    if (b.direction == "server_to_device")
        s += ",\"mode\":" + Q(b.file.mode) + ",\"overwrite\":" + (b.file.overwrite ? "true" : "false");
    return s + "}";
}
std::string FileAckPayload(std::uint64_t reply, const std::string &id, const std::string &status,
                           std::uint64_t n) {
    std::string s = "{\"reply_to\":" + std::to_string(reply) + ",\"transfer_id\":" + Q(id) +
                    ",\"status\":" + Q(status) + ",\"received\":" + std::to_string(n);
    if (status != "ready")
        s += std::string(",\"sha256_ok\":") + (status == "done" ? "true" : "false");
    return s + "}";
}
bool CheckFileAck(const std::string &raw, std::uint64_t reply, const std::string &id,
                  const std::string &phase, std::uint64_t size, bool *accepted, std::string *error) {
    try {
        JsonObject o;
        Require(ParseJsonObject(raw, &o, error), "invalid ACK JSON");
        Require(FileUInt(o, "reply_to") == reply && FileString(o, "transfer_id") == id,
                "ACK correlation mismatch");
        if (o.count("message"))
            Require(FileString(o, "message").size() <= 512, "invalid ACK message");
        std::string status = FileString(o, "status");
        std::uint64_t n = FileUInt(o, "received");
        Require(n <= std::uint64_t(std::numeric_limits<std::int64_t>::max()) &&
                    (status == phase || status == "failed"),
                "invalid ACK state");
        if (status == "ready")
            Require(n == 0 && o.count("sha256_ok") == 0, "ready must omit sha256_ok");
        else
            Require(FileBool(o, "sha256_ok") == (status == "done") && (status != "done" || n == size),
                    "invalid ACK checksum");
        *accepted = status == phase;
        return true;
    } catch (const std::exception &e) {
        *error = e.what();
        return false;
    }
}
std::string FileChunkPayload(const std::string &id, std::uint64_t off, const char *data, std::size_t n) {
    std::string s;
    FileUUID(id, &s);
    s.resize(28);
    for (int i = 23; i >= 16; i--) {
        s[i] = static_cast<char>(off);
        off >>= 8;
    }
    std::uint32_t len = static_cast<std::uint32_t>(n);
    for (int i = 27; i >= 24; i--) {
        s[i] = static_cast<char>(len);
        len >>= 8;
    }
    s.append(data, n);
    return s;
}
bool CheckFileChunk(const std::vector<std::uint8_t> &b, const FileParams &p, std::uint64_t off,
                    std::uint32_t limit, std::string *error) {
    std::string u;
    FileUUID(p.transfer_id, &u);
    if (b.size() < 29 || std::string(b.begin(), b.begin() + 16) != u) {
        *error = "invalid CHUNK identity";
        return false;
    }
    std::uint64_t o = 0;
    std::uint32_t n = 0;
    for (unsigned i = 16; i < 24; i++)
        o = (o << 8) | b[i];
    for (unsigned i = 24; i < 28; i++)
        n = (n << 8) | b[i];
    if (o != off || n == 0 || n > limit || n != b.size() - 28 || off > p.size || n > p.size - off) {
        *error = "invalid CHUNK offset/length";
        return false;
    }
    return true;
}
} // namespace rmp
