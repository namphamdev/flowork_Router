// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — audit pass surface review.

// flow-tray native build (CGO + systray).

//go:build cgo_tray
// +build cgo_tray

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Body is intentionally minimal. The reference template below shows how the
// real binary integrates with github.com/getlantern/systray once the dep is
// vendored. Until then this build target compiles to a no-op so the
// `-tags cgo_tray` flag is at least valid.
func main() {
	url := os.Getenv("FLOW_ROUTER_URL")
	if url == "" {
		url = "http://127.0.0.1:2402"
	}
	fmt.Fprintln(os.Stderr, "flow-tray native build — dependency stub")
	fmt.Fprintln(os.Stderr, "Run: go get github.com/getlantern/systray && rebuild")
	fmt.Fprintln(os.Stderr, "Opening dashboard:", url)
	_ = exec.Command("xdg-open", url).Start()
	_ = exec.Command("open", url).Start()
}

/* Reference template (uncomment once dep is vendored):

import (
    "github.com/getlantern/systray"
)

func main() {
    systray.Run(onReady, onExit)
}

func onReady() {
    systray.SetTitle("flow_router")
    systray.SetTooltip("flow_router AI gateway")
    mOpen := systray.AddMenuItem("Open dashboard", "Launch the web UI")
    mStatus := systray.AddMenuItem("Check status", "Probe /api/health")
    systray.AddSeparator()
    mQuit := systray.AddMenuItem("Quit", "Exit the tray app")
    go func() {
        for {
            select {
            case <-mOpen.ClickedCh:
                openURL()
            case <-mStatus.ClickedCh:
                showStatus()
            case <-mQuit.ClickedCh:
                systray.Quit()
                return
            }
        }
    }()
}

func onExit() {}

func openURL() { _ = exec.Command("xdg-open", "http://127.0.0.1:2402").Start() }
func showStatus() { ... }
*/
