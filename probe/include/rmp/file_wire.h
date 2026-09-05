#ifndef RMP_FILE_WIRE_H
#define RMP_FILE_WIRE_H
#include "rmp/json.h"
#include <vector>
namespace rmp {
struct FileParams {
    std::string transfer_id, remote_path, sha256, mode, result_name;
    std::uint64_t size;
    bool overwrite;
    FileParams() : size(0), overwrite(false) {}
    bool operator==(const FileParams &b) const;
};
struct FileBegin {
    FileParams file;
    std::string task_id, direction, name;
    std::uint32_t chunk_size;
    FileBegin() : chunk_size(0) {}
};
bool FileUUID(const std::string &, std::string *bytes = NULL);
bool FileName(const std::string &);
bool FileDigest(const std::string &);
bool ParseFileParams(const std::string &, const std::string &, FileParams *, std::string *);
bool ParseFileBegin(const std::string &, std::uint32_t, FileBegin *, std::string *);
std::string FileBeginPayload(const FileBegin &);
std::string FileEndPayload(const FileParams &);
std::string FileAckPayload(std::uint64_t, const std::string &, const std::string &, std::uint64_t);
bool CheckFileAck(const std::string &, std::uint64_t, const std::string &, const std::string &, std::uint64_t,
                  bool *, std::string *);
std::string FileChunkPayload(const std::string &, std::uint64_t, const char *, std::size_t);
bool CheckFileChunk(const std::vector<std::uint8_t> &, const FileParams &, std::uint64_t, std::uint32_t,
                    std::string *);
// Typed accessors throw only within file worker/validated parsing boundaries.
std::string FileString(const JsonObject &, const char *);
std::uint64_t FileUInt(const JsonObject &, const char *);
bool FileBool(const JsonObject &, const char *);
} // namespace rmp
#endif
