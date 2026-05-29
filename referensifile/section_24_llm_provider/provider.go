// Package provider mendefinisikan interface model terpadu dan struktur data yang dibagikan oleh tiap adapter protokol.
package provider

import (
	"context"
	"encoding/json"
)

// Role merepresentasikan tipe role pada unified message model.
type Role string

const (
	// RoleSystem merepresentasikan pesan system prompt.
	RoleSystem Role = "system"
	// RoleUser merepresentasikan pesan input user.
	RoleUser Role = "user"
	// RoleAssistant merepresentasikan pesan assistant yang dihasilkan model.
	RoleAssistant Role = "assistant"
	// RoleTool merepresentasikan pesan hasil eksekusi tool.
	RoleTool Role = "tool"
)

// StopReason merepresentasikan alasan berakhirnya satu respons model.
type StopReason string

const (
	// StopReasonEndTurn menandakan output pada turn ini berakhir secara alami.
	StopReasonEndTurn StopReason = "end_turn"
	// StopReasonToolUse menandakan model meminta eksekusi tool sebelum melanjutkan reasoning.
	StopReasonToolUse StopReason = "tool_use"
)

// MultimodalPart menyimpan data yang lebih kompleks seperti gambar, audio, atau video.
type MultimodalPart struct {
	Type     string `json:"type"`                // "text", "image_url", "audio_url"
	Text     string `json:"text,omitempty"`      // Untuk teks dalam array multimodal
	MIMEType string `json:"mime_type,omitempty"` // Contoh: "image/jpeg", "audio/mp3"
	Data     string `json:"data,omitempty"`      // URL statis atau Base64 terencode
}

// Message adalah struktur message terpadu yang dipakai lintas provider.
type Message struct {
	Role            Role             `json:"role"`
	Content         string           `json:"content,omitempty"`          // Mode klasik teks utuh
	MultimodalParts []MultimodalPart `json:"multimodal_parts,omitempty"` // Mode canggih (A&R / TikTok)
	Thinking        string           `json:"thinking,omitempty"`
	ToolCalls       []ToolCall       `json:"tool_calls,omitempty"`
	ToolCallID      string           `json:"tool_call_id,omitempty"`
	Name            string           `json:"name,omitempty"`
}

// ToolCall mendeskripsikan satu permintaan tool call dari model.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolDefinition mendeskripsikan definisi tool yang diexpose ke model.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// ThinkingParams configures extended thinking for a request.
type ThinkingParams struct {
	Enabled      bool `json:"enabled"`
	BudgetTokens int  `json:"budget_tokens,omitempty"`
}

// Request merepresentasikan unified request yang dikirim ke provider.
type Request struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Thinking    *ThinkingParams  `json:"thinking,omitempty"`
}

// Response merepresentasikan unified response yang dikembalikan provider.
type Response struct {
	Message    Message    `json:"message"`
	StopReason StopReason `json:"stop_reason"`
	Usage      Usage      `json:"usage"`
}

// Usage mencatat penggunaan token untuk request ini.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	ThinkingTokens           int `json:"thinking_tokens,omitempty"`
}

// StreamEventType — category of a streaming event.
type StreamEventType string

const (
	StreamEventTextDelta     StreamEventType = "text_delta"
	StreamEventToolCallStart StreamEventType = "tool_call_start"
	StreamEventToolCallDelta StreamEventType = "tool_call_delta"
	StreamEventToolCallEnd   StreamEventType = "tool_call_end"
	StreamEventMessageStop   StreamEventType = "message_stop"
	StreamEventError         StreamEventType = "error"
)

// StreamEvent — incremental event from streaming completion.
type StreamEvent struct {
	Type        StreamEventType `json:"type"`
	Text        string          `json:"text,omitempty"`
	ToolCall    *ToolCall       `json:"tool_call,omitempty"`
	PartialJSON string          `json:"partial_json,omitempty"`
	StopReason  StopReason      `json:"stop_reason,omitempty"`
	Usage       *Usage          `json:"usage,omitempty"`
	Err         error           `json:"-"`
}

// Client mendefinisikan interface pemanggilan model yang terpadu.
type Client interface {
	// Name mengembalikan identifier tipe provider dari client ini.
	Name() string
	// Complete mengirim satu unified request dan mengembalikan respons yang sudah dinormalkan.
	Complete(ctx context.Context, req Request) (Response, error)
}

// StreamingClient — optional interface for providers supporting streaming.
type StreamingClient interface {
	Client
	// CompleteStream sends a request and streams events via the returned channel.
	// Channel closes when done (success or error). Always check event.Err.
	CompleteStream(ctx context.Context, req Request) (<-chan StreamEvent, error)
}
