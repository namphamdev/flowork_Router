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
