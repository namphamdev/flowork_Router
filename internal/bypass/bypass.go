// Claude CLI bypass handler.

package bypass

import (
	"strings"
)

// Message is the minimum view the detector needs of one chat message.
type Message struct {
	Role    string
	Content string
}

// Decision is the detector verdict.
type Decision struct {
	// Bypass = true → caller should NOT forward to upstream.
	Bypass bool
	// NamingTitle, if non-empty, is the topic title to return in the body of
	// the stub response (3-word slice of the user message).
	NamingTitle string
	// Reason identifies which rule fired — exposed for logging.
	Reason string
}

// Detect inspects the request shape against the five known Claude-CLI no-op
// patterns. SkipPatterns is the (user-configurable) extra list of substrings
// that should also short-circuit; pass nil/empty to disable that check.
// CcFilterNaming toggles the "isNewTopic" probe (off by default).
func Detect(messages []Message, userAgent string, skipPatterns []string, ccFilterNaming bool) Decision {
	if !strings.Contains(userAgent, "claude-cli") || len(messages) == 0 {
		return Decision{}
	}

	// 1) Title extraction — trailing assistant message contains exactly "{".
	last := messages[len(messages)-1]
	if last.Role == "assistant" && strings.TrimSpace(last.Content) == "{" {
		return Decision{Bypass: true, Reason: "title"}
	}

	// 2) Warmup — first message text == "Warmup".
	first := messages[0]
	if strings.TrimSpace(first.Content) == "Warmup" {
		return Decision{Bypass: true, Reason: "warmup"}
	}

	// 3) Count — exactly one user message saying "count".
	if len(messages) == 1 && messages[0].Role == "user" && strings.TrimSpace(messages[0].Content) == "count" {
		return Decision{Bypass: true, Reason: "count"}
	}

	// 4) Skip patterns — any user message contains any listed substring.
	if len(skipPatterns) > 0 {
		var sb strings.Builder
		for _, m := range messages {
			if m.Role == "user" {
				sb.WriteString(m.Content)
				sb.WriteByte(' ')
			}
		}
		joined := sb.String()
		for _, p := range skipPatterns {
			if p = strings.TrimSpace(p); p != "" && strings.Contains(joined, p) {
				return Decision{Bypass: true, Reason: "skip-pattern"}
			}
		}
	}

	// 5) CC naming bypass — when enabled, a system message carrying
	//    "isNewTopic" means the CLI is asking us to title the conversation.
	//    Return the first ≤3 user words as the title.
	if ccFilterNaming {
		var systemText strings.Builder
		var firstUser string
		for _, m := range messages {
			if m.Role == "system" {
				systemText.WriteString(m.Content)
				systemText.WriteByte(' ')
			}
			if firstUser == "" && m.Role == "user" {
				firstUser = m.Content
			}
		}
		if strings.Contains(systemText.String(), "isNewTopic") {
			return Decision{
				Bypass:      true,
				Reason:      "naming",
				NamingTitle: firstThreeWords(firstUser),
			}
		}
	}

	return Decision{}
}

// DefaultStubText is the body of a non-naming stub response.
const DefaultStubText = "CLI Command Execution: Clear Terminal"

// StubText returns the content to put in the assistant message of the stub
// response. For a naming bypass the body is a JSON object the CLI expects;
// for any other bypass we emit a harmless placeholder.
func StubText(d Decision) string {
	if d.NamingTitle != "" {
		// Manual JSON: avoid encoding/json import for one fixed shape.
		return `{"isNewTopic":true,"title":` + jsonQuote(d.NamingTitle) + `}`
	}
	return DefaultStubText
}

func firstThreeWords(s string) string {
	parts := strings.Fields(strings.TrimSpace(s))
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, " ")
}

// jsonQuote is a small string escaper sufficient for the title — escapes the
// two characters the JSON spec requires inside a "string" literal.
func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				continue // drop control chars
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
