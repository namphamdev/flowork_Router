// Package ingestor — model_refresh.go
// Scheduled model pool refresh from OpenRouter API.
// Used by the scheduled pricing goroutine in flowork-gui main.go.
package ingestor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/teetah2402/flowork/internal/safeclient"
)

// RefreshModelPool fetches live model data from OpenRouter and updates model_pool table.
// This is the reusable version of BrainModelPoolRefreshHandler's core logic.
func RefreshModelPool(database *sql.DB, apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("API key not provided")
	}

	client := safeclient.NewClient(30 * time.Second)
	req, _ := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB limit
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var orResp struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Pricing struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
			ContextLength int `json:"context_length"`
			Architecture  *struct {
				Modality string `json:"modality"`
			} `json:"architecture"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &orResp); err != nil {
		return fmt.Errorf("parse OpenRouter response: %w", err)
	}

	updated := 0
	for _, m := range orResp.Data {
		if m.ID == "" {
			continue
		}
		modelName := m.Name
		if modelName == "" {
			modelName = m.ID
		}
		category := categorizeModelRefresh(m.ID, func() string {
			if m.Architecture != nil {
				return m.Architecture.Modality
			}
			return ""
		}())
		costPrompt := parseORPriceRefresh(m.Pricing.Prompt)
		costCompletion := parseORPriceRefresh(m.Pricing.Completion)
		isFree := 0
		if costPrompt == 0 && costCompletion == 0 {
			isFree = 1
		}

		_, err := database.Exec(`INSERT INTO model_pool (model_id, model_name, category, context_window, cost_prompt, cost_completion, is_free)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(model_id) DO UPDATE SET
				model_name=excluded.model_name,
				category=excluded.category,
				context_window=excluded.context_window,
				cost_prompt=excluded.cost_prompt,
				cost_completion=excluded.cost_completion,
				is_free=excluded.is_free`,
			m.ID, modelName, category, m.ContextLength, costPrompt, costCompletion, isFree)
		if err == nil {
			updated++
		}
	}

	log.Printf("fq-brain: model pool refresh — %d models fetched, %d updated/inserted", len(orResp.Data), updated)
	return nil
}

// categorizeModelRefresh maps model ID + modality to display category.
// Duplicate of guiapi.categorizeModel to avoid import cycle.
func categorizeModelRefresh(modelID, modality string) string {
	low := strings.ToLower(modelID + " " + modality)
	switch {
	case strings.Contains(low, "image") || strings.Contains(low, "dall-e") || strings.Contains(low, "stable-diffusion") || strings.Contains(low, "flux"):
		return "Vision/Image"
	case strings.Contains(low, "audio") || strings.Contains(low, "tts") || strings.Contains(low, "whisper"):
		return "Audio/TTS"
	case strings.Contains(low, "video"):
		return "Video"
	case strings.Contains(low, "embed"):
		return "Embedding"
	case strings.Contains(low, ":free") || strings.Contains(low, "free"):
		return "Free Models"
	default:
		return "Text/Chat"
	}
}

// parseORPriceRefresh parses OpenRouter pricing string to float64.
func parseORPriceRefresh(s string) float64 {
	if s == "" || s == "0" {
		return 0
	}
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
