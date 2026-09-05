#include "rmp/frame.h"

#include <algorithm>
#include <limits>
#include <stdexcept>

namespace rmp {
namespace {

const std::uint8_t kMagic[4] = {'R', 'M', 'P', '1'};

void PutUint16(std::uint8_t* out, std::uint16_t value) {
    out[0] = static_cast<std::uint8_t>((value >> 8) & 0xFFU);
    out[1] = static_cast<std::uint8_t>(value & 0xFFU);
}

void PutUint32(std::uint8_t* out, std::uint32_t value) {
    out[0] = static_cast<std::uint8_t>((value >> 24) & 0xFFU);
    out[1] = static_cast<std::uint8_t>((value >> 16) & 0xFFU);
    out[2] = static_cast<std::uint8_t>((value >> 8) & 0xFFU);
    out[3] = static_cast<std::uint8_t>(value & 0xFFU);
}

void PutUint64(std::uint8_t* out, std::uint64_t value) {
    for (int index = 7; index >= 0; --index) {
        out[index] = static_cast<std::uint8_t>(value & 0xFFU);
        value >>= 8;
    }
}

std::uint16_t GetUint16(const std::uint8_t* in) {
    return static_cast<std::uint16_t>(
        (static_cast<std::uint16_t>(in[0]) << 8) |
        static_cast<std::uint16_t>(in[1]));
}

std::uint32_t GetUint32(const std::uint8_t* in) {
    return (static_cast<std::uint32_t>(in[0]) << 24) |
           (static_cast<std::uint32_t>(in[1]) << 16) |
           (static_cast<std::uint32_t>(in[2]) << 8) |
           static_cast<std::uint32_t>(in[3]);
}

std::uint64_t GetUint64(const std::uint8_t* in) {
    std::uint64_t value = 0;
    for (std::size_t index = 0; index < 8; ++index) {
        value = (value << 8) | static_cast<std::uint64_t>(in[index]);
    }
    return value;
}

}  // namespace

const char* FrameErrorName(FrameErrorCode code) {
    switch (code) {
    case FrameErrorCode::kNone:
        return "NONE";
    case FrameErrorCode::kIncompleteHeader:
        return "INCOMPLETE_HEADER";
    case FrameErrorCode::kBadMagic:
        return "BAD_MAGIC";
    case FrameErrorCode::kUnsupportedVersion:
        return "UNSUPPORTED_VERSION";
    case FrameErrorCode::kPayloadTooLarge:
        return "PAYLOAD_TOO_LARGE";
    }
    return "UNKNOWN";
}

void EncodeHeader(const Header& header, std::uint8_t output[kHeaderSize]) {
    std::copy(kMagic, kMagic + 4, output);
    output[4] = header.version;
    output[5] = header.type;
    PutUint16(output + 6, header.flags);
    PutUint32(output + 8, header.payload_len);
    PutUint64(output + 12, header.message_id);
}

bool DecodeHeader(const std::uint8_t* data,
                  std::size_t size,
                  std::uint32_t max_payload,
                  Header* header,
                  FrameErrorCode* error) {
    if (error != NULL) {
        *error = FrameErrorCode::kNone;
    }
    if (size < kHeaderSize) {
        if (error != NULL) {
            *error = FrameErrorCode::kIncompleteHeader;
        }
        return false;
    }
    if (!std::equal(kMagic, kMagic + 4, data)) {
        if (error != NULL) {
            *error = FrameErrorCode::kBadMagic;
        }
        return false;
    }

    Header decoded;
    decoded.version = data[4];
    decoded.type = data[5];
    decoded.flags = GetUint16(data + 6);
    decoded.payload_len = GetUint32(data + 8);
    decoded.message_id = GetUint64(data + 12);
    if (header != NULL) {
        *header = decoded;
    }
    if (decoded.version != kVersion) {
        if (error != NULL) {
            *error = FrameErrorCode::kUnsupportedVersion;
        }
        return false;
    }
    if (decoded.type == kTypeFileChunk) max_payload=28+512*1024;
    if (decoded.payload_len > max_payload) {
        if (error != NULL) {
            *error = FrameErrorCode::kPayloadTooLarge;
        }
        return false;
    }
    return true;
}

std::vector<std::uint8_t> EncodeFrame(const Header& input_header,
                                      const std::vector<std::uint8_t>& payload) {
    if (payload.size() > std::numeric_limits<std::uint32_t>::max()) {
        throw std::length_error("payload does not fit uint32");
    }
    Header header = input_header;
    header.payload_len = static_cast<std::uint32_t>(payload.size());
    std::vector<std::uint8_t> output(kHeaderSize + payload.size());
    EncodeHeader(header, &output[0]);
    std::copy(payload.begin(), payload.end(), output.begin() + kHeaderSize);
    return output;
}

StreamDecoder::StreamDecoder(std::uint32_t max_payload)
    : max_payload_(max_payload), failed_(FrameErrorCode::kNone) {}

bool StreamDecoder::Feed(const std::uint8_t* data,
                         std::size_t size,
                         std::vector<Frame>* frames,
                         FrameErrorCode* error) {
    if (error != NULL) {
        *error = FrameErrorCode::kNone;
    }
    if (failed_ != FrameErrorCode::kNone) {
        if (error != NULL) {
            *error = failed_;
        }
        return false;
    }
    if (size > 0) {
        buffer_.insert(buffer_.end(), data, data + size);
    }
    while (buffer_.size() >= kHeaderSize) {
        Header header;
        FrameErrorCode decode_error = FrameErrorCode::kNone;
        if (!DecodeHeader(&buffer_[0], kHeaderSize, max_payload_, &header, &decode_error)) {
            failed_ = decode_error;
            if (error != NULL) {
                *error = decode_error;
            }
            return false;
        }
        const std::size_t frame_size = kHeaderSize + static_cast<std::size_t>(header.payload_len);
        if (buffer_.size() < frame_size) {
            break;
        }
        Frame frame;
        frame.header = header;
        frame.payload.assign(buffer_.begin() + kHeaderSize, buffer_.begin() + frame_size);
        frames->push_back(frame);
        buffer_.erase(buffer_.begin(), buffer_.begin() + frame_size);
    }
    return true;
}

std::size_t StreamDecoder::buffered() const {
    return buffer_.size();
}

void StreamDecoder::SetMaxPayload(std::uint32_t max_payload) {
    max_payload_ = max_payload;
}

}  // namespace rmp
