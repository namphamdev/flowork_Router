// Executor Base Helpers.

package executors

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/flowork-os/flowork_Router/internal/store"
)

// httpClient is shared by every executor. Mirrors the timeout profile used by
// the router's outbound calls.
var httpClient = &http.Client{Timeout: 300 * time.Second}

// streamingPartialWrite is the sentinel status the dispatcher treats as
// "headers/body already started — no retry possible". Kept in sync with the
// equivalent constant in internal/router/dispatcher_stream.go.
const StreamingPartialWrite = -1

// dataLine is the SSE wire prefix for OpenAI-style delta lines.
const dataLine = "data: "

// BuildRequest constructs an outbound HTTP request shared by both Stream and
// NonStream paths. apiKey, additional headers and URL come from the caller.
func BuildRequest(ctx context.Context, method, url string, body []byte, headers map[string]string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

// PipeOpenAISSE copies an upstream OpenAI-shape SSE stream to w line-by-line,
// flushing after each line. Used by executors whose vendor already emits
// OpenAI chat-completion-chunk JSON in the SSE data: payload.
func PipeOpenAISSE(resp *http.Response, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	var usage Usage
	var firstLineWritten bool
	for scanner.Scan() {
		line := scanner.Bytes()
		if _, werr := w.Write(append(line, '\n')); werr != nil {
			return usage, StreamingPartialWrite, werr
		}
		firstLineWritten = true
		if bytes.HasPrefix(line, []byte(dataLine)) {
			payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte(dataLine)))
			if len(payload) > 0 && !bytes.Equal(payload, []byte("[DONE]")) {
				var probe struct {
					Usage *Usage `json:"usage,omitempty"`
				}
				if json.Unmarshal(payload, &probe) == nil && probe.Usage != nil {
					usage = *probe.Usage
				}
			}
		}
		flusher.Flush()
	}
	if err := scanner.Err(); err != nil {
		if firstLineWritten {
			return usage, StreamingPartialWrite, err
		}
		return usage, http.StatusBadGateway, err
	}
	return usage, http.StatusOK, nil
}

// DoNonStream sends a non-stream JSON POST and returns the raw response body.
// The caller is responsible for unmarshalling into the OpenAI response shape.
func DoNonStream(req *http.Request) ([]byte, Usage, int, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, Usage{}, http.StatusBadGateway, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if resp.StatusCode != http.StatusOK {
		return nil, Usage{}, resp.StatusCode, fmt.Errorf("upstream %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var probe struct {
		Usage Usage `json:"usage"`
	}
	_ = json.Unmarshal(body, &probe)
	return body, probe.Usage, resp.StatusCode, nil
}

// DoStream sends a streaming JSON POST and pipes the response through f.
func DoStream(req *http.Request, w http.ResponseWriter, flusher http.Flusher) (Usage, int, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return Usage{}, http.StatusBadGateway, fmt.Errorf("upstream: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		return Usage{}, resp.StatusCode, fmt.Errorf("upstream %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	return PipeOpenAISSE(resp, w, flusher)
}

// ProviderString safely extracts a string config value from p.Data, or "" when
// the key is absent or the wrong type.
func ProviderString(p *store.ProviderConnection, key string) string {
	if p == nil || p.Data == nil {
		return ""
	}
	if v, ok := p.Data[key].(string); ok {
		return v
	}
	return v0(p.Data[key])
}

func v0(v any) string {
	switch t := v.(type) {
	case fmt.Stringer:
		return t.String()
	default:
		return ""
	}
}

// MarshalRequest emits the canonical OpenAI request body that most vendors
// understand. Executors may post-process before sending.
func MarshalRequest(r Request) []byte {
	payload := map[string]any{
		"model":      r.Model,
		"messages":   toMessageMaps(r.Messages),
		"max_tokens": r.MaxTokens,
		"stream":     r.Stream,
	}
	if r.Temperature > 0 {
		payload["temperature"] = r.Temperature
	}
	if r.TopP > 0 {
		payload["top_p"] = r.TopP
	}
	if len(r.Tools) > 0 {
		payload["tools"] = r.Tools
	}
	b, _ := json.Marshal(payload)
	return b
}

func toMessageMaps(m []Message) []map[string]any {
	out := make([]map[string]any, len(m))
	for i, mm := range m {
		out[i] = map[string]any{"role": mm.Role, "content": mm.Content}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// trimRightSlash is a tiny helper used by every executor when joining a base
// URL with a path. We don't want double slashes.
func trimRightSlash(s string) string { return strings.TrimRight(s, "/") }
