package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	HeaderSize       = 20
	Version1   uint8 = 1

	TypeRegister     uint8 = 0x01
	TypeRegisterAck  uint8 = 0x02
	TypeHeartbeat    uint8 = 0x03
	TypeHeartbeatAck uint8 = 0x04
	TypeTask         uint8 = 0x10
	TypeTaskAck      uint8 = 0x11
	TypeTaskResult   uint8 = 0x12
	TypeFileBegin    uint8 = 0x30
	TypeFileChunk    uint8 = 0x31
	TypeFileEnd      uint8 = 0x32
	TypeFileAck      uint8 = 0x33
	TypeError        uint8 = 0xFE

	FlagResponse uint16 = 1 << 0
	FlagBinary   uint16 = 1 << 1
	FlagMore     uint16 = 1 << 2

	MaxControlPayload uint32 = 1024 * 1024
)

var magic = [4]byte{'R', 'M', 'P', '1'}

type Header struct {
	Version    uint8
	Type       uint8
	Flags      uint16
	PayloadLen uint32
	MessageID  uint64
}

type Frame struct {
	Header  Header
	Payload []byte
}

type FrameError struct {
	Code   string
	Header *Header
	Detail string
}

func (e *FrameError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

func EncodeHeader(h Header) [HeaderSize]byte {
	var out [HeaderSize]byte
	copy(out[0:4], magic[:])
	out[4] = h.Version
	out[5] = h.Type
	binary.BigEndian.PutUint16(out[6:8], h.Flags)
	binary.BigEndian.PutUint32(out[8:12], h.PayloadLen)
	binary.BigEndian.PutUint64(out[12:20], h.MessageID)
	return out
}

func DecodeHeader(data []byte, maxPayload uint32) (Header, error) {
	if len(data) < HeaderSize {
		return Header{}, io.ErrUnexpectedEOF
	}
	h := Header{
		Version:    data[4],
		Type:       data[5],
		Flags:      binary.BigEndian.Uint16(data[6:8]),
		PayloadLen: binary.BigEndian.Uint32(data[8:12]),
		MessageID:  binary.BigEndian.Uint64(data[12:20]),
	}
	if string(data[0:4]) != string(magic[:]) {
		return Header{}, &FrameError{Code: "BAD_MAGIC", Detail: "magic must be RMP1"}
	}
	if h.Version != Version1 {
		return h, &FrameError{Code: "UNSUPPORTED_VERSION", Header: &h, Detail: fmt.Sprintf("version %d", h.Version)}
	}
	if maxPayload == 0 {
		maxPayload = MaxControlPayload
	}
	if h.Type == TypeFileChunk {
		maxPayload = 28 + 512*1024
	}
	if h.PayloadLen > maxPayload {
		return h, &FrameError{Code: "PAYLOAD_TOO_LARGE", Header: &h, Detail: fmt.Sprintf("payload_len %d exceeds %d", h.PayloadLen, maxPayload)}
	}
	return h, nil
}

func EncodeFrame(frame Frame) ([]byte, error) {
	if uint64(len(frame.Payload)) > uint64(^uint32(0)) {
		return nil, errors.New("payload is too large for uint32")
	}
	frame.Header.PayloadLen = uint32(len(frame.Payload))
	header := EncodeHeader(frame.Header)
	out := make([]byte, HeaderSize+len(frame.Payload))
	copy(out[:HeaderSize], header[:])
	copy(out[HeaderSize:], frame.Payload)
	return out, nil
}

func WriteFrame(w io.Writer, frame Frame) error {
	encoded, err := EncodeFrame(frame)
	if err != nil {
		return err
	}
	for len(encoded) > 0 {
		n, writeErr := w.Write(encoded)
		if writeErr != nil {
			return writeErr
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		encoded = encoded[n:]
	}
	return nil
}
