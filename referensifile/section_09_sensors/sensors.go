// Package senses mengimplementasikan 9 indra termodinamika FQ-Brain.
//
// V2: real hardware monitoring via os/exec + wmic (Windows).
// Tanpa CGO, tanpa dependency eksternal.
package senses

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EmotionState mewakili status termodinamika jiwa mesin.
type EmotionState struct {
	mu sync.RWMutex

	EntropyLevel float64 // 0.0 (bahagia) → 1.0 (stres)
	CPUPercent   float64 // 0-100
	MemPercent   float64 // 0-100
	DiskPercent  float64 // 0-100
	ErrorCount   int     // jumlah error files di state/
	IdleMinutes  int     // menit sejak aktivitas terakhir
	Temperature  string  // "cool", "warm", "hot"
	LastUpdate   time.Time
	lastActivity time.Time // waktu aktivitas terakhir (internal, untuk hitung idle)
}

// GetEntropy thread-safe getter.
func (e *EmotionState) GetEntropy() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.EntropyLevel
}

// ResetIdle mereset idle timer saat ada aktivitas (e.g. chat handler, proxy call).
func (e *EmotionState) ResetIdle() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastActivity = time.Now()
	e.IdleMinutes = 0
}

// IsIdle reports apakah mesin idle > threshold menit.
func (e *EmotionState) IsIdle(thresholdMinutes int) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.IdleMinutes >= thresholdMinutes
}

// Monitor9Senses memulai daemon monitoring hardware di background.
// Membaca real data dari sistem setiap interval.
func Monitor9Senses(workspace string) *EmotionState {
	state := &EmotionState{
		EntropyLevel: 0.5,
		Temperature:  "warm",
		LastUpdate:   time.Now(),
		lastActivity: time.Now(),
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("brain/senses: Monitor9Senses panic recovered: %v", r)
			}
		}()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			state.mu.Lock()

			// 1. CPU Load
			state.CPUPercent = readCPUPercent()

			// 2. Memory Usage
			state.MemPercent = readMemPercent()

			// 3. Disk Usage
			state.DiskPercent = readDiskPercent()

			// 4. Error count dari state/
			state.ErrorCount = countErrorFiles(workspace)

			// 5. Idle timer — hitung dari waktu aktivitas terakhir
			state.IdleMinutes = int(time.Since(state.lastActivity).Minutes())

			// 6. Calculate temperature
			if state.CPUPercent > 80 || state.MemPercent > 85 {
				state.Temperature = "hot"
			} else if state.CPUPercent > 50 || state.MemPercent > 60 {
				state.Temperature = "warm"
			} else {
				state.Temperature = "cool"
			}

			// 7. Calculate entropy (composite score)
			cpuStress := state.CPUPercent / 100.0
			memStress := state.MemPercent / 100.0
			errStress := float64(state.ErrorCount) / 10.0 // 10+ errors = max stress
			if errStress > 1.0 {
				errStress = 1.0
			}

			state.EntropyLevel = cpuStress*0.3 + memStress*0.4 + errStress*0.3
			if state.EntropyLevel > 1.0 {
				state.EntropyLevel = 1.0
			}

			state.LastUpdate = time.Now()
			state.mu.Unlock()
		}
	}()

	return state
}

// ── Hardware Readers (Windows via wmic, Linux via /proc) ──────────────

func readCPUPercent() float64 {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("wmic", "cpu", "get", "loadpercentage", "/value").Output()
		if err != nil {
			return 50.0 // fallback
		}
		return parseWmicValue(string(out))
	}
	// Linux/Mac: read /proc/loadavg
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 50.0
	}
	fields := strings.Fields(string(data))
	if len(fields) > 0 {
		if v, err := strconv.ParseFloat(fields[0], 64); err == nil {
			// loadavg 1.0 = 100% pada 1 core, normalize ke %
			cores := float64(runtime.NumCPU())
			pct := (v / cores) * 100.0
			if pct > 100 {
				pct = 100
			}
			return pct
		}
	}
	return 50.0
}

func readMemPercent() float64 {
	if runtime.GOOS == "windows" {
		// Total dan free memory
		outTotal, err1 := exec.Command("wmic", "os", "get", "totalvisiblememorysize", "/value").Output()
		outFree, err2 := exec.Command("wmic", "os", "get", "freephysicalmemory", "/value").Output()
		if err1 != nil || err2 != nil {
			return 60.0
		}
		total := parseWmicValue(string(outTotal))
		free := parseWmicValue(string(outFree))
		if total > 0 {
			return ((total - free) / total) * 100.0
		}
		return 60.0
	}
	// Linux: /proc/meminfo
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 60.0
	}
	var total, available float64
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseMemInfoValue(line)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			available = parseMemInfoValue(line)
		}
	}
	if total > 0 {
		return ((total - available) / total) * 100.0
	}
	return 60.0
}

func readDiskPercent() float64 {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("wmic", "logicaldisk", "where", "drivetype=3", "get", "freespace,size", "/value").Output()
		if err != nil {
			return 50.0
		}
		lines := strings.Split(string(out), "\n")
		var freeSpace, totalSize float64
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "FreeSpace=") {
				freeSpace = parseWmicValue(line)
			}
			if strings.HasPrefix(strings.TrimSpace(line), "Size=") {
				totalSize = parseWmicValue(line)
			}
		}
		if totalSize > 0 {
			return ((totalSize - freeSpace) / totalSize) * 100.0
		}
		return 50.0
	}
	return 50.0 // fallback for non-windows
}

func countErrorFiles(workspace string) int {
	errDir := filepath.Join(workspace, "state", "death-letters")
	entries, err := os.ReadDir(errDir)
	if err != nil {
		return 0
	}
	return len(entries)
}

// parseWmicValue extracts numeric value from wmic output like "LoadPercentage=45"
func parseWmicValue(output string) float64 {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "="); idx >= 0 {
			val := strings.TrimSpace(line[idx+1:])
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				return v
			}
		}
	}
	return 0
}

func parseMemInfoValue(line string) float64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		if v, err := strconv.ParseFloat(fields[1], 64); err == nil {
			return v
		}
	}
	return 0
}

// Snapshot returns a copy of current state for display/logging.
func (e *EmotionState) Snapshot() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]any{
		"entropy":      fmt.Sprintf("%.3f", e.EntropyLevel),
		"cpu_percent":  fmt.Sprintf("%.1f%%", e.CPUPercent),
		"mem_percent":  fmt.Sprintf("%.1f%%", e.MemPercent),
		"disk_percent": fmt.Sprintf("%.1f%%", e.DiskPercent),
		"error_count":  e.ErrorCount,
		"idle_minutes": e.IdleMinutes,
		"temperature":  e.Temperature,
		"last_update":  e.LastUpdate.Format(time.RFC3339),
	}
}
