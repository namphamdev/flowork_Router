// Cursor ConnectRPC protobuf codec.

package executors

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Protobuf wire types used by Cursor's chat path. FIXED32/FIXED64 exist in
// the spec but the chat schema never uses them, so we omit them.
const (
	wireVarint = 0
	wireLen    = 2
)

// Role enum values in the Cursor schema.
const (
	cursorRoleUser      = 1
	cursorRoleAssistant = 2
)

// UnifiedMode enum values.
const (
	cursorModeChat  = 1
	cursorModeAgent = 2
)

// Field numbers for the messages we actually encode/decode. Names mirror
// the captured Cursor protobuf schema for cross-checking against traffic.
const (
	// StreamUnifiedChatRequestWithTools (top level)
	cfRequest = 1

	// StreamUnifiedChatRequest
	cfMessages      = 1
	cfModel         = 5
	cfCursorSetting = 15
	cfMetadata      = 26
	cfIsAgentic     = 27
	cfUnifiedMode   = 46
	cfUnifiedName   = 54

	// ConversationMessage
	cfMsgContent       = 1
	cfMsgRole          = 2
	cfMsgID            = 13
	cfMsgIsAgentic     = 29
	cfMsgUnifiedMode   = 47

	// StreamUnifiedChatResponse (extractText scan targets)
	cfResponseText = 1
)

// encodeVarint writes a uint64 as a protobuf varint (7 bits per byte, MSB set
// for continuation).
func encodeVarint(value uint64) []byte {
	buf := make([]byte, 0, binary.MaxVarintLen64)
	for value >= 0x80 {
		buf = append(buf, byte(value&0x7F)|0x80)
		value >>= 7
	}
	buf = append(buf, byte(value&0x7F))
	return buf
}

// decodeVarint reads a varint from buf starting at offset. Returns the value
// + total bytes consumed (including the terminator).
func decodeVarint(buf []byte, offset int) (uint64, int, error) {
	var v uint64
	var shift uint
	pos := offset
	for {
		if pos >= len(buf) {
			return 0, 0, errors.New("varint: truncated")
		}
		b := buf[pos]
		pos++
		v |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return v, pos - offset, nil
		}
		shift += 7
		if shift >= 64 {
			return 0, 0, errors.New("varint: overflow")
		}
	}
}

// encodeFieldVarint emits a VARINT field: tag + varint(value).
func encodeFieldVarint(fieldNum int, value uint64) []byte {
	tag := uint64(fieldNum<<3) | wireVarint
	out := encodeVarint(tag)
	return append(out, encodeVarint(value)...)
}

// encodeFieldLen emits a LEN field: tag + varint(len) + data.
// data may be a string, []byte, or nil for empty.
func encodeFieldLen(fieldNum int, data []byte) []byte {
	tag := uint64(fieldNum<<3) | wireLen
	out := encodeVarint(tag)
	out = append(out, encodeVarint(uint64(len(data)))...)
	out = append(out, data...)
	return out
}

// encodeFieldLenString is a convenience wrapper for string data.
func encodeFieldLenString(fieldNum int, s string) []byte {
	return encodeFieldLen(fieldNum, []byte(s))
}

// concatBytes joins multiple byte slices into one.
func concatBytes(parts ...[]byte) []byte {
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// encodeCursorMessage builds the ConversationMessage protobuf for one chat
// message. role = cursorRoleUser / cursorRoleAssistant. hasTools just toggles
// the IsAgentic + UnifiedMode flags — we don't actually serialize tools yet.
func encodeCursorMessage(content, role, messageID string, hasTools bool) []byte {
	roleNum := uint64(cursorRoleUser)
	if role == "assistant" {
		roleNum = cursorRoleAssistant
	}
	mode := uint64(cursorModeChat)
	agentic := uint64(0)
	if hasTools {
		mode = cursorModeAgent
		agentic = 1
	}
	return concatBytes(
		encodeFieldLenString(cfMsgContent, content),
		encodeFieldVarint(cfMsgRole, roleNum),
		encodeFieldLenString(cfMsgID, messageID),
		encodeFieldVarint(cfMsgIsAgentic, agentic),
		encodeFieldVarint(cfMsgUnifiedMode, mode),
	)
}

// encodeCursorChatRequest builds the StreamUnifiedChatRequest body for a list
// of (content, role) pairs. Each message gets a stable id of the form
// "msg-<index>" — Cursor accepts arbitrary opaque ids.
func encodeCursorChatRequest(messages []CursorMessage, modelName string) []byte {
	parts := make([][]byte, 0, len(messages)+4)
	for i, m := range messages {
		mid := fmt.Sprintf("msg-%d", i)
		parts = append(parts, encodeFieldLen(cfMessages, encodeCursorMessage(m.Content, m.Role, mid, false)))
	}

	// Model sub-message: just a name field.
	model := encodeFieldLenString(1, modelName) // sub-field 1 = MODEL_NAME inside the Model message
	parts = append(parts, encodeFieldLen(cfModel, model))

	// UnifiedMode = CHAT (1).
	parts = append(parts, encodeFieldVarint(cfUnifiedMode, cursorModeChat))
	parts = append(parts, encodeFieldLenString(cfUnifiedName, "chat"))
	parts = append(parts, encodeFieldVarint(cfIsAgentic, 0))

	return concatBytes(parts...)
}

// CursorMessage is the minimum chat-message shape this codec needs.
type CursorMessage struct {
	Content string
	Role    string // "user" | "assistant" | "system" (system is folded into user content by the caller)
}

// wrapConnectRPCFrame prepends the 5-byte ConnectRPC header (compression flag
// + 4-byte big-endian length). Cursor's chat path does not support compressed
// requests so the flag is always 0x00.
func wrapConnectRPCFrame(payload []byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = 0x00 // flags: no compression
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

// connectFrame is the result of parsing one ConnectRPC frame.
type connectFrame struct {
	Flags    byte
	Length   int
	Payload  []byte
	Consumed int
}

// parseConnectRPCFrame reads one frame from buf. Returns nil when buf is
// truncated. Compressed frames (flags & 0x01) are returned with the raw
// gzipped payload — callers can decompress out-of-band if needed.
func parseConnectRPCFrame(buf []byte) *connectFrame {
	if len(buf) < 5 {
		return nil
	}
	length := int(binary.BigEndian.Uint32(buf[1:5]))
	if len(buf) < 5+length {
		return nil
	}
	payload := make([]byte, length)
	copy(payload, buf[5:5+length])
	return &connectFrame{
		Flags:    buf[0],
		Length:   length,
		Payload:  payload,
		Consumed: 5 + length,
	}
}

// extractTextFromCursorResponse walks the decoded protobuf payload looking
// for response-text fields. The Cursor schema places the assistant's reply in
// field 1 (LEN-wire) of the top-level response message. Nested messages are
// recursed so MCP-wrapped text is also surfaced.
func extractTextFromCursorResponse(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	out := ""
	offset := 0
	for offset < len(payload) {
		tag, n, err := decodeVarint(payload, offset)
		if err != nil {
			return out
		}
		offset += n
		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		switch wireType {
		case wireVarint:
			_, n, err := decodeVarint(payload, offset)
			if err != nil {
				return out
			}
			offset += n
		case wireLen:
			length, n, err := decodeVarint(payload, offset)
			if err != nil {
				return out
			}
			offset += n
			if offset+int(length) > len(payload) {
				return out
			}
			data := payload[offset : offset+int(length)]
			offset += int(length)
			if fieldNum == cfResponseText && looksLikeUTF8(data) {
				out += string(data)
			} else {
				// Recurse — many response payloads wrap text in nested messages.
				if nested := extractTextFromCursorResponse(data); nested != "" {
					out += nested
				}
			}
		default:
			// Unknown wire type — bail out rather than guess.
			return out
		}
	}
	return out
}

// looksLikeUTF8 is a cheap heuristic: payload bytes that are mostly printable
// or whitespace are treated as text; anything else is treated as a nested
// protobuf message.
func looksLikeUTF8(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	printable := 0
	for _, c := range b {
		if c == 0x09 || c == 0x0A || c == 0x0D || (c >= 0x20 && c < 0x7F) || c >= 0xC0 {
			printable++
		}
	}
	return printable*100/len(b) >= 80
}
