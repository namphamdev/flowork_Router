// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file. Standard test patterns, no production race/leak risk.

package brain

import "testing"

func TestSkillsLoaded(t *testing.T) {
	all := Skills()
	if len(all) < 30 {
		t.Fatalf("expected ~40 embedded skills, got %d", len(all))
	}
	// every skill should have a name and a non-empty body
	for _, s := range all {
		if s.Name == "" || s.Body == "" {
			t.Errorf("skill missing name/body: %+v", s.Name)
		}
	}
	t.Logf("loaded %d skills", len(all))
}

func TestSelectSkills(t *testing.T) {
	cases := []string{
		"help me debug this error",
		"plan the implementation step by step",
		"verify the result before claiming done",
	}
	for _, q := range cases {
		sel := SelectSkills(q, 3)
		t.Logf("query %q → %d skills", q, len(sel))
		for _, s := range sel {
			t.Logf("   - %s :: %s", s.Name, s.Description)
		}
		if len(sel) == 0 {
			t.Errorf("expected at least one skill for %q", q)
		}
	}
}
