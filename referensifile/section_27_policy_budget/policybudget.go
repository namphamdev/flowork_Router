// Package policybudget wires the policy.Limits resolver ke finance.BudgetGuard
// shared singleton. Package terpisah supaya finance + policy tidak ikat-mengikat
// langsung (keep finance pure-Go tanpa yaml dep, keep policy unaware of HTTP
// budget mechanics).
//
// Binary main.go tinggal panggil:
//
//	policybudget.Wire(workspace)
//
// setelah config.LoadDotEnv(workspace). Semua subsequent LLM call via openai
// provider akan consult policy.yaml untuk per-model/per-agent cap.
package policybudget

import (
	"github.com/teetah2402/flowork/internal/finance"
	"github.com/teetah2402/flowork/internal/policy"
)

// Wire memasang resolver yang konsultasi policy.Shared(workspace).MergeLimits
// untuk tiap (model, agent) pair. Idempotent — aman dipanggil beberapa kali.
func Wire(workspace string) {
	finance.Shared().SetLimitsResolver(func(model, agent string) (float64, float64, bool) {
		lim := policy.Shared(workspace).MergeLimits(model, agent)
		return lim.DailyUSDCap, lim.PerTaskUSDCap, lim.ForceFree
	})
}
