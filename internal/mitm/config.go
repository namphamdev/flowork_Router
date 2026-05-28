// MITM Config (TARGET_HOSTS, URL_PATTERNS, MODEL_*).

package mitm

import (
	"regexp"
	"strings"
)

// TargetHosts is the canonical list of hosts the MITM intercepts. Mirrors
// upstream config.js TARGET_HOSTS exactly.
var TargetHosts = []string{
	"daily-cloudcode-pa.googleapis.com",
	"cloudcode-pa.googleapis.com",
	"api.individual.githubcopilot.com",
	"q.us-east-1.amazonaws.com",
	"api2.cursor.sh",
}

// URLPatterns maps the tool name to the URL substrings the rewriter cares
// about. Mirrors upstream config.js URL_PATTERNS.
var URLPatterns = map[string][]string{
	"antigravity": {":generateContent", ":streamGenerateContent"},
	"copilot":     {"/chat/completions", "/v1/messages", "/responses"},
	"kiro":        {"/generateAssistantResponse"},
	"cursor":      {"/BidiAppend", "/RunSSE", "/RunPoll", "/Run"},
}

// ModelSynonyms — rawModel exact match → canonical alias.
var ModelSynonyms = map[string]map[string]string{
	"antigravity": {
		"gemini-default":     "gemini-3.5-flash-low",
		"gemini-3.1-pro-high": "gemini-pro-agent",
		"gemini-3-pro-high":  "gemini-pro-agent",
		"gemini-3-pro-low":   "gemini-3.1-pro-low",
	},
}

// ModelPattern is a (regex → canonical-alias) fallback entry. Order matters:
// more specific patterns first.
type ModelPattern struct {
	Match *regexp.Regexp
	Alias string
}

// ModelPatterns mirrors upstream config.js MODEL_PATTERNS.
var ModelPatterns = map[string][]ModelPattern{
	"antigravity": {
		{regexp.MustCompile(`(?i)flash.*low|low.*flash|flash.*medium|medium.*flash`), "gemini-3.5-flash-low"},
		{regexp.MustCompile(`(?i)flash.*agent|agent.*flash|flash`), "gemini-3-flash-agent"},
		{regexp.MustCompile(`(?i)pro.*low|low.*pro`), "gemini-3.1-pro-low"},
		{regexp.MustCompile(`(?i)gemini.*pro|pro.*gemini`), "gemini-pro-agent"},
		{regexp.MustCompile(`(?i)opus`), "claude-opus-4-6-thinking"},
		{regexp.MustCompile(`(?i)sonnet|claude`), "claude-sonnet-4-6"},
		{regexp.MustCompile(`(?i)gpt.*oss|oss`), "gpt-oss-120b-medium"},
	},
}

// LogBlacklistURLParts — substrings whose req/resp must NOT be dumped (noisy
// telemetry / polling). Mirrors upstream LOG_BLACKLIST_URL_PARTS.
var LogBlacklistURLParts = []string{
	"recordCodeAssistMetrics",
	"recordTrajectoryAnalytics",
	"fetchAdminControls",
	"listExperiments",
	"fetchUserInfo",
}

// GetToolForHost returns the per-IDE handler key ("antigravity" / "copilot" /
// "kiro" / "cursor") for the given host, or "" when not intercepted.
func GetToolForHost(host string) string {
	h := strings.SplitN(host, ":", 2)[0]
	switch h {
	case "api.individual.githubcopilot.com":
		return "copilot"
	case "daily-cloudcode-pa.googleapis.com", "cloudcode-pa.googleapis.com":
		return "antigravity"
	case "q.us-east-1.amazonaws.com":
		return "kiro"
	case "api2.cursor.sh":
		return "cursor"
	}
	return ""
}
