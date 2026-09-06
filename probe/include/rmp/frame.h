#ifndef RMP_FRAME_H
#define RMP_FRAME_H

#include <cstddef>
#include <cstdint>
#include <string>
#include <vector>

namespace rmp {

static const std::size_t kHeaderSize = 20;
static const std::uint8_t kVersion = 1;
static const std::uint32_t kMaxControlPayload = 1024U * 1024U;

static const std::uint8_t kTypeRegister = 0x01;
static const std::uint8_t kTypeRegisterAck = 0x02;
static const std::uint8_t kTypeHeartbeat = 0x03;
static const std::uint8_t kTypeHeartbeatAck = 0x04;
static const std::uint8_t kTypeTask = 0x10;
static const std::uint8_t kTypeTaskAck = 0x11;
static const std::uint8_t kTypeTaskResult = 0x12;
static const std::uint8_t kTypeFileBegin=0x30;
static const std::uint8_t kTypeFileChunk=0x31;
static const std::uint8_t kTypeFileEnd=0x32;
static const std::uint8_t kTypeFileAck=0x33;
static const std::uint8_t kTypeError = 0xFE;
static const std::uint8_t kTypeTunnelConnect = 0x40;
static const std::uint8_t kTypeTunnelClose = 0x41;
static const std::uint8_t kTypeTunnelStatus = 0x42;

static const std::uint16_t kFlagResponse = 1U << 0;
static const std::uint16_t kFlagBinary = 1U << 1;
static const std::uint16_t kFlagMore = 1U << 2;

struct Header {
    std::uint8_t version;
    std::uint8_t type;
    std::uint16_t flags;
    std::uint32_t payload_len;
    std::uint64_t message_id;

    Header()
        : version(kVersion), type(0), flags(0), payload_len(0), message_id(0) {}
};

struct Frame {
    Header header;
    std::vector<std::uint8_t> payload;
};

enum class FrameErrorCode {
    kNone,
    kIncompleteHeader,
    kBadMagic,
    kUnsupportedVersion,
    kPayloadTooLarge
};

const char* FrameErrorName(FrameErrorCode code);

void EncodeHeader(const Header& header, std::uint8_t output[kHeaderSize]);

bool DecodeHeader(const std::uint8_t* data,
                  std::size_t size,
                  std::uint32_t max_payload,
                  Header* header,
                  FrameErrorCode* error);

std::vector<std::uint8_t> EncodeFrame(const Header& header,
                                      const std::vector<std::uint8_t>& payload);

class StreamDecoder {
public:
    explicit StreamDecoder(std::uint32_t max_payload = kMaxControlPayload);

    bool Feed(const std::uint8_t* data,
              std::size_t size,
              std::vector<Frame>* frames,
              FrameErrorCode* error);

    void SetMaxPayload(std::uint32_t max_payload);
    std::size_t buffered() const;

private:
    std::uint32_t max_payload_;
    std::vector<std::uint8_t> buffer_;
    FrameErrorCode failed_;
};

}  // namespace rmp

#endif
