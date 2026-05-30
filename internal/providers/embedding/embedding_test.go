// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file. Standard test patterns, no production race/leak risk.

package embedding

import "testing"

func TestEmbeddingProviders_AllRegistered(t *testing.T) {
	want := []string{"openai", "gemini", "openaiCompat"}
	for _, n := range want {
		if Get(n) == nil {
			t.Fatalf("embedding provider %q not registered", n)
		}
	}
}
