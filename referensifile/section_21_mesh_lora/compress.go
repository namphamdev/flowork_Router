// Package lora — compress.go: payload compression untuk LoRa low-bandwidth.
//
// LoRa SF12 ~250bps. 1 KB raw payload = ~32 detik air time. Compression
// 3-5x = 6-10 detik = critical untuk burst sync.
//
// MVP: gzip stdlib (always available, no CGO, ~3-4x ratio buat Indonesian
// text). Future: switch ke zstd dictionary mode (lebih ratio tinggi tapi
// butuh dep external + train custom dict per use case).
//
// Strategy: kalau compressed > raw, kirim raw flag (anti compression overhead
// untuk small payload). Header byte: 0x00 = raw, 0x01 = gzip.

package lora

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
)

// Compression mode flag (1 byte prefix).
const (
	CompressNone byte = 0x00
	CompressGzip byte = 0x01
)

// ErrUnsupportedCompression — payload header byte unknown.
var ErrUnsupportedCompression = errors.New("lora: unsupported compression mode")

// Compress payload. Output prefix dengan 1-byte mode flag. Kalau compressed
// > raw, return raw mode (anti-overhead).
func Compress(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		gz.Close()
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	gzipped := buf.Bytes()

	// Anti-overhead: kalau gzip lebih besar (small payload), kirim raw
	if len(gzipped)+1 >= len(raw)+1 {
		out := make([]byte, 0, len(raw)+1)
		out = append(out, CompressNone)
		out = append(out, raw...)
		return out, nil
	}

	out := make([]byte, 0, len(gzipped)+1)
	out = append(out, CompressGzip)
	out = append(out, gzipped...)
	return out, nil
}

// Decompress parse mode flag + decode payload.
func Decompress(buf []byte) ([]byte, error) {
	if len(buf) == 0 {
		return nil, errors.New("lora: empty compress payload")
	}
	mode := buf[0]
	body := buf[1:]

	switch mode {
	case CompressNone:
		out := make([]byte, len(body))
		copy(out, body)
		return out, nil

	case CompressGzip:
		gz, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip new reader: %w", err)
		}
		defer gz.Close()
		out, err := io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("gzip read: %w", err)
		}
		return out, nil

	default:
		return nil, fmt.Errorf("%w: 0x%02x", ErrUnsupportedCompression, mode)
	}
}

// CompressionRatio — debug helper, return raw_size / compressed_size.
// 1.0 = no benefit, 3.0 = 3x smaller, etc.
func CompressionRatio(raw, compressed int) float64 {
	if compressed == 0 {
		return 0
	}
	return float64(raw) / float64(compressed)
}
