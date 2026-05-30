// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/router package — audit pass surface review.

// Caveman injection adapter.

package router

import "github.com/flowork-os/flowork_Router/internal/caveman"

// injectCavemanIntoRequest mutates req.Messages so the dispatched payload
// carries the caveman style instruction. No-op when level is off/unknown.
func injectCavemanIntoRequest(req *OpenAIRequest, level string) {
	prompt := caveman.Prompt(caveman.Normalize(level))
	if prompt == "" {
		return
	}
	for i := range req.Messages {
		if req.Messages[i].Role == "system" || req.Messages[i].Role == "developer" {
			req.Messages[i].Content = caveman.InjectIntoSystem(req.Messages[i].Content, prompt)
			return
		}
	}
	// No existing system message — prepend one carrying just the modifier.
	req.Messages = append([]OpenAIMessage{{Role: "system", Content: prompt}}, req.Messages...)
}
