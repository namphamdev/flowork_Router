// Package lora — delta.go: CRDT delta builder + replay untuk M14 LoRa sync.
//
// Workflow:
//
//   Sender (peer A):
//     1. Receive SyncOffer from peer B (peer B's bloom filter + lastHLC)
//     2. Query local promoted packets WHERE hlc > peer.lastHLC
//     3. For each candidate, check peer.bloom — kalau Has(packet_id) skip
//     4. Compress payload (gzip), classify priority (constitution / skill / drawer / etc)
//     5. Push to PriorityQueue → drain via send loop into Frame DELTA_CHUNK
//
//   Receiver (peer B):
//     1. Send SyncOffer (own bloom + lastHLC) ke peer A
//     2. Receive DELTA_CHUNK frames
//     3. Decompress + verify signature (M4 filter L1 still applies)
//     4. INSERT OR IGNORE ke peer_packets (idempotent FQP-4)
//     5. Update lastHLC = max(local, packet.hlc)
//     6. Send ACK frame per received seq#
//
// Append-only invariant maintained: ngga ada DELETE / UPDATE pada peer_packets
// (ANTI_KIAMAT_PROTOCOL Invariant 3). HLC monotonic conflict resolve via
// pubkey hex sort tiebreak (deterministic).

package lora

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

// DeltaChunk — single packet ke kirim via LoRa DELTA_CHUNK frame.
//
// Wire format (deterministic encoding):
//
//	┌─────────┬───────────┬────────────────┬──────────────┐
//	│ HLC     │ ID_LEN    │ PACKET_ID      │ COMPRESSED   │
//	│ 8 byte  │ 1 byte    │ N byte         │ payload      │
//	└─────────┴───────────┴────────────────┴──────────────┘
//
//	HLC      = uint64 BE (hybrid logical clock for ordering)
//	ID_LEN   = uint8 (PACKET_ID len, max 64 byte hex SHA256)
//	PACKET_ID = signed packet ID (untuk dedup di receiver bloom)
//	COMPRESSED = output dari compress.Compress(canonical_signed_packet_bytes)
type DeltaChunk struct {
	HLC      uint64
	PacketID string // hex SHA256 of canonical signed packet
	Payload  []byte // compressed canonical packet bytes
	Priority Priority
}

// Errors.
var (
	ErrChunkTooSmall   = errors.New("lora: delta chunk smaller than minimum header")
	ErrPacketIDOversize = errors.New("lora: packet ID exceeds 64-byte hex max")
)

// MaxPacketIDLen — SHA256 hex = 64 char (32 byte raw uncommon, hex stored sebagai
// canonical in DB).
const MaxPacketIDLen = 64

// EncodeChunk serialize DeltaChunk ke wire bytes (suitable for Frame.Payload).
//
// Total size = 8 (hlc) + 1 (id_len) + len(id) + len(compressed)
// Caller responsible split kalau > MaxPayload (244 byte) — tipikal large packet
// butuh multiple frames dengan same packet_id sebagai reassembly key.
func EncodeChunk(c *DeltaChunk) ([]byte, error) {
	if len(c.PacketID) > MaxPacketIDLen {
		return nil, fmt.Errorf("%w: %d > %d", ErrPacketIDOversize, len(c.PacketID), MaxPacketIDLen)
	}
	buf := make([]byte, 0, 8+1+len(c.PacketID)+len(c.Payload))
	buf = binary.BigEndian.AppendUint64(buf, c.HLC)
	buf = append(buf, byte(len(c.PacketID)))
	buf = append(buf, []byte(c.PacketID)...)
	buf = append(buf, c.Payload...)
	return buf, nil
}

// DecodeChunk parse wire bytes ke DeltaChunk.
func DecodeChunk(buf []byte) (*DeltaChunk, error) {
	if len(buf) < 9 { // 8 hlc + 1 id_len
		return nil, fmt.Errorf("%w: %d < 9", ErrChunkTooSmall, len(buf))
	}
	hlc := binary.BigEndian.Uint64(buf[0:8])
	idLen := int(buf[8])
	if idLen > MaxPacketIDLen {
		return nil, fmt.Errorf("%w: id_len=%d", ErrPacketIDOversize, idLen)
	}
	if len(buf) < 9+idLen {
		return nil, fmt.Errorf("%w: short id (need %d, got %d)", ErrChunkTooSmall, 9+idLen, len(buf))
	}
	id := string(buf[9 : 9+idLen])
	payload := make([]byte, len(buf)-9-idLen)
	copy(payload, buf[9+idLen:])
	return &DeltaChunk{
		HLC:      hlc,
		PacketID: id,
		Payload:  payload,
	}, nil
}

// SyncOfferPayload — wire format untuk SYNC_OFFER frame:
//
//	┌──────────────┬─────────────┐
//	│ LAST_HLC     │ BLOOM_BITS  │
//	│ 8 byte       │ 256 byte    │
//	└──────────────┴─────────────┘
//
// Total: 264 byte = 2 frames LoRa (split, reassemble di receiver via seq#
// continuity). Atau pakai SF7 frame yang max 230+ byte single-frame.
type SyncOfferPayload struct {
	LastHLC uint64
	Bloom   *Bloom
}

// EncodeSyncOffer serialize.
func EncodeSyncOffer(p *SyncOfferPayload) []byte {
	buf := make([]byte, 0, 8+BloomBytes)
	buf = binary.BigEndian.AppendUint64(buf, p.LastHLC)
	buf = append(buf, p.Bloom.Bytes()...)
	return buf
}

// DecodeSyncOffer parse.
func DecodeSyncOffer(buf []byte) (*SyncOfferPayload, error) {
	if len(buf) < 8+BloomBytes {
		return nil, fmt.Errorf("sync_offer: short buf %d < %d", len(buf), 8+BloomBytes)
	}
	hlc := binary.BigEndian.Uint64(buf[0:8])
	bloom := FromBytes(buf[8 : 8+BloomBytes])
	return &SyncOfferPayload{LastHLC: hlc, Bloom: bloom}, nil
}

// PacketSource abstraks DB / store — caller implement supaya delta builder
// tidak terikat ke specific schema.
type PacketSource interface {
	// PromotedPacketsAfter return packet records with hlc > sinceHLC, sorted ASC,
	// limit oleh maxBytes total compressed payload (caller responsible budget).
	PromotedPacketsAfter(sinceHLC uint64, maxBytes int) ([]PacketRecord, error)
}

// PacketRecord — minimal info dari peer_packets row buat build delta.
type PacketRecord struct {
	HLC          uint64
	PacketID     string         // hex SHA256
	CanonicalRaw []byte         // pre-canonicalized signed bytes
	Priority     Priority
}

// BuildDelta scan source untuk packet > sinceHLC yang ngga ada di peer bloom.
// Compress payload + classify priority. Return slice of DeltaChunk siap encode.
//
// budgetBytes = max total raw size (sebelum compress). Stop scanning saat budget
// habis. Caller bisa partial sync (lanjut next round).
func BuildDelta(src PacketSource, peerBloom *Bloom, sinceHLC uint64, budgetBytes int) ([]DeltaChunk, error) {
	if peerBloom == nil {
		peerBloom = NewBloom() // empty bloom = peer punya nothing → kirim semua
	}
	candidates, err := src.PromotedPacketsAfter(sinceHLC, budgetBytes)
	if err != nil {
		return nil, fmt.Errorf("query packets: %w", err)
	}

	var chunks []DeltaChunk
	bytesUsed := 0
	for _, rec := range candidates {
		// Bloom check — skip kalau peer kemungkinan punya
		if peerBloom.Has([]byte(rec.PacketID)) {
			continue
		}
		compressed, err := Compress(rec.CanonicalRaw)
		if err != nil {
			continue
		}
		if bytesUsed+len(compressed) > budgetBytes {
			break
		}
		chunks = append(chunks, DeltaChunk{
			HLC:      rec.HLC,
			PacketID: rec.PacketID,
			Payload:  compressed,
			Priority: rec.Priority,
		})
		bytesUsed += len(compressed)
	}
	return chunks, nil
}

// PacketSink — receiver-side persistence interface.
type PacketSink interface {
	// InsertPacket idempotent insert (INSERT OR IGNORE pattern, dedup via packet_id).
	// Return (inserted bool, err). inserted=false kalau dedup (already had).
	InsertPacket(rec PacketRecord) (bool, error)

	// CurrentLastHLC max(hlc) di local promoted set (untuk SyncOffer reply).
	CurrentLastHLC() (uint64, error)
}

// ReplayDelta receive chunks, decompress, sort by HLC ASC, INSERT OR IGNORE.
//
// Return (inserted_count, skipped_count, err). Skipped = dedup (idempotent).
func ReplayDelta(sink PacketSink, chunks []DeltaChunk) (int, int, error) {
	// Sort by HLC ascending — replay in order
	sortChunksByHLC(chunks)

	inserted, skipped := 0, 0
	for _, c := range chunks {
		raw, err := Decompress(c.Payload)
		if err != nil {
			continue
		}
		ok, err := sink.InsertPacket(PacketRecord{
			HLC:          c.HLC,
			PacketID:     c.PacketID,
			CanonicalRaw: raw,
			Priority:     c.Priority,
		})
		if err != nil {
			return inserted, skipped, fmt.Errorf("insert %s: %w", c.PacketID, err)
		}
		if ok {
			inserted++
		} else {
			skipped++
		}
	}
	return inserted, skipped, nil
}

// BuildLocalBloom scan all promoted packet IDs ke bloom filter.
// Caller pass list of packet IDs (hex SHA256).
func BuildLocalBloom(packetIDs []string) *Bloom {
	b := NewBloom()
	for _, id := range packetIDs {
		b.Add([]byte(id))
	}
	return b
}

// PacketIDFromHash — helper untuk caller compute deterministic packet_id
// dari raw signed packet bytes (SHA256 hex).
func PacketIDFromHash(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

// sortChunksByHLC simple insertion sort (small N typical, < 100 chunks per round).
// Ngga import "sort" buat keep package self-contained.
func sortChunksByHLC(chunks []DeltaChunk) {
	for i := 1; i < len(chunks); i++ {
		for j := i; j > 0 && chunks[j].HLC < chunks[j-1].HLC; j-- {
			chunks[j], chunks[j-1] = chunks[j-1], chunks[j]
		}
	}
}
