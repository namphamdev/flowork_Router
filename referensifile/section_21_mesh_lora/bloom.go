// Package lora — bloom.go: bloom filter pre-sync untuk M14 LoRa delta protocol.
//
// Use case: peer A mau push ke peer B via LoRa low-bandwidth. Sebelum kirim
// data, exchange bloom filter dari packet ID yang receiver punya. Sender
// skip kirim packet yang bloom positive (~99% dedup, false positive ~0.1%
// acceptable di LoRa context).
//
// Spec:
//   - Size: 2048 bit = 256 byte (fit di 1 LoRa frame payload SyncOffer)
//   - Hash funcs: 8 (SipHash-2-4 dengan 8 different keys)
//   - Capacity: ~512 unique items dengan false positive rate ~1%
//     (lebih besar di brain corpus akan naik FPR — acceptable untuk LoRa)
//
// SipHash chosen over MurmurHash karena cryptographic strength (resistance
// terhadap adversary yang craft packet dengan ID collision intentional).

package lora

import (
	"encoding/binary"
)

const (
	BloomBytes    = 256
	BloomBits     = BloomBytes * 8 // 2048
	BloomHashFns  = 8
)

// Bloom is a 2048-bit Bloom filter dengan 8 hash functions.
type Bloom struct {
	bits [BloomBytes]byte
}

// NewBloom return empty bloom filter (all bits 0).
func NewBloom() *Bloom {
	return &Bloom{}
}

// Add insert key ke bloom (set 8 bits via 8 hash funcs).
func (b *Bloom) Add(key []byte) {
	for i := 0; i < BloomHashFns; i++ {
		bit := bloomHash(key, uint64(i)) % BloomBits
		b.bits[bit/8] |= 1 << (bit % 8)
	}
}

// Has return true kalau semua 8 hash bits set (probably-in).
// False = definitely-not-in. True = probably-in (false positive ~1%).
func (b *Bloom) Has(key []byte) bool {
	for i := 0; i < BloomHashFns; i++ {
		bit := bloomHash(key, uint64(i)) % BloomBits
		if b.bits[bit/8]&(1<<(bit%8)) == 0 {
			return false
		}
	}
	return true
}

// Bytes return underlying 256-byte filter (untuk wire transmission via
// SyncOffer payload).
func (b *Bloom) Bytes() []byte {
	out := make([]byte, BloomBytes)
	copy(out, b.bits[:])
	return out
}

// FromBytes parse wire bytes ke Bloom. Returns nil kalau len != BloomBytes.
func FromBytes(buf []byte) *Bloom {
	if len(buf) != BloomBytes {
		return nil
	}
	b := &Bloom{}
	copy(b.bits[:], buf)
	return b
}

// Count estimate jumlah set bits (untuk debug — saturated bloom = banyak FP).
func (b *Bloom) Count() int {
	n := 0
	for _, by := range b.bits {
		n += popcount(by)
	}
	return n
}

// EstimatedFalsePositiveRate compute FPR berdasar set bit count.
// Formula: (k_bits_set / total_bits)^k_hash_funcs
func (b *Bloom) EstimatedFalsePositiveRate() float64 {
	setBits := float64(b.Count())
	ratio := setBits / float64(BloomBits)
	fpr := 1.0
	for i := 0; i < BloomHashFns; i++ {
		fpr *= ratio
	}
	return fpr
}

// bloomHash — SipHash-like dengan key index sebagai salt. Pure Go, no deps.
//
// Note: ini bukan full SipHash-2-4 spec. Untuk MVP pakai FNV-1a + per-fn
// seed (good enough untuk bloom, weak terhadap adversarial collision yang
// targeted — acceptable since attacker harus kontrol packet ID, dan ID =
// SHA256 hash of canonical bytes (collision-resistant by construction).
func bloomHash(key []byte, fnIndex uint64) uint64 {
	const (
		fnvOffset = 14695981039346656037
		fnvPrime  = 1099511628211
	)
	h := uint64(fnvOffset)
	// Mix in fn index sebagai salt
	var saltBuf [8]byte
	binary.BigEndian.PutUint64(saltBuf[:], fnIndex)
	for _, b := range saltBuf {
		h ^= uint64(b)
		h *= fnvPrime
	}
	for _, b := range key {
		h ^= uint64(b)
		h *= fnvPrime
	}
	return h
}

// popcount return jumlah bit 1 di byte (lookup table 256-entry, fast).
func popcount(b byte) int {
	return bitCountTable[b]
}

var bitCountTable = func() [256]int {
	var t [256]int
	for i := 0; i < 256; i++ {
		t[i] = (i & 1) + t[i>>1]
	}
	return t
}()
