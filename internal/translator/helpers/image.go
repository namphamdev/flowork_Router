// Helper: image content block translation (OpenAI ⇄ Anthropic ⇄ Gemini).
package helpers

import "strings"

// OpenAIImageToAnthropic converts an OpenAI `image_url` part →
// Anthropic `image.source` block. Supports both raw URL and base64 data-URL.
func OpenAIImageToAnthropic(part map[string]any) map[string]any {
	urlObj, _ := part["image_url"].(map[string]any)
	url, _ := urlObj["url"].(string)
	if strings.HasPrefix(url, "data:") {
		// data:<mime>;base64,<payload>
		mime, payload := splitDataURL(url)
		return map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mime,
				"data":       payload,
			},
		}
	}
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type": "url",
			"url":  url,
		},
	}
}

// AnthropicImageToOpenAI converts an Anthropic `image` block →
// OpenAI `image_url` part. Re-emits a data: URL when base64.
func AnthropicImageToOpenAI(block map[string]any) map[string]any {
	src, _ := block["source"].(map[string]any)
	srcType, _ := src["type"].(string)
	switch srcType {
	case "base64":
		mt, _ := src["media_type"].(string)
		data, _ := src["data"].(string)
		return map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": "data:" + mt + ";base64," + data,
			},
		}
	case "url":
		url, _ := src["url"].(string)
		return map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": url,
			},
		}
	}
	return nil
}

// OpenAIImageToGemini converts an OpenAI `image_url` part →
// Gemini `inline_data` part (base64) or `file_data` (URL).
func OpenAIImageToGemini(part map[string]any) map[string]any {
	urlObj, _ := part["image_url"].(map[string]any)
	url, _ := urlObj["url"].(string)
	if strings.HasPrefix(url, "data:") {
		mime, payload := splitDataURL(url)
		return map[string]any{
			"inline_data": map[string]any{
				"mime_type": mime,
				"data":      payload,
			},
		}
	}
	return map[string]any{
		"file_data": map[string]any{
			"file_uri": url,
		},
	}
}

// splitDataURL parses "data:<mime>;base64,<payload>" → (mime, payload).
// Falls back to ("application/octet-stream", "") on malformed input.
func splitDataURL(s string) (string, string) {
	if !strings.HasPrefix(s, "data:") {
		return "application/octet-stream", ""
	}
	body := strings.TrimPrefix(s, "data:")
	sep := strings.Index(body, ",")
	if sep < 0 {
		return "application/octet-stream", ""
	}
	head, payload := body[:sep], body[sep+1:]
	mime := head
	if i := strings.Index(head, ";"); i >= 0 {
		mime = head[:i]
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	return mime, payload
}
