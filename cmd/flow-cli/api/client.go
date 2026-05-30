// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — CLI command/menu.

// flow-cli HTTP client.

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client is the shared HTTP client used by every CLI menu.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// New returns a client configured for baseURL + optional bearer apiKey.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Get fetches path and unmarshals the response into out (which may be nil).
func (c *Client) Get(path string, out any) error { return c.do("GET", path, nil, out) }

// Post sends body as JSON and unmarshals the response into out.
func (c *Client) Post(path string, body, out any) error { return c.do("POST", path, body, out) }

// Put is the PUT variant.
func (c *Client) Put(path string, body, out any) error { return c.do("PUT", path, body, out) }

// Delete sends a DELETE for path.
func (c *Client) Delete(path string) error { return c.do("DELETE", path, nil, nil) }

// GetQuery convenience: appends a query string from params and GETs.
func (c *Client) GetQuery(path string, params map[string]string, out any) error {
	if len(params) > 0 {
		v := url.Values{}
		for k, val := range params {
			v.Set(k, val)
		}
		sep := "?"
		if bytes.ContainsRune([]byte(path), '?') {
			sep = "&"
		}
		path += sep + v.Encode()
	}
	return c.Get(path, out)
}

func (c *Client) do(method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	if rdr != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s -> %d: %s", method, path, resp.StatusCode, truncate(data))
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decode: %w (raw: %s)", err, truncate(data))
		}
	}
	return nil
}

func truncate(b []byte) string {
	if len(b) > 240 {
		return string(b[:240]) + "…"
	}
	return string(b)
}
