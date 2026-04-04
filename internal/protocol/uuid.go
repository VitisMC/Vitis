package protocol

import (
	"crypto/md5"
	"fmt"
)

// OfflinePlayerUUID generates a UUID v3 (MD5-based) for an offline-mode player.
func OfflinePlayerUUID(username string) UUID {
	h := md5.Sum([]byte("OfflinePlayer:" + username))

	h[6] = (h[6] & 0x0f) | 0x30
	h[8] = (h[8] & 0x3f) | 0x80

	var u UUID
	u[0] = uint64(h[0])<<56 | uint64(h[1])<<48 | uint64(h[2])<<40 | uint64(h[3])<<32 |
		uint64(h[4])<<24 | uint64(h[5])<<16 | uint64(h[6])<<8 | uint64(h[7])
	u[1] = uint64(h[8])<<56 | uint64(h[9])<<48 | uint64(h[10])<<40 | uint64(h[11])<<32 |
		uint64(h[12])<<24 | uint64(h[13])<<16 | uint64(h[14])<<8 | uint64(h[15])
	return u
}

// ParseUUID parses a hyphenated UUID string into a UUID value.
func ParseUUID(s string) (UUID, error) {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return UUID{}, fmt.Errorf("parse uuid: invalid format %q", s)
	}
	hex := s[0:8] + s[9:13] + s[14:18] + s[19:23] + s[24:36]
	if len(hex) != 32 {
		return UUID{}, fmt.Errorf("parse uuid: invalid hex length")
	}
	var b [16]byte
	for i := 0; i < 16; i++ {
		hi, okHi := hexVal(hex[i*2])
		lo, okLo := hexVal(hex[i*2+1])
		if !okHi || !okLo {
			return UUID{}, fmt.Errorf("parse uuid: invalid hex char at position %d", i*2)
		}
		b[i] = hi<<4 | lo
	}
	var u UUID
	u[0] = uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
	u[1] = uint64(b[8])<<56 | uint64(b[9])<<48 | uint64(b[10])<<40 | uint64(b[11])<<32 |
		uint64(b[12])<<24 | uint64(b[13])<<16 | uint64(b[14])<<8 | uint64(b[15])
	return u, nil
}

func hexVal(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	default:
		return 0, false
	}
}

// UUIDToString formats a UUID as a standard hyphenated string.
func UUIDToString(u UUID) string {
	var b [16]byte
	b[0] = byte(u[0] >> 56)
	b[1] = byte(u[0] >> 48)
	b[2] = byte(u[0] >> 40)
	b[3] = byte(u[0] >> 32)
	b[4] = byte(u[0] >> 24)
	b[5] = byte(u[0] >> 16)
	b[6] = byte(u[0] >> 8)
	b[7] = byte(u[0])
	b[8] = byte(u[1] >> 56)
	b[9] = byte(u[1] >> 48)
	b[10] = byte(u[1] >> 40)
	b[11] = byte(u[1] >> 32)
	b[12] = byte(u[1] >> 24)
	b[13] = byte(u[1] >> 16)
	b[14] = byte(u[1] >> 8)
	b[15] = byte(u[1])
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		uint64(b[10])<<40|uint64(b[11])<<32|uint64(b[12])<<24|uint64(b[13])<<16|uint64(b[14])<<8|uint64(b[15]),
	)
}
