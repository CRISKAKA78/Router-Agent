package overlay

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/bits"
	"strings"
)

// MachineID matches EasyTier 2.6.4 parse_or_hash_machine_id: a UUID is
// preserved; other raw identifiers use Rust DefaultHasher (SipHash-1-3).
// Persisted mappings take precedence over this derivation at the call site.
func MachineID(device string) string {
	if device == "" {
		return UUID()
	}
	trimmed := strings.ToLower(strings.TrimSpace(device))
	compact := strings.ReplaceAll(trimmed, "-", "")
	if len(compact) == 32 {
		if b, e := hex.DecodeString(compact); e == nil {
			return formatMachineUUID(b)
		}
	}
	raw := []byte(device)
	var digest [16]byte
	binary.BigEndian.PutUint64(digest[:8], sip13(raw))
	raw = append(raw, digest[:8]...)
	binary.BigEndian.PutUint64(digest[8:], sip13(raw))
	return formatMachineUUID(digest[:])
}
func formatMachineUUID(b []byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func sip13(p []byte) uint64 {
	v0, v1, v2, v3 := uint64(0x736f6d6570736575), uint64(0x646f72616e646f6d), uint64(0x6c7967656e657261), uint64(0x7465646279746573)
	round := func() {
		v0 += v1
		v1 = bits.RotateLeft64(v1, 13)
		v1 ^= v0
		v0 = bits.RotateLeft64(v0, 32)
		v2 += v3
		v3 = bits.RotateLeft64(v3, 16)
		v3 ^= v2
		v0 += v3
		v3 = bits.RotateLeft64(v3, 21)
		v3 ^= v0
		v2 += v1
		v1 = bits.RotateLeft64(v1, 17)
		v1 ^= v2
		v2 = bits.RotateLeft64(v2, 32)
	}
	tail := uint64(len(p)) << 56
	for len(p) >= 8 {
		m := binary.LittleEndian.Uint64(p)
		v3 ^= m
		round()
		v0 ^= m
		p = p[8:]
	}
	for i, b := range p {
		tail |= uint64(b) << uint(8*i)
	}
	v3 ^= tail
	round()
	v0 ^= tail
	v2 ^= 0xff
	round()
	round()
	round()
	return v0 ^ v1 ^ v2 ^ v3
}
