// Package ingestor — model_resolver.go
// Resolves agent-specific model from brain DB at daemon boot time.
//
// Called by aiwarga.Run() after env var resolution. If the agent has a model
// set in the brain DB (via GUI swap), that takes precedence over the default
// model but NOT over explicit env var override (env > DB > default).
//
// This completes the "runtime wiring" gap where model swap in the GUI only
// updated the DB + prompt file but didn't affect the actual model used at
// runtime.
package ingestor

import (
	"fmt"
	"log"
	"strings"

	braindb "github.com/teetah2402/flowork/brain/db"
	_ "modernc.org/sqlite"

	"github.com/teetah2402/flowork/internal/wargaregistry"
)

// ResolveAgentModel queries the brain DB for an agent's model ID.
// Returns the model_id string if found, or empty string if:
//   - agent not found in DB
//   - DB file doesn't exist or can't be opened
//   - model field is empty in DB
//
// This is a best-effort lookup — callers should use their own default
// if the returned string is empty. The function never returns an error
// that should block daemon startup.
func ResolveAgentModel(workspace, agentName string) string {
	if workspace == "" || agentName == "" {
		return ""
	}
	db, err := braindb.Open(workspace)
	if err != nil {
		return ""
	}
	defer db.Close()

	var model string
	err = db.QueryRow("SELECT COALESCE(model,'') FROM agents WHERE name = ?", agentName).Scan(&model)
	if err != nil {
		return ""
	}
	model = strings.TrimSpace(model)
	if model != "" {
		log.Printf("fq-brain: resolved model for %q from DB: %s", agentName, model)
	}
	return model
}

// ResolveAgentModelWithFallback is a convenience wrapper that returns
// fallbackModel if DB lookup returns empty.
func ResolveAgentModelWithFallback(workspace, agentName, fallbackModel string) string {
	if resolved := ResolveAgentModel(workspace, agentName); resolved != "" {
		return resolved
	}
	return fallbackModel
}

// LookupAgentPersona maps daemon identity to the Nusantara persona name.
// Delegates to wargaregistry.IdentityWarga — single source of truth (effekdomino #8).
func LookupAgentPersona(identity string) string {
	return wargaregistry.IdentityWarga(identity)
}

// ResolveModelForDaemon combines persona lookup + DB model resolution.
// This is the main entry point for aiwarga.Run().
//
// Priority chain: env var (already handled by caller) > DB model > defaultModel.
// This function only handles the "DB model > defaultModel" part.
func ResolveModelForDaemon(workspace, identity, defaultModel string) string {
	persona := LookupAgentPersona(identity)
	resolved := ResolveAgentModelWithFallback(workspace, persona, "")
	if resolved != "" {
		return resolved
	}
	// Also try with the raw identity name (for agents like merpati, balai
	// whose identity IS the persona name)
	if persona != identity {
		resolved = ResolveAgentModelWithFallback(workspace, identity, "")
		if resolved != "" {
			return resolved
		}
	}
	return defaultModel
}

// UpdateAgentModelInDB persists a model change to the brain DB.
// Called when /model slash command changes model at runtime — keeps DB
// in sync with the running agent.
func UpdateAgentModelInDB(workspace, agentName, modelID string) error {
	if workspace == "" || agentName == "" {
		return fmt.Errorf("workspace and agentName required")
	}
	db, err := braindb.Open(workspace)
	if err != nil {
		return fmt.Errorf("open brain DB: %w", err)
	}
	defer db.Close()

	_, err = db.Exec("UPDATE agents SET model = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?",
		modelID, agentName)
	if err != nil {
		return fmt.Errorf("update agent model: %w", err)
	}
	log.Printf("fq-brain: updated model for %q in DB: %s", agentName, modelID)
	return nil
}
