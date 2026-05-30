// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package mcpcatalog

import (
	"testing"
)

func withCleanCustom(t *testing.T, fn func()) {
	t.Helper()
	orig := append([]Plugin{}, custom...)
	defer func() { custom = orig }()
	custom = nil
	fn()
}

func TestCatalog_ContainsAllDefaults(t *testing.T) {
	withCleanCustom(t, func() {
		c := Catalog()
		seen := map[string]bool{}
		for _, p := range c {
			seen[p.Name] = true
		}
		for _, want := range []string{"exa", "tavily", "browsermcp"} {
			if !seen[want] {
				t.Errorf("default plugin missing: %q", want)
			}
		}
	})
}

func TestCatalog_ExaIsHTTP(t *testing.T) {
	withCleanCustom(t, func() {
		p, ok := Lookup("exa")
		if !ok {
			t.Fatal("exa missing")
		}
		if p.Transport != "http" {
			t.Errorf("exa transport: %s", p.Transport)
		}
		if p.URL != "https://mcp.exa.ai/mcp" {
			t.Errorf("exa URL drifted: %s", p.URL)
		}
		if p.OAuth {
			t.Error("exa should not require OAuth")
		}
	})
}

func TestCatalog_TavilyRequiresOAuth(t *testing.T) {
	withCleanCustom(t, func() {
		p, _ := Lookup("tavily")
		if !p.OAuth {
			t.Error("tavily MCP requires OAuth — flag missing")
		}
		if len(p.ToolNames) < 4 {
			t.Errorf("tavily should declare 4 tools, got %d", len(p.ToolNames))
		}
	})
}

func TestCatalog_BrowserMCPIsStdio(t *testing.T) {
	withCleanCustom(t, func() {
		p, _ := Lookup("browsermcp")
		if p.Transport != "stdio" {
			t.Errorf("browsermcp should be stdio, got %s", p.Transport)
		}
		if p.Command != "npx" {
			t.Errorf("browsermcp command: %s", p.Command)
		}
		if p.Extension == "" {
			t.Error("browsermcp should advertise its companion extension URL")
		}
	})
}

func TestRegister_AddsCustomEntry(t *testing.T) {
	withCleanCustom(t, func() {
		Register(Plugin{Name: "myplug", Title: "My", Transport: "http", URL: "https://x"})
		got, ok := Lookup("myplug")
		if !ok {
			t.Fatal("registered plugin missing from catalog")
		}
		if got.Title != "My" {
			t.Errorf("title wrong: %s", got.Title)
		}
	})
}

func TestRegister_IgnoresBlankName(t *testing.T) {
	withCleanCustom(t, func() {
		before := len(Catalog())
		Register(Plugin{Title: "no name"})
		if len(Catalog()) != before {
			t.Fatal("blank-name plugin should be ignored")
		}
	})
}

func TestRegister_LaterWinsOnNameClash(t *testing.T) {
	withCleanCustom(t, func() {
		Register(Plugin{Name: "x", Title: "first"})
		Register(Plugin{Name: "x", Title: "second"})
		p, _ := Lookup("x")
		if p.Title != "second" {
			t.Fatalf("expected later registration to win, got %q", p.Title)
		}
	})
}

func TestRegister_NeverShadowsDefaults(t *testing.T) {
	withCleanCustom(t, func() {
		Register(Plugin{Name: "exa", Title: "hijacker"})
		p, _ := Lookup("exa")
		if p.Title == "hijacker" {
			t.Fatal("custom registration must NOT shadow defaults — defaults are deduped first")
		}
	})
}

func TestSet_ReplacesCustomLayerOnly(t *testing.T) {
	withCleanCustom(t, func() {
		Register(Plugin{Name: "one"})
		Register(Plugin{Name: "two"})
		Set([]Plugin{{Name: "fresh"}})
		if _, ok := Lookup("one"); ok {
			t.Error("Set() should have wiped previous custom entries")
		}
		// Defaults survive.
		if _, ok := Lookup("exa"); !ok {
			t.Error("Set() must not touch defaults")
		}
	})
}

func TestLookup_UnknownReturnsFalse(t *testing.T) {
	if _, ok := Lookup("nonexistent"); ok {
		t.Fatal("unknown plugin must return false")
	}
}
