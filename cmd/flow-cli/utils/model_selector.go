// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./cmd/flow-cli/utils package — audit pass surface review.

// Model selector: fetch /v1/models from the router and prompt user to pick.
package utils

import (
	"sort"

	"github.com/flowork-os/flowork_Router/cmd/flow-cli/api"
)

// PickModel asks the router for /v1/models, sorts by id, and prompts the user
// to choose one. Returns "" when no models found or selection cancelled.
func PickModel(c *api.Client) string {
	var wrap struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := c.Get("/v1/models", &wrap); err != nil {
		Error("/v1/models: " + err.Error())
		return ""
	}
	if len(wrap.Data) == 0 {
		Warn("no models available")
		return ""
	}
	sort.Slice(wrap.Data, func(i, j int) bool { return wrap.Data[i].ID < wrap.Data[j].ID })
	labels := make([]string, len(wrap.Data))
	for i, m := range wrap.Data {
		labels[i] = m.ID + "   (" + m.OwnedBy + ")"
	}
	idx := Select("Pick model", labels)
	if idx < 0 {
		return ""
	}
	return wrap.Data[idx].ID
}
