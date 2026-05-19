package aiutils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

// HashString returns a SHA-256 hex digest of the input string.
func HashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// HashBytes returns a SHA-256 hex digest of the input bytes.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// HashReader hashes the contents of a reader.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ContentHash computes a content-addressable hash for deduplication.
func ContentHash(content string) string {
	return HashString(content)
}

// ShortHash returns the first n characters of the hash.
func ShortHash(s string, n int) string {
	h := HashString(s)
	if n >= len(h) {
		return h
	}
	return h[:n]
}

// UUIDv7 generates a UUID v7 (time-ordered) as a string.
// Format: tttttttt-tttt-7xxx-yxxx-xxxxxxxxxxxx
func UUIDv7() string {
	now := time.Now().UnixMilli()
	var buf [16]byte

	// Timestamp (milliseconds since epoch) — 48 bits
	buf[0] = byte(now >> 40)
	buf[1] = byte(now >> 32)
	buf[2] = byte(now >> 24)
	buf[3] = byte(now >> 16)
	buf[4] = byte(now >> 8)
	buf[5] = byte(now)

	// Random bytes for remaining 80 bits
	random := make([]byte, 10)
	rand.Read(random)
	copy(buf[6:], random)

	// Set version (7) in the 4 most significant bits of byte 6
	buf[6] = (buf[6] & 0x0f) | 0x70

	// Set variant (10xx) in the 2 most significant bits of byte 8
	buf[8] = (buf[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
}

var (
	muID      sync.Mutex
	counterID int64
)

// ShortID generates a short unique identifier (8 chars).
func ShortID() string {
	muID.Lock()
	counterID++
	c := counterID
	muID.Unlock()
	now := time.Now().UnixMilli()
	data := fmt.Sprintf("%d-%d", now, c)
	return ShortHash(data, 8)
}

// SessionID generates a human-readable session identifier.
func SessionID() string {
	return "session_" + ShortID()
}

// FastHash returns a non-cryptographic hash for quick comparisons.
// Uses FNV-1a for speed.
func FastHash(s string) uint64 {
	var h uint64 = 14695981039346656037 // FNV offset basis
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211 // FNV prime
	}
	return h
}

// Digest computes a hex-encoded hash for the given parts.
func Digest(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		io.WriteString(h, p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// IsHash checks if a string looks like a hex hash of the given length.
func IsHash(s string, length int) bool {
	if len(s) != length {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
