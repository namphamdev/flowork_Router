// Skills (Reusable Prompt Templates).

package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Skill — reusable prompt template.
type Skill struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`         // slug, used in URL/dispatch
	Description  string    `json:"description"`
	SystemPrompt string    `json:"systemPrompt"`
	UserTemplate string    `json:"userTemplate"` // template w/ {{var}} placeholders
	DefaultModel string    `json:"defaultModel"`
	Temperature  float64   `json:"temperature"`
	MaxTokens    int       `json:"maxTokens"`
	Variables    []string  `json:"variables"`    // list of {{var}} names extracted
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

const skillKVPrefix = "skill:"

func ListSkills(d *sql.DB) ([]Skill, error) {
	rows, err := d.Query(`SELECT k, v FROM kv WHERE k LIKE ? ORDER BY k ASC`, skillKVPrefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Skill
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		var s Skill
		if err := json.Unmarshal([]byte(v), &s); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func GetSkillByName(d *sql.DB, name string) (*Skill, error) {
	skills, err := ListSkills(d)
	if err != nil {
		return nil, err
	}
	for i := range skills {
		if skills[i].Name == name {
			return &skills[i], nil
		}
	}
	return nil, nil
}

func UpsertSkill(d *sql.DB, s *Skill) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
		s.CreatedAt = time.Now().UTC()
	}
	s.UpdatedAt = time.Now().UTC()
	s.Variables = extractVariables(s.UserTemplate)
	v, _ := json.Marshal(s)
	_, err := d.Exec(`INSERT INTO kv (k, v, updatedAt) VALUES (?, ?, ?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v, updatedAt=excluded.updatedAt`,
		skillKVPrefix+s.ID, string(v), s.UpdatedAt.Format(time.RFC3339))
	return err
}

func DeleteSkill(d *sql.DB, id string) error {
	_, err := d.Exec(`DELETE FROM kv WHERE k = ?`, skillKVPrefix+id)
	return err
}

// RenderSkillTemplate substitutes every {{var}} span in tpl with vars[name]
// (trimmed name). Unprovided variables render as "" so no literal template
// syntax leaks into the prompt. Mirrors extractVariables' scan so spacing like
// {{ name }} is handled identically.
func RenderSkillTemplate(tpl string, vars map[string]string) string {
	var b strings.Builder
	i := 0
	for i < len(tpl) {
		start := indexFrom(tpl, "{{", i)
		if start < 0 {
			b.WriteString(tpl[i:])
			break
		}
		end := indexFrom(tpl, "}}", start+2)
		if end < 0 {
			b.WriteString(tpl[i:])
			break
		}
		b.WriteString(tpl[i:start])
		b.WriteString(vars[strings.TrimSpace(tpl[start+2:end])])
		i = end + 2
	}
	return b.String()
}

// extractVariables finds {{var}} placeholders di template.
func extractVariables(tpl string) []string {
	var out []string
	seen := map[string]bool{}
	i := 0
	for i < len(tpl) {
		start := indexFrom(tpl, "{{", i)
		if start < 0 {
			break
		}
		end := indexFrom(tpl, "}}", start+2)
		if end < 0 {
			break
		}
		name := strings.TrimSpace(tpl[start+2 : end])
		if name != "" && !seen[name] {
			out = append(out, name)
			seen[name] = true
		}
		i = end + 2
	}
	return out
}

func indexFrom(s, sub string, start int) int {
	if start < 0 || start >= len(s) {
		return -1
	}
	idx := strings.Index(s[start:], sub)
	if idx < 0 {
		return -1
	}
	return start + idx
}
