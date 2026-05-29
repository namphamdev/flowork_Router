package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/teetah2402/flowork/internal/finance"
)

// CompleteStream — SSE streaming for OpenAI-compatible endpoints.
// Supports: OpenAI, DeepSeek, xAI (Grok), any OpenAI-compat provider.
func (c *OpenAIClient) CompleteStream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = c.defaultMod
	}

	// rc-honest-audit 2026-04-21: BudgetGuard pre-call untuk streaming path.
	// Sebelumnya stream by-pass guard — itu bocor kedua kenapa 226% over cap.
	var reservedUSD float64
	var guardActive bool
	if budgetGuardActive() && isOpenRouterURL(c.baseURL) {
		reservedUSD = estimateCostUSD(req)
		agent := strings.TrimSpace(os.Getenv("FLOWORK_AGENT_HANDLE"))
		if err := finance.Shared().CheckBudgetFor(ctx, model, agent, reservedUSD); err != nil {
			if errors.Is(err, finance.ErrBudgetExceeded) {
				return nil, err
			}
			fmt.Fprintf(os.Stderr, "provider/stream: budget poll warning: %v\n", err)
		} else {
			guardActive = true
		}
	}
	// Closure helper: release reservation kalau stream gagal establish.
	releaseOnFail := func() {
		if guardActive {
			finance.Shared().ReleaseReservation(reservedUSD)
		}
	}

	payload := map[string]any{
		"model":       model,
		"messages":    toOpenAIMessages(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
		"stream_options": map[string]any{
			"include_usage": true,
		},
	}
	if len(req.Tools) > 0 {
		payload["tools"] = toOpenAITools(req.Tools)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		releaseOnFail()
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		releaseOnFail()
		return nil, fmt.Errorf("request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := streamHTTPClient().Do(httpReq)
	if err != nil {
		releaseOnFail()
		return nil, fmt.Errorf("do: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Capture body untuk error msg + retry decision
		bs := make([]byte, 2048)
		n, _ := resp.Body.Read(bs)
		resp.Body.Close()
		errMsg := fmt.Sprintf("provider %s: %s", resp.Status, string(bs[:n]))

		// rc-claude-rescue 2026-04-20: HTTP 402 / insufficient_credits — auto
		// fallback ke FREE_AGENT_MODEL (sama pattern dengan non-stream path).
		if resp.StatusCode == 402 || isInsufficientCreditsError(errors.New(errMsg)) {
			freeModel := safeFreeModel()
			if isOpenRouterURL(c.baseURL) && freeModel != "" &&
				!strings.Contains(strings.ToLower(model), ":free") &&
				!strings.EqualFold(freeModel, model) {
				fmt.Fprintf(os.Stderr, "provider/stream: HTTP 402/credits exhausted untuk %s, retry stream dengan free model %s\n", model, freeModel)
				model = freeModel
				payload["model"] = freeModel
				body2, _ := json.Marshal(payload)
				httpReq2, herr := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body2))
				if herr != nil {
					releaseOnFail()
					return nil, fmt.Errorf("free fallback request: %w", herr)
				}
				httpReq2.Header.Set("Content-Type", "application/json")
				httpReq2.Header.Set("Authorization", "Bearer "+c.apiKey)
				httpReq2.Header.Set("Accept", "text/event-stream")
				resp, err = streamHTTPClient().Do(httpReq2)
				if err != nil {
					releaseOnFail()
					return nil, fmt.Errorf("free fallback do: %w", err)
				}
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					defer resp.Body.Close()
					releaseOnFail()
					bs2 := make([]byte, 2048)
					m, _ := resp.Body.Read(bs2)
					return nil, fmt.Errorf("free fallback %s: %s", resp.Status, string(bs2[:m]))
				}
				// Free retry sukses — fall-through ke streaming goroutine di bawah.
			} else {
				releaseOnFail()
				return nil, errors.New(errMsg)
			}
		} else {
			releaseOnFail()
			return nil, errors.New(errMsg)
		}
	}

	events := make(chan StreamEvent, 16)
	go func() {
		var finalUsage *Usage
		defer func() {
			if r := recover(); r != nil {
				events <- StreamEvent{Type: StreamEventError, Err: fmt.Errorf("stream panic: %v", r)}
			}
			// Settle reservation setelah stream selesai (sukses atau panic).
			// Kalau tidak ada final usage frame, record estimasi (upper bound
			// konservatif) biar reservasi tidak leak.
			if guardActive {
				if finalUsage != nil {
					actual := actualCostUSD(model, finalUsage.InputTokens, finalUsage.OutputTokens, finalUsage.CacheReadInputTokens)
					finance.Shared().Record(actual)
				} else {
					finance.Shared().Record(reservedUSD)
				}
			}
		}()
		defer resp.Body.Close()
		defer close(events)

		// Wrap parseSSEStream supaya final usage ditangkap untuk Record.
		// Fix bug-6: panic di parseSSEStream (misal OpenRouter kirim JSON
		// shape aneh, slice OOB) akan crash seluruh program karena panic di
		// child goroutine NOT propagate ke outer defer recover — goroutine
		// terpisah punya stack sendiri. Emit StreamEventError lalu close
		// channel supaya caller dapat sinyal kegagalan, bukan crash.
		captured := make(chan StreamEvent, 16)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Best-effort: kirim error event. Channel sudah buffered
					// 16 jadi non-blocking selama downstream tidak mati.
					select {
					case captured <- StreamEvent{Type: StreamEventError, Err: fmt.Errorf("parseSSEStream panic: %v", r)}:
					default:
					}
				}
				close(captured)
			}()
			parseSSEStream(resp.Body, captured)
		}()
		for ev := range captured {
			if ev.Usage != nil {
				finalUsage = ev.Usage
			}
			events <- ev
		}
	}()
	return events, nil
}

// SSE delta payload shape (OpenAI-compatible)
type ssePayload struct {
	Choices []struct {
		Delta struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

func parseSSEStream(body io.Reader, out chan<- StreamEvent) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	// Track tool_call accumulation (indexed by tool_call index in delta)
	toolCallState := make(map[int]*ToolCall)
	toolCallArgs := make(map[int]*strings.Builder)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var payload ssePayload
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}
		// Usage frame (no choices, usage-only)
		if len(payload.Choices) == 0 && payload.Usage != nil {
			out <- StreamEvent{
				Type: StreamEventMessageStop,
				Usage: &Usage{
					InputTokens:  payload.Usage.PromptTokens,
					OutputTokens: payload.Usage.CompletionTokens,
				},
			}
			continue
		}
		if len(payload.Choices) == 0 {
			continue
		}
		ch := payload.Choices[0]

		// Text content delta
		if ch.Delta.Content != "" {
			out <- StreamEvent{Type: StreamEventTextDelta, Text: ch.Delta.Content}
		}

		// Tool call deltas
		for _, tc := range ch.Delta.ToolCalls {
			idx := tc.Index
			state, ok := toolCallState[idx]
			if !ok {
				state = &ToolCall{ID: tc.ID, Name: tc.Function.Name}
				toolCallState[idx] = state
				toolCallArgs[idx] = &strings.Builder{}
				out <- StreamEvent{
					Type:     StreamEventToolCallStart,
					ToolCall: state,
				}
			}
			if tc.Function.Arguments != "" {
				toolCallArgs[idx].WriteString(tc.Function.Arguments)
				out <- StreamEvent{
					Type:        StreamEventToolCallDelta,
					PartialJSON: tc.Function.Arguments,
					ToolCall:    state,
				}
			}
			// If ID or name arrive late
			if state.ID == "" && tc.ID != "" {
				state.ID = tc.ID
			}
			if state.Name == "" && tc.Function.Name != "" {
				state.Name = tc.Function.Name
			}
		}

		if ch.FinishReason != "" {
			// Finalize tool calls in deterministic index order (Go maps iterate randomly).
			indices := make([]int, 0, len(toolCallState))
			for idx := range toolCallState {
				indices = append(indices, idx)
			}
			sort.Ints(indices)
			for _, idx := range indices {
				state := toolCallState[idx]
				state.Arguments = json.RawMessage(toolCallArgs[idx].String())
				out <- StreamEvent{Type: StreamEventToolCallEnd, ToolCall: state}
			}
			stopReason := StopReasonEndTurn
			if ch.FinishReason == "tool_calls" {
				stopReason = StopReasonToolUse
			}
			out <- StreamEvent{Type: StreamEventMessageStop, StopReason: stopReason}
		}
	}
	if err := scanner.Err(); err != nil {
		out <- StreamEvent{Type: StreamEventError, Err: err}
	}
}
