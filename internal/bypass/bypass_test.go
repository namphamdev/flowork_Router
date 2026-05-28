package bypass

import (
	"strings"
	"testing"
)

func TestDetect_WrongUserAgentNeverBypasses(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "Warmup"}}
	for _, ua := range []string{"", "curl/8.0", "python-requests/2.32", "Mozilla/5.0"} {
		if d := Detect(msgs, ua, nil, false); d.Bypass {
			t.Errorf("UA=%q must NOT trigger bypass", ua)
		}
	}
}

func TestDetect_Warmup(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "Warmup"}}
	d := Detect(msgs, "claude-cli/1.0", nil, false)
	if !d.Bypass || d.Reason != "warmup" {
		t.Fatalf("warmup not detected: %+v", d)
	}
}

func TestDetect_Count(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "count"}}
	d := Detect(msgs, "claude-cli/1.0", nil, false)
	if !d.Bypass || d.Reason != "count" {
		t.Fatalf("count not detected: %+v", d)
	}
}

func TestDetect_Title(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: "draft a function"},
		{Role: "assistant", Content: "{"},
	}
	d := Detect(msgs, "claude-cli/1.0", nil, false)
	if !d.Bypass || d.Reason != "title" {
		t.Fatalf("title bypass not detected: %+v", d)
	}
}

func TestDetect_SkipPatterns(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "internal heartbeat keepalive ping"}}
	d := Detect(msgs, "claude-cli/1.0", []string{"keepalive ping"}, false)
	if !d.Bypass || d.Reason != "skip-pattern" {
		t.Fatalf("skip pattern not detected: %+v", d)
	}
}

func TestDetect_SkipPatternsRequireNonEmpty(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "ping"}}
	d := Detect(msgs, "claude-cli/1.0", []string{"", "   "}, false)
	if d.Bypass {
		t.Fatalf("blank skip patterns must not trigger: %+v", d)
	}
}

func TestDetect_NamingBypass(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "Reply with isNewTopic JSON object"},
		{Role: "user", Content: "explain go channels for beginners"},
	}
	d := Detect(msgs, "claude-cli/1.0", nil, true)
	if !d.Bypass || d.Reason != "naming" {
		t.Fatalf("naming bypass not detected: %+v", d)
	}
	if d.NamingTitle != "explain go channels" {
		t.Fatalf("naming title should be first 3 words, got %q", d.NamingTitle)
	}
}

func TestDetect_NamingDisabledByFlag(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "Reply with isNewTopic JSON"},
		{Role: "user", Content: "anything"},
	}
	if d := Detect(msgs, "claude-cli/1.0", nil, false); d.Bypass {
		t.Fatalf("naming bypass triggered with flag off: %+v", d)
	}
}

func TestDetect_EmptyMessagesReturnsNoBypass(t *testing.T) {
	if d := Detect(nil, "claude-cli/1.0", nil, true); d.Bypass {
		t.Fatal("empty messages must not bypass")
	}
}

func TestStubText_NonNamingUsesDefault(t *testing.T) {
	d := Decision{Bypass: true, Reason: "warmup"}
	if StubText(d) != DefaultStubText {
		t.Fatalf("non-naming stub wrong: %s", StubText(d))
	}
}

func TestStubText_NamingEmitsJSONTitle(t *testing.T) {
	out := StubText(Decision{Bypass: true, Reason: "naming", NamingTitle: "hello world here"})
	if !strings.Contains(out, `"isNewTopic":true`) || !strings.Contains(out, `"title":"hello world here"`) {
		t.Fatalf("naming JSON shape wrong: %s", out)
	}
}

func TestJsonQuote_EscapesSpecials(t *testing.T) {
	if jsonQuote(`a"b\c`) != `"a\"b\\c"` {
		t.Fatalf("quoting wrong: %s", jsonQuote(`a"b\c`))
	}
	if jsonQuote("ab\nc") != `"ab\nc"` {
		t.Fatalf("newline escape wrong: %s", jsonQuote("ab\nc"))
	}
}
