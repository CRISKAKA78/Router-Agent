package protocol

import (
	"encoding/binary"
	"errors"
	"testing"
)

func TestHeaderEncodeDecodeBigEndian(t *testing.T) {
	header := Header{
		Version:    Version1,
		Type:       TypeHeartbeat,
		Flags:      0x0102,
		PayloadLen: 0x03040506,
		MessageID:  0x0708090A0B0C0D0E,
	}
	encoded := EncodeHeader(header)
	if got := string(encoded[0:4]); got != "RMP1" {
		t.Fatalf("magic = %q", got)
	}
	if got := binary.BigEndian.Uint16(encoded[6:8]); got != header.Flags {
		t.Fatalf("flags = %#x", got)
	}
	if got := binary.BigEndian.Uint32(encoded[8:12]); got != header.PayloadLen {
		t.Fatalf("payload_len = %#x", got)
	}
	if got := binary.BigEndian.Uint64(encoded[12:20]); got != header.MessageID {
		t.Fatalf("message_id = %#x", got)
	}
	decoded, err := DecodeHeader(encoded[:], ^uint32(0))
	if err != nil {
		t.Fatal(err)
	}
	if decoded != header {
		t.Fatalf("decoded = %#v, want %#v", decoded, header)
	}
}

func testFrame(t *testing.T, messageID uint64, payload string) []byte {
	t.Helper()
	encoded, err := EncodeFrame(Frame{
		Header:  Header{Version: Version1, Type: TypeHeartbeat, MessageID: messageID},
		Payload: []byte(payload),
	})
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestDecoderHalfHeader(t *testing.T) {
	encoded := testFrame(t, 1, `{}`)
	decoder := NewDecoder(MaxControlPayload)
	frames, err := decoder.Feed(encoded[:10])
	if err != nil || len(frames) != 0 {
		t.Fatalf("first feed frames=%d err=%v", len(frames), err)
	}
	frames, err = decoder.Feed(encoded[10:])
	if err != nil || len(frames) != 1 {
		t.Fatalf("second feed frames=%d err=%v", len(frames), err)
	}
}

func TestDecoderHalfPayload(t *testing.T) {
	encoded := testFrame(t, 1, `{"uptime":1,"uptime_valid":true,"running_tasks":0}`)
	split := HeaderSize + 5
	decoder := NewDecoder(MaxControlPayload)
	frames, err := decoder.Feed(encoded[:split])
	if err != nil || len(frames) != 0 {
		t.Fatalf("first feed frames=%d err=%v", len(frames), err)
	}
	frames, err = decoder.Feed(encoded[split:])
	if err != nil || len(frames) != 1 {
		t.Fatalf("second feed frames=%d err=%v", len(frames), err)
	}
}

func TestDecoderMultipleFramesInOneRead(t *testing.T) {
	first := testFrame(t, 1, `{}`)
	second := testFrame(t, 2, `{"uptime":2,"uptime_valid":true,"running_tasks":0}`)
	decoder := NewDecoder(MaxControlPayload)
	frames, err := decoder.Feed(append(first, second...))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 || frames[0].Header.MessageID != 1 || frames[1].Header.MessageID != 2 {
		t.Fatalf("unexpected frames: %#v", frames)
	}
}

func TestDecoderHeaderErrors(t *testing.T) {
	tests := []struct {
		name string
		edit func([]byte)
		code string
	}{
		{"bad magic", func(data []byte) { data[0] = 'X' }, "BAD_MAGIC"},
		{"unsupported version", func(data []byte) { data[4] = 2 }, "UNSUPPORTED_VERSION"},
		{"payload too large", func(data []byte) { binary.BigEndian.PutUint32(data[8:12], 1025) }, "PAYLOAD_TOO_LARGE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := testFrame(t, 1, `{}`)
			tc.edit(data)
			_, err := NewDecoder(1024).Feed(data[:HeaderSize])
			var frameErr *FrameError
			if !errors.As(err, &frameErr) || frameErr.Code != tc.code {
				t.Fatalf("err=%v, want code %s", err, tc.code)
			}
		})
	}
}
