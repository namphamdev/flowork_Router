// HTTP surface for the curated MCP plugin catalog.

package main

import (
	"net/http"

	"github.com/flowork-os/flowork_Router/internal/mcpcatalog"
)

// mcpCatalogHandler — GET /api/mcp/catalog returns the curated list of
// one-click MCP servers users can register from the dashboard.
func mcpCatalogHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	plugins := mcpcatalog.Catalog()
	writeJSON(w, http.StatusOK, map[string]any{
		"plugins": plugins,
		"count":   len(plugins),
	})
}
