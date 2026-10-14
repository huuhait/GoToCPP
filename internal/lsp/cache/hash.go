package cache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
)

// A Hash is the SHA-256 hash of the inputs of a computation: the content
// of a file, or everything a parse, a load or a check depends on. The
// memoized pipeline keys its caches with it.
type Hash [sha256.Size]byte

// String returns the first 16 hex digits, for messages and tests.
func (h Hash) String() string { return hex.EncodeToString(h[:8]) }

// hashOf returns the hash of data.
func hashOf(data []byte) Hash { return sha256.Sum256(data) }

// A hasher builds a Hash from parts. Each part is written with its length
// first, so that two different lists of parts never give the same bytes.
type hasher struct{ h hash.Hash }

func newHasher(kind string) *hasher {
	h := &hasher{sha256.New()}
	h.str(kind)
	return h
}

func (h *hasher) str(s string) {
	h.int(int64(len(s)))
	h.h.Write([]byte(s))
}

func (h *hasher) int(n int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(n))
	h.h.Write(b[:])
}

func (h *hasher) bool(b bool) {
	if b {
		h.int(1)
	} else {
		h.int(0)
	}
}

func (h *hasher) hash(x Hash) { h.h.Write(x[:]) }

func (h *hasher) sum() Hash {
	var x Hash
	h.h.Sum(x[:0])
	return x
}
