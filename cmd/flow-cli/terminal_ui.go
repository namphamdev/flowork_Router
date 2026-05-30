// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./cmd/flow-cli package — audit pass surface review.

// flow-cli interactive terminal UI.

package main

import (
	"github.com/flowork-os/flowork_Router/cmd/flow-cli/api"
	"github.com/flowork-os/flowork_Router/cmd/flow-cli/menus"
	"github.com/flowork-os/flowork_Router/cmd/flow-cli/utils"
)

func cmdUI(_ []string) error {
	c := api.New(rootURL, apiKey)
	utils.Header("flow_router — interactive control")
	utils.Info("URL: " + rootURL)
	return utils.RunMenu("Main menu", []utils.MenuItem{
		{Label: "Providers", Action: func() error { return menus.Providers(c) }},
		{Label: "API Keys", Action: func() error { return menus.APIKeys(c) }},
		{Label: "Combos", Action: func() error { return menus.Combos(c) }},
		{Label: "Settings", Action: func() error { return menus.Settings(c) }},
		{Label: "CLI Tools", Action: func() error { return menus.CLITools(c) }},
	})
}
