// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package executors

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

// Varint round-trip across a wide value range.
func TestEncodeDecodeVarint_RoundTrip(t *testing.T) {
	cases := []uint64{0, 1, 127, 128, 255, 300, 16383, 16384, 1 << 20, 1 << 35, ^uint64(0)}
	for _, v := range cases {
		enc := encodeVarint(v)
		got, n, err := decodeVarint(enc, 0)
		if err != nil {
			t.Fatalf("decode err for %d: %v", v, err)
		}
		if got != v || n != len(enc) {
			t.Errorf("roundtrip %d: got=%d n=%d want=%d len=%d", v, got, n, v, len(enc))
		}
	}
}

// Encoding 1, 127, 128 should produce the canonical 1, 1, 2 byte forms.
func TestEncodeVarint_ByteLengths(t *testing.T) {
	cases := map[uint64]int{
		0:     1,
		1:     1,
		127:   1,
		128:   2,
		16383: 2,
		16384: 3,
	}
	for v, want := range cases {
		if got := len(encodeVarint(v)); got != want {
			t.Errorf("encodeVarint(%d) len = %d, want %d", v, got, want)
		}
	}
}

// Decoding a truncated varint must surface an error, not silently succeed.
func TestDecodeVarint_TruncatedFails(t *testing.T) {
	// 0x80 means "more bytes coming" but we cut here — should fail.
	if _, _, err := decodeVarint([]byte{0x80}, 0); err == nil {
		t.Fatal("expected error on truncated varint")
	}
}

func TestEncodeFieldVarint_TagShape(t *testing.T) {
	// Field 5, wireType VARINT (0), value 1 → tag=(5<<3)|0=40 → varint("40")=0x28, then 0x01.
	got := encodeFieldVarint(5, 1)
	want := []byte{0x28, 0x01}
	if !bytes.Equal(got, want) {
		t.Errorf("encodeFieldVarint(5,1) = %x, want %x", got, want)
	}
}

func TestEncodeFieldLen_TagAndLength(t *testing.T) {
	// Field 1, wireType LEN (2), "hi" → tag=(1<<3)|2=10 → varint("10")=0x0a,
	// then len=0x02, then "hi" bytes.
	got := encodeFieldLenString(1, "hi")
	want := []byte{0x0a, 0x02, 'h', 'i'}
	if !bytes.Equal(got, want) {
		t.Errorf("encodeFieldLen(1,\"hi\") = %x, want %x", got, want)
	}
}

func TestEncodeCursorMessage_Roundtrip(t *testing.T) {
	enc := encodeCursorMessage("hello", "user", "msg-0", false)
	// Walk the bytes and confirm field 1 (content) decodes to "hello".
	got := extractTextFromCursorResponse(enc)
	if !strings.Contains(got, "hello") {
		t.Fatalf("content not recoverable, got %q", got)
	}
}

func TestWrapConnectRPCFrame_ShapeAndLength(t *testing.T) {
	payload := []byte("PROTOBUF_BODY")
	frame := wrapConnectRPCFrame(payload)
	if len(frame) != 5+len(payload) {
		t.Fatalf("frame len wrong: %d", len(frame))
	}
	if frame[0] != 0x00 {
		t.Fatalf("flags byte should be 0 for uncompressed, got %x", frame[0])
	}
	if got := binary.BigEndian.Uint32(frame[1:5]); int(got) != len(payload) {
		t.Fatalf("length field = %d, want %d", got, len(payload))
	}
	if !bytes.Equal(frame[5:], payload) {
		t.Fatal("payload not copied verbatim")
	}
}

func TestParseConnectRPCFrame_RoundTrip(t *testing.T) {
	payload := []byte("test-payload-XYZ")
	frame := wrapConnectRPCFrame(payload)
	parsed := parseConnectRPCFrame(frame)
	if parsed == nil {
		t.Fatal("parse returned nil")
	}
	if parsed.Flags != 0x00 || parsed.Length != len(payload) {
		t.Fatalf("frame meta wrong: %+v", parsed)
	}
	if !bytes.Equal(parsed.Payload, payload) {
		t.Fatal("payload mismatch")
	}
	if parsed.Consumed != 5+len(payload) {
		t.Fatalf("consumed = %d, want %d", parsed.Consumed, 5+len(payload))
	}
}

func TestParseConnectRPCFrame_TruncatedReturnsNil(t *testing.T) {
	if parseConnectRPCFrame([]byte{0, 0, 0, 0}) != nil { // <5 bytes
		t.Fatal("4-byte buffer must not parse")
	}
	// 5-byte header advertises 100-byte payload but body is 3 bytes → nil.
	hdr := []byte{0, 0, 0, 0, 100, 'a', 'b', 'c'}
	if parseConnectRPCFrame(hdr) != nil {
		t.Fatal("truncated body must return nil")
	}
}

func TestExtractText_FindsResponseTextInTopLevel(t *testing.T) {
	// Top-level field 1 (cfResponseText) carries "answer". extractText should
	// recognise it as text and surface it without recursing.
	enc := encodeFieldLenString(cfResponseText, "answer")
	if got := extractTextFromCursorResponse(enc); got != "answer" {
		t.Fatalf("extractText = %q, want \"answer\"", got)
	}
}

func TestExtractText_RecursesIntoNested(t *testing.T) {
	// Wrap "ok" inside a nested field (number 99 — clearly NOT cfResponseText)
	// so extractText must recurse to find it.
	inner := encodeFieldLenString(cfResponseText, "ok")
	wrapped := encodeFieldLen(99, inner)
	got := extractTextFromCursorResponse(wrapped)
	if got != "ok" {
		t.Fatalf("nested extract = %q, want \"ok\"", got)
	}
}

func TestEncodeCursorChatRequest_HasMessageAndModel(t *testing.T) {
	body := encodeCursorChatRequest(
		[]CursorMessage{{Content: "hi", Role: "user"}},
		"claude-3.5-sonnet",
	)
	if len(body) == 0 {
		t.Fatal("empty body")
	}
	// Content "hi" should be present somewhere.
	if !bytes.Contains(body, []byte("hi")) {
		t.Errorf("content missing from encoded body: %x", body)
	}
	// Model name should also be present.
	if !bytes.Contains(body, []byte("claude-3.5-sonnet")) {
		t.Errorf("model name missing from encoded body: %x", body)
	}
}
