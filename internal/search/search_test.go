package search

import "testing"

func TestSearchProviders_AllRegistered(t *testing.T) {
	want := []string{"tavily", "brave", "serpapi", "duckduckgo"}
	for _, n := range want {
		if Get(n) == nil {
			t.Fatalf("search provider %q not registered", n)
		}
	}
	if len(List()) < len(want) {
		t.Fatalf("List returned %d, expected ≥%d", len(List()), len(want))
	}
}
