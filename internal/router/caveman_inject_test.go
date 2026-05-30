// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — Test file.

package router

import (
	"strings"
	"testing"
)

func TestInjectCaveman_OffIsNoOp(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "user", Content: "hello"},
	}}
	injectCavemanIntoRequest(req, "")
	if len(req.Messages) != 1 || req.Messages[0].Role != "user" {
		t.Fatalf("empty level must not mutate messages: %+v", req.Messages)
	}
}

func TestInjectCaveman_AppendsToExistingSystem(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "hi"},
	}}
	injectCavemanIntoRequest(req, "lite")
	if len(req.Messages) != 2 {
		t.Fatalf("should not add a new message, got %d", len(req.Messages))
	}
	sys := req.Messages[0].Content
	if !strings.HasPrefix(sys, "You are a helpful assistant.") {
		t.Fatalf("existing system text dropped: %q", sys)
	}
	if !strings.Contains(sys, "Respond tersely") {
		t.Fatalf("caveman prompt not appended: %q", sys)
	}
}

func TestInjectCaveman_PrependsWhenNoSystem(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "user", Content: "hi"},
	}}
	injectCavemanIntoRequest(req, "ultra")
	if len(req.Messages) != 2 {
		t.Fatalf("should prepend a system message, got %d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Fatalf("first message must be system, got %q", req.Messages[0].Role)
	}
	if !strings.Contains(req.Messages[0].Content, "ultra-terse") {
		t.Fatalf("ultra prompt not present: %q", req.Messages[0].Content)
	}
}

func TestInjectCaveman_DeveloperRoleTreatedAsSystem(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{
		{Role: "developer", Content: "rules go here"},
		{Role: "user", Content: "hi"},
	}}
	injectCavemanIntoRequest(req, "full")
	if len(req.Messages) != 2 {
		t.Fatalf("should not add a new message, got %d", len(req.Messages))
	}
	if !strings.HasPrefix(req.Messages[0].Content, "rules go here") {
		t.Fatalf("developer content dropped: %q", req.Messages[0].Content)
	}
	if !strings.Contains(req.Messages[0].Content, "caveman") {
		t.Fatalf("caveman prompt missing on developer role: %q", req.Messages[0].Content)
	}
}

func TestInjectCaveman_UnknownLevelIsNoOp(t *testing.T) {
	req := &OpenAIRequest{Messages: []OpenAIMessage{{Role: "user", Content: "hi"}}}
	injectCavemanIntoRequest(req, "thinking-quirky")
	if len(req.Messages) != 1 {
		t.Fatalf("unknown level must be no-op, got %d msgs", len(req.Messages))
	}
}
