// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/mitm/handlers package — audit pass surface review.

// Per-IDE MITM handler: kiro (AWS CodeWhisperer Kiro).
package handlers

import "net/http"

func init() { Register(&kiroHandler{}) }

type kiroHandler struct{}

func (k *kiroHandler) Name() string { return "kiro" }

// Handle maps /generateAssistantResponse to flow_router's chat completions
// endpoint. Strip the CodeWhisperer profile-arn header — the dispatcher does
// not need it; the kiro executor reads it from the provider connection data.
func (k *kiroHandler) Handle(w http.ResponseWriter, r *http.Request) {
	r.Header.Del("x-amzn-codewhisperer-profile-arn")
	r.Header.Del("amz-sdk-invocation-id")
	rerouteToRouter(w, r, "/v1/chat/completions")
}
