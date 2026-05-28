// Antigravity IDE version override.

package mitm

import "strings"

// Default override; bumped over time to match the version Cloud Code Assist's
// Antigravity endpoint expects.
var AntigravityIDEVersionDefault = "0.16.0"

// ApplyAntigravityIDEVersionOverride rewrites the IDE version header value to
// match what Antigravity expects. Mirrors the upstream patch step that prevents
// "your client is too old" errors when the upstream gets aggressive about it.
func ApplyAntigravityIDEVersionOverride(headers map[string]string) {
	if headers == nil {
		return
	}
	for k := range headers {
		if strings.EqualFold(k, "x-ide-version") || strings.EqualFold(k, "x-client-version") {
			headers[k] = AntigravityIDEVersionDefault
		}
	}
}
