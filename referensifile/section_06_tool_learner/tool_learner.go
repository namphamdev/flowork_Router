package ingestor

import (
	"database/sql"
	"log"
	"strings"

	"github.com/teetah2402/flowork/internal/provider"
)

// ToolLearner mempelajari pola penggunaan tool dari recordings.
// Setelah cukup data, brain bisa memprediksi tool mana yang cocok
// untuk prompt tertentu.
type ToolLearner struct {
	DB *sql.DB
}

// NewToolLearner membuat learner baru.
func NewToolLearner(db *sql.DB) *ToolLearner {
	return &ToolLearner{DB: db}
}

// LearnFromToolCalls mengekstrak pattern dari tool calls dan menyimpannya.
func (l *ToolLearner) LearnFromToolCalls(prompt string, toolCalls []provider.ToolCall) {
	if len(toolCalls) == 0 || prompt == "" {
		return
	}

	// Extract keywords dari prompt sebagai trigger
	words := strings.Fields(strings.ToLower(prompt))
	keywords := filterToolKeywords(words)
	if len(keywords) == 0 {
		return
	}
	trigger := strings.Join(keywords, " ")

	for _, tc := range toolCalls {
		_, err := l.DB.Exec(`
			INSERT INTO tool_patterns (trigger_pattern, tool_name, arguments_template, success_count)
			VALUES (?, ?, ?, 1)
			ON CONFLICT(trigger_pattern, tool_name)
			DO UPDATE SET
				success_count = success_count + 1,
				amplitude = CAST(success_count + 1 AS REAL) / MAX(success_count + fail_count + 1, 1)`,
			trigger, tc.Name, string(tc.Arguments),
		)
		if err != nil {
			log.Printf("fq-brain: tool learn error for %s: %v", tc.Name, err)
		}
	}
}

// PredictTools mengembalikan tool names yang paling mungkin dibutuhkan.
func (l *ToolLearner) PredictTools(prompt string, limit int) []string {
	words := strings.Fields(strings.ToLower(prompt))
	keywords := filterToolKeywords(words)
	if len(keywords) == 0 {
		return nil
	}

	// Match keywords ke trigger patterns
	var tools []string
	for _, kw := range keywords {
		rows, err := l.DB.Query(`
			SELECT tool_name FROM tool_patterns
			WHERE trigger_pattern LIKE ?
			ORDER BY amplitude DESC, success_count DESC
			LIMIT ?`, "%"+kw+"%", limit)
		if err != nil {
			continue
		}

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				tools = append(tools, name)
			}
		}
		// Sprint 3.5d (BUG-C15 fix): rows.Err() check
		if err := rows.Err(); err != nil {
			log.Printf("fq-brain: predict tools rows err: %v", err)
		}
		rows.Close()
	}

	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, t := range tools {
		if !seen[t] {
			seen[t] = true
			unique = append(unique, t)
		}
	}

	if len(unique) > limit {
		unique = unique[:limit]
	}
	return unique
}

// filterToolKeywords memilih kata-kata yang informatif untuk tool prediction.
func filterToolKeywords(words []string) []string {
	// Tool-relevant keywords
	actionWords := map[string]bool{
		"buat": true, "tulis": true, "baca": true, "hapus": true, "edit": true,
		"cari": true, "grep": true, "run": true, "jalankan": true, "test": true,
		"build": true, "commit": true, "push": true, "fetch": true, "install": true,
		"create": true, "write": true, "read": true, "delete": true, "search": true,
		"file": true, "folder": true, "directory": true, "command": true, "bash": true,
		"todo": true, "plan": true, "screenshot": true, "browser": true, "web": true,
	}

	var filtered []string
	for _, w := range words {
		if actionWords[w] || len(w) > 4 {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) > 5 {
		filtered = filtered[:5]
	}
	return filtered
}
