// Endpoint resolver: builds the effective router base URL from env/flags.
package utils

import (
	"net/url"
	"os"
	"strings"
)

const DefaultURL = "http://127.0.0.1:2402"

// Resolve returns the effective base URL. Priority: explicit override → env
// FLOW_ROUTER_URL → default. Strips trailing slashes.
func Resolve(override string) string {
	candidates := []string{override, os.Getenv("FLOW_ROUTER_URL"), DefaultURL}
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if u, err := url.Parse(c); err == nil && u.Scheme != "" && u.Host != "" {
			return strings.TrimRight(c, "/")
		}
	}
	return DefaultURL
}

// ResolveKey returns the bearer key from override/env (empty when neither set).
func ResolveKey(override string) string {
	if override != "" {
		return override
	}
	return os.Getenv("FLOW_ROUTER_KEY")
}
