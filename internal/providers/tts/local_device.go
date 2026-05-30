// === LOCKED FILE ===
// Status: STABLE — DO NOT MODIFY without owner approval.
// Owner: Aola Sahidin (Mr.Dev)
// Repo: https://github.com/flowork-os/flowork_Router
// Locked at: 2026-05-30
// Reason: Audit pass — ./internal/providers/tts package — audit pass surface review.

// Vendor: localDevice — system-native TTS (say/espeak/PowerShell SpeechSynthesizer).
// On the server itself this generally is not useful; the local TTS executes on
// the *router's* machine. Kept for parity with upstream so dashboard configs map.
package tts

import (
	"context"
	"errors"
)

func init() { Register(&localDeviceProvider{}) }

type localDeviceProvider struct{}

func (l *localDeviceProvider) Name() string { return "localDevice" }

func (l *localDeviceProvider) Speak(ctx context.Context, req Request) ([]byte, string, error) {
	// The upstream pattern shells out to OS TTS; in flow_router we mark this
	// vendor as available but expect the dashboard to render audio on the
	// browser side via the Web Speech API instead of the server.
	return nil, "", errors.New("localDevice TTS is browser-rendered; route via dashboard's Web Speech API")
}
