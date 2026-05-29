// Package ingestor — model_pool.go
// Ingest katalog 344+ model dari promp/LIST_MODEL.MD ke tabel model_pool.
// Digunakan sebagai lookup pool saat swap model engine agent.
package ingestor

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IngestModelPool memparse katalog model dari `prompt_templates WHERE name='list_model'`
// (canonical) atau fallback ke file `promp/LIST_MODEL.MD`, lalu insert/update ke
// tabel model_pool. Format content = Markdown table per kategori.
//
// Catatan: katalog model biasanya di-override via RefreshModelPool (live fetch
// dari OpenRouter API). IngestModelPool jalan di boot sebagai seed awal kalau
// model_pool kosong.
func IngestModelPool(db *sql.DB, projectRoot string) (int, error) {
	content := loadModelPoolContent(db, projectRoot)
	if content == "" {
		return 0, nil // ga ada source = skip
	}

	count := 0
	currentCategory := ""
	scanner := bufio.NewScanner(strings.NewReader(content))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect category heading: ## Kategori: Audio/TTS (4 models)
		if strings.HasPrefix(line, "## Kategori:") {
			cat := strings.TrimPrefix(line, "## Kategori:")
			// Strip " (N models)" suffix
			if idx := strings.Index(cat, "("); idx > 0 {
				cat = cat[:idx]
			}
			currentCategory = strings.TrimSpace(cat)
			continue
		}

		// Skip headers and separators
		if !strings.HasPrefix(line, "| `") {
			continue
		}

		// Parse table row: | `model_id` | Name | Context | $Prompt | $Completion |
		parts := strings.Split(line, "|")
		if len(parts) < 6 {
			continue
		}

		modelID := strings.Trim(strings.TrimSpace(parts[1]), "`")
		modelName := strings.TrimSpace(parts[2])
		contextStr := strings.TrimSpace(parts[3])
		costPromptStr := strings.TrimSpace(parts[4])
		costCompStr := strings.TrimSpace(parts[5])

		if modelID == "" || modelName == "" {
			continue
		}

		contextWindow, _ := strconv.Atoi(contextStr)

		// Parse cost: "$0.000002" → float64
		costPrompt := parseCost(costPromptStr)
		costCompletion := parseCost(costCompStr)

		isFree := 0
		if costPrompt == 0 && costCompletion == 0 {
			isFree = 1
		}

		_, err := db.Exec(`INSERT INTO model_pool (model_id, model_name, category, context_window, cost_prompt, cost_completion, is_free)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(model_id) DO UPDATE SET
				model_name=excluded.model_name,
				category=excluded.category,
				context_window=excluded.context_window,
				cost_prompt=excluded.cost_prompt,
				cost_completion=excluded.cost_completion,
				is_free=excluded.is_free`,
			modelID, modelName, currentCategory, contextWindow, costPrompt, costCompletion, isFree)
		if err == nil {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("scanner error: %w", err)
	}

	return count, nil
}

// loadModelPoolContent resolve katalog model markdown. Priority:
//  1. prompt_templates WHERE LOWER(name)='list_model'
//  2. File fallback `promp/LIST_MODEL.MD` — emergency bootstrap
func loadModelPoolContent(db *sql.DB, projectRoot string) string {
	var tmpl string
	if err := db.QueryRow(
		"SELECT content FROM prompt_templates WHERE LOWER(name) = 'list_model'",
	).Scan(&tmpl); err == nil && strings.TrimSpace(tmpl) != "" {
		return tmpl
	}
	if data, err := os.ReadFile(filepath.Join(projectRoot, "promp", "LIST_MODEL.MD")); err == nil {
		return string(data)
	}
	return ""
}

// parseCost parses "$0.000002" or "$0" to float64.
func parseCost(s string) float64 {
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimSpace(s)
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
