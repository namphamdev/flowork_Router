package router

import (
	"testing"

	"github.com/flowork-os/flowork_Router/internal/store"
)

func TestComboFallbackOrder_ExcludesPicked(t *testing.T) {
	c := &store.Combo{
		Models:   []string{"sonnet", "haiku", "opus"},
		Strategy: store.ComboStrategyPriority,
	}
	got := comboFallbackOrder(c, "sonnet")
	if len(got) != 2 || got[0] != "haiku" || got[1] != "opus" {
		t.Fatalf("expected [haiku opus], got %v", got)
	}
}

func TestComboFallbackOrder_PreservesPriorityOrder(t *testing.T) {
	c := &store.Combo{
		Models:   []string{"a", "b", "c", "d"},
		Strategy: store.ComboStrategyPriority,
	}
	got := comboFallbackOrder(c, "c")
	if len(got) != 3 {
		t.Fatalf("expected 3, got %v", got)
	}
	// Other models keep original relative order.
	if got[0] != "a" || got[1] != "b" || got[2] != "d" {
		t.Fatalf("order broken: %v", got)
	}
}

func TestComboFallbackOrder_NilForSingleOrEmpty(t *testing.T) {
	if got := comboFallbackOrder(&store.Combo{Models: []string{"only"}}, "only"); got != nil {
		t.Fatalf("single-model combo should return nil, got %v", got)
	}
	if got := comboFallbackOrder(&store.Combo{Models: nil}, ""); got != nil {
		t.Fatalf("empty combo should return nil, got %v", got)
	}
	if got := comboFallbackOrder(nil, ""); got != nil {
		t.Fatalf("nil combo should return nil, got %v", got)
	}
}

func TestComboFallbackOrder_HandlesPickedNotInList(t *testing.T) {
	// Picked model didn't end up in the list (shouldn't happen, but defensive).
	c := &store.Combo{Models: []string{"a", "b"}}
	got := comboFallbackOrder(c, "x-never-in-list")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("expected both models intact, got %v", got)
	}
}

func TestComboFallbackOrder_DuplicateModels(t *testing.T) {
	// Real configs sometimes have duplicates (manual edit). ALL occurrences
	// of `picked` get dropped — fallback walks the remaining unique-by-
	// position list.
	c := &store.Combo{Models: []string{"a", "a", "b", "a"}}
	got := comboFallbackOrder(c, "a")
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("expected [b], got %v", got)
	}
}
