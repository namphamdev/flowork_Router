// Package mesh implements P2P mesh networking for multi-agent communication.
// This file adds automatic fallback to local LLM (Ollama) when cloud providers
// are unreachable, enabling offline-first mesh agent collaboration.
package mesh

import (
	"context"
	"net/http"
	"os"
	"time"
)

// FallbackMode describes the current provider fallback state.
type FallbackMode string

const (
	// FallbackCloud means using cloud providers normally.
	FallbackCloud FallbackMode = "cloud"
	// FallbackLocal means using local Ollama due to offline/unreachable cloud.
	FallbackLocal FallbackMode = "local"
	// FallbackMesh means using a mesh peer's provider.
	FallbackMesh FallbackMode = "mesh"
)

// FallbackConfig configures the auto-fallback behavior.
type FallbackConfig struct {
	// OllamaURL is the local Ollama endpoint (default: http://localhost:11434)
	OllamaURL string
	// OllamaModel is the model to use when falling back to Ollama
	OllamaModel string
	// ProbeInterval is how often to check cloud connectivity
	ProbeInterval time.Duration
	// ProbeTimeout is the max time to wait for a cloud probe
	ProbeTimeout time.Duration
}

// DefaultFallbackConfig returns sensible defaults.
func DefaultFallbackConfig() FallbackConfig {
	url := os.Getenv("OLLAMA_HOST")
	if url == "" {
		url = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2"
	}
	return FallbackConfig{
		OllamaURL:     url,
		OllamaModel:   model,
		ProbeInterval: 30 * time.Second,
		ProbeTimeout:  5 * time.Second,
	}
}

// FallbackManager manages automatic switching between cloud, local, and mesh providers.
type FallbackManager struct {
	config      FallbackConfig
	currentMode FallbackMode
	lastProbe   time.Time
	ollamaOK    bool
	cloudOK     bool
}

// NewFallbackManager creates a new fallback manager.
func NewFallbackManager(config FallbackConfig) *FallbackManager {
	return &FallbackManager{
		config:      config,
		currentMode: FallbackCloud,
	}
}

// Mode returns the current fallback mode.
func (fm *FallbackManager) Mode() FallbackMode {
	return fm.currentMode
}

// ProbeAndSwitch checks connectivity and switches providers accordingly.
// Call this periodically (e.g., before each turn) to auto-switch.
func (fm *FallbackManager) ProbeAndSwitch(ctx context.Context) FallbackMode {
	now := time.Now()
	if now.Sub(fm.lastProbe) < fm.config.ProbeInterval {
		return fm.currentMode
	}
	fm.lastProbe = now

	// Check cloud connectivity by probing a known endpoint
	fm.cloudOK = fm.probeCloud(ctx)
	fm.ollamaOK = fm.probeOllama(ctx)

	switch {
	case fm.cloudOK:
		fm.currentMode = FallbackCloud
	case fm.ollamaOK:
		fm.currentMode = FallbackLocal
	default:
		// Try mesh peers (if available through the mesh transport layer)
		fm.currentMode = FallbackMesh
	}

	return fm.currentMode
}

// probeCloud checks if we can reach the internet (using a lightweight HEAD).
func (fm *FallbackManager) probeCloud(ctx context.Context) bool {
	client := &http.Client{Timeout: fm.config.ProbeTimeout}
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://api.openai.com", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// probeOllama checks if the local Ollama server is running.
func (fm *FallbackManager) probeOllama(ctx context.Context) bool {
	client := &http.Client{Timeout: fm.config.ProbeTimeout}
	req, err := http.NewRequestWithContext(ctx, "GET", fm.config.OllamaURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ShouldUseOllama returns whether the agent should use Ollama right now.
func (fm *FallbackManager) ShouldUseOllama() bool {
	return fm.currentMode == FallbackLocal
}

// ShouldUseMesh returns whether the agent should try mesh peers.
func (fm *FallbackManager) ShouldUseMesh() bool {
	return fm.currentMode == FallbackMesh
}

// GetOllamaModel returns the configured Ollama model name.
func (fm *FallbackManager) GetOllamaModel() string {
	return fm.config.OllamaModel
}
