// Common helpers shared by filters: pre-compiled regex helpers + itoa.
package filters

import (
	"regexp"
	"strconv"
)

func mustCompile(p string) *regexp.Regexp { return regexp.MustCompile(p) }

func itoa(n int) string { return strconv.Itoa(n) }
