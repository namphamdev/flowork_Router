// Package lora — frame.go: LoRa packet frame format untuk M14 delta protocol.
//
// SX1276 SF12 max payload effective ~50 byte (lower SF lebih besar tapi range
// shorter). Frame ini design untuk fit di 256-byte limit module umum, dengan
// header overhead 12 byte = max payload 244 byte.
//
// Layout (big-endian):
//
//	┌─────────┬─────────┬─────────┬─────────┬──────────┬──────────┐
//	│ MAGIC   │ VER     │ TYPE    │ SEQ#    │ PAYLOAD  │ CRC32    │
//	│ 2 byte  │ 1 byte  │ 1 byte  │ 4 byte  │ N byte   │ 4 byte   │
//	└─────────┴─────────┴─────────┴─────────┴──────────┴──────────┘
//
//	MAGIC = 0xF1 0x0F (Flowork)
//	VER   = 0x01
//	TYPE  = 1 byte enum (sync_offer / sync_request / delta_chunk / ack / nack / ping)
//	SEQ#  = uint32 monotonic per session (used for ACK chain + dedup)
//	CRC32 = IEEE polynomial over header + payload (anti corruption mid-air)
//
// Total max frame: 12 + 244 = 256 byte, fit semua LoRa module SX12xx family.

package lora

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

// Magic bytes — 0xF10F (Flowork). Identify Flowork frame vs random noise.
const (
	MagicMSB byte = 0xF1
	MagicLSB byte = 0x0F

	ProtocolVersion byte = 0x01

	HeaderLen  = 12  // magic(2) + ver(1) + type(1) + seq(4) + crc(4)
	MaxPayload = 244 // 256 - HeaderLen
	MaxFrame   = HeaderLen + MaxPayload
)

// Frame type byte enum.
const (
	TypeSyncOffer   byte = 0x01 // peer A advertise bloom filter + lastHLC
	TypeSyncRequest byte = 0x02 // peer B request specific delta range
	TypeDeltaChunk  byte = 0x03 // actual data payload
	TypeAck         byte = 0x04 // confirm seq# received
	TypeNack        byte = 0x05 // CRC fail / unable parse, request retry
	TypePing        byte = 0x06 // heartbeat, keep session alive
	TypePong        byte = 0x07 // ping response
)

// Errors.
var (
	ErrShortFrame         = errors.New("lora: frame shorter than header")
	ErrBadMagic           = errors.New("lora: magic bytes mismatch (not Flowork frame)")
	ErrUnsupportedVersion = errors.New("lora: unsupported protocol version")
	ErrPayloadTooLarge    = errors.New("lora: payload exceeds MaxPayload")
	ErrCRCMismatch        = errors.New("lora: CRC32 mismatch (frame corrupted in transit)")
)

// Frame represents a single LoRa packet frame.
type Frame struct {
	Type    byte
	Seq     uint32
	Payload []byte
}

// Encode serialize frame ke wire format. Returns ErrPayloadTooLarge kalau
// payload > MaxPayload (caller harus split sebelum encode).
func (f *Frame) Encode() ([]byte, error) {
	if len(f.Payload) > MaxPayload {
		return nil, fmt.Errorf("%w: %d > %d", ErrPayloadTooLarge, len(f.Payload), MaxPayload)
	}
	buf := make([]byte, 0, HeaderLen+len(f.Payload))
	buf = append(buf, MagicMSB, MagicLSB, ProtocolVersion, f.Type)
	buf = binary.BigEndian.AppendUint32(buf, f.Seq)
	buf = append(buf, f.Payload...)

	// CRC32 IEEE over everything before crc field
	crc := crc32.ChecksumIEEE(buf)
	buf = binary.BigEndian.AppendUint32(buf, crc)
	return buf, nil
}

// DecodeFrame parse wire bytes ke Frame. Validates magic, version, CRC.
//
// Errors:
//   - ErrShortFrame: buf < HeaderLen
//   - ErrBadMagic: not Flowork frame
//   - ErrUnsupportedVersion: protocol version != 0x01
//   - ErrCRCMismatch: payload corrupted
func DecodeFrame(buf []byte) (*Frame, error) {
	if len(buf) < HeaderLen {
		return nil, fmt.Errorf("%w: %d < %d", ErrShortFrame, len(buf), HeaderLen)
	}
	if buf[0] != MagicMSB || buf[1] != MagicLSB {
		return nil, ErrBadMagic
	}
	if buf[2] != ProtocolVersion {
		return nil, fmt.Errorf("%w: got 0x%02x want 0x%02x", ErrUnsupportedVersion, buf[2], ProtocolVersion)
	}

	payloadEnd := len(buf) - 4
	expectedCRC := binary.BigEndian.Uint32(buf[payloadEnd:])
	actualCRC := crc32.ChecksumIEEE(buf[:payloadEnd])
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("%w: expected=0x%08x actual=0x%08x", ErrCRCMismatch, expectedCRC, actualCRC)
	}

	return &Frame{
		Type:    buf[3],
		Seq:     binary.BigEndian.Uint32(buf[4:8]),
		Payload: buf[8:payloadEnd],
	}, nil
}

// TypeName human-readable nama dari type byte (untuk log/debug).
func TypeName(t byte) string {
	switch t {
	case TypeSyncOffer:
		return "SYNC_OFFER"
	case TypeSyncRequest:
		return "SYNC_REQUEST"
	case TypeDeltaChunk:
		return "DELTA_CHUNK"
	case TypeAck:
		return "ACK"
	case TypeNack:
		return "NACK"
	case TypePing:
		return "PING"
	case TypePong:
		return "PONG"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02x)", t)
	}
}
