// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./cmd/flow-cli/utils package — audit pass surface review.

// Common menu plumbing shared by all sub-menus.
package utils

import "fmt"

// MenuItem is one entry in an interactive menu.
type MenuItem struct {
	Label  string
	Action func() error
}

// RunMenu draws a numbered list with title and dispatches to the picked item.
// Returns when the user picks "Back" (the implicit final entry) or invalid input.
func RunMenu(title string, items []MenuItem) error {
	for {
		Header(title)
		labels := make([]string, len(items)+1)
		for i, it := range items {
			labels[i] = it.Label
		}
		labels[len(items)] = "Back"
		idx := Select("Choose", labels)
		if idx < 0 || idx == len(items) {
			return nil
		}
		if err := items[idx].Action(); err != nil {
			Error(fmt.Sprintf("%v", err))
		}
	}
}
