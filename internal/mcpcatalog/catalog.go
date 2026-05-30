// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// Curated catalog of one-click MCP servers users can register without
// hand-editing config. Two flavours:
//
//	Remote HTTP MCP   — hosted endpoints (exa.ai, tavily.com), no spawn
//	Local stdio MCP   — npx-wrapped local processes (browsermcp, …)
//
// Surfacing this list via /api/mcp/catalog lets the dashboard's MCP tab
// render a "register one-click" card next to each entry; the actual
// registration still flows through the existing MCPServer CRUD so the
// allowlist + handler paths stay consistent.

package mcpcatalog

// Plugin describes a single one-click MCP entry. Empty Command means the
// entry is HTTP-only; empty URL means it's spawn-only.
type Plugin struct {
	Name        string   `json:"name"`        // stable id used by clients to dedup
	Title       string   `json:"title"`       // human-readable
	Description string   `json:"description"` // 1-2 sentences for the card
	Transport   string   `json:"transport"`   // "http" | "stdio"
	URL         string   `json:"url,omitempty"`
	Command     string   `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`
	OAuth       bool     `json:"oauth,omitempty"`        // upstream uses OAuth handshake
	Extension   string   `json:"extensionUrl,omitempty"` // companion browser/IDE extension
	ToolNames   []string `json:"toolNames,omitempty"`    // declared tool ids
}

// DefaultPlugins is the canonical list rendered on first boot. Operators
// can extend at runtime via Register() or replace via Set().
var DefaultPlugins = []Plugin{
	{
		Name:        "exa",
		Title:       "Exa",
		Description: "Real-time web search + code documentation lookup via Exa's hosted MCP.",
		Transport:   "http",
		URL:         "https://mcp.exa.ai/mcp",
		OAuth:       false,
		ToolNames:   []string{"web_search_exa", "web_fetch_exa"},
	},
	{
		Name:        "tavily",
		Title:       "Tavily",
		Description: "Web search optimised for LLM agents (search + extract + crawl + map).",
		Transport:   "http",
		URL:         "https://mcp.tavily.com/mcp",
		OAuth:       true,
		ToolNames:   []string{"tavily_search", "tavily_extract", "tavily_crawl", "tavily_map"},
	},
	{
		Name:        "browsermcp",
		Title:       "Browser MCP",
		Description: "Drive your running Chrome instance (requires the Browser MCP extension).",
		Transport:   "stdio",
		Command:     "npx",
		Args:        []string{"-y", "@browsermcp/mcp@latest"},
		Extension:   "https://chromewebstore.google.com/detail/browser-mcp-automate-your/bjfgambnhccakkhmkepdoekmckoijdlc",
		ToolNames: []string{
			"browser_navigate", "browser_snapshot", "browser_click", "browser_type",
			"browser_screenshot", "browser_get_console_logs", "browser_wait",
			"browser_press_key", "browser_go_back", "browser_go_forward",
		},
	},
}

var custom []Plugin

// Catalog returns the full one-click plugin list: defaults followed by any
// runtime registrations, with later duplicates (by Name) ignored.
func Catalog() []Plugin {
	out := make([]Plugin, 0, len(DefaultPlugins)+len(custom))
	seen := map[string]bool{}
	for _, p := range DefaultPlugins {
		if p.Name == "" || seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		out = append(out, p)
	}
	for _, p := range custom {
		if p.Name == "" || seen[p.Name] {
			continue
		}
		seen[p.Name] = true
		out = append(out, p)
	}
	return out
}

// Register adds a custom plugin (idempotent by Name — later wins).
func Register(p Plugin) {
	if p.Name == "" {
		return
	}
	// Drop any existing entry with the same name first so the new one wins.
	filtered := custom[:0]
	for _, e := range custom {
		if e.Name != p.Name {
			filtered = append(filtered, e)
		}
	}
	custom = append(filtered, p)
}

// Set replaces the custom layer entirely. Defaults are untouched.
func Set(list []Plugin) {
	custom = append(custom[:0], list...)
}

// Lookup returns the plugin with the given name, or false.
func Lookup(name string) (Plugin, bool) {
	for _, p := range Catalog() {
		if p.Name == name {
			return p, true
		}
	}
	return Plugin{}, false
}
