package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ClaudeProcess represents a running Claude CLI process
type ClaudeProcess struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	WorkingDir string  `json:"workingDir"`
	CPUPercent float64 `json:"cpuPercent"`
	MemoryMB   float64 `json:"memoryMb"`
	StartTime  int64   `json:"startTime"`
}

// ProcessMonitor tracks Claude processes
type ProcessMonitor struct {
	mu           sync.RWMutex
	prevCPUTimes map[int]cpuTime
	prevSample   time.Time
	clkTck       float64
}

type cpuTime struct {
	utime uint64
	stime uint64
}

// NewProcessMonitor creates a new process monitor
func NewProcessMonitor() *ProcessMonitor {
	return &ProcessMonitor{
		prevCPUTimes: make(map[int]cpuTime),
		prevSample:   time.Now(),
		clkTck:       100.0, // Default clock ticks per second on Linux
	}
}

// GetProcesses returns all running Claude processes
func (pm *ProcessMonitor) GetProcesses() ([]ClaudeProcess, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	now := time.Now()
	elapsed := now.Sub(pm.prevSample).Seconds()
	if elapsed < 0.1 {
		elapsed = 0.1 // Minimum sample interval
	}

	var processes []ClaudeProcess
	currentCPUTimes := make(map[int]cpuTime)
	nameCount := make(map[string]int)

	// First pass: collect all Claude processes
	var rawProcesses []struct {
		proc    ClaudeProcess
		cpuTime cpuTime
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// Check if this is a Claude process
		if !isClaude(pid) {
			continue
		}

		proc := ClaudeProcess{PID: pid}

		// Get working directory
		cwdPath := filepath.Join("/proc", entry.Name(), "cwd")
		if cwd, err := os.Readlink(cwdPath); err == nil {
			proc.WorkingDir = cwd
			proc.Name = filepath.Base(cwd)
		} else {
			proc.Name = "claude"
		}

		// Get memory usage
		proc.MemoryMB = getMemoryMB(pid)

		// Get CPU times
		ct := getCPUTime(pid)
		currentCPUTimes[pid] = ct

		// Get start time
		proc.StartTime = getStartTime(pid)

		rawProcesses = append(rawProcesses, struct {
			proc    ClaudeProcess
			cpuTime cpuTime
		}{proc, ct})
	}

	// Sort by start time for consistent naming
	sort.Slice(rawProcesses, func(i, j int) bool {
		return rawProcesses[i].proc.StartTime < rawProcesses[j].proc.StartTime
	})

	// Second pass: calculate CPU% and assign names
	for _, rp := range rawProcesses {
		proc := rp.proc
		ct := rp.cpuTime

		// Calculate CPU percentage
		if prev, ok := pm.prevCPUTimes[proc.PID]; ok {
			totalDelta := float64((ct.utime - prev.utime) + (ct.stime - prev.stime))
			proc.CPUPercent = (totalDelta / pm.clkTck / elapsed) * 100.0
			if proc.CPUPercent < 0 {
				proc.CPUPercent = 0
			}
			if proc.CPUPercent > 100 {
				proc.CPUPercent = 100
			}
		}

		// Handle duplicate names
		baseName := proc.Name
		nameCount[baseName]++
		if nameCount[baseName] > 1 {
			ordinal := nameCount[baseName]
			suffix := getOrdinalSuffix(ordinal)
			proc.Name = fmt.Sprintf("%s (%s)", baseName, suffix)
		}

		processes = append(processes, proc)
	}

	// Update state
	pm.prevCPUTimes = currentCPUTimes
	pm.prevSample = now

	// Clean up old entries
	for pid := range pm.prevCPUTimes {
		if _, ok := currentCPUTimes[pid]; !ok {
			delete(pm.prevCPUTimes, pid)
		}
	}

	return processes, nil
}

func isClaude(pid int) bool {
	// Check /proc/{pid}/comm for process name
	commPath := filepath.Join("/proc", strconv.Itoa(pid), "comm")
	data, err := os.ReadFile(commPath)
	if err != nil {
		return false
	}

	comm := strings.TrimSpace(string(data))
	return comm == "claude"
}

func getMemoryMB(pid int) float64 {
	statmPath := filepath.Join("/proc", strconv.Itoa(pid), "statm")
	data, err := os.ReadFile(statmPath)
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0
	}

	// Second field is resident set size in pages
	rss, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0
	}

	// Page size is typically 4KB
	pageSize := uint64(os.Getpagesize())
	return float64(rss*pageSize) / (1024 * 1024)
}

func getCPUTime(pid int) cpuTime {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return cpuTime{}
	}

	// Find the closing parenthesis for the command name
	content := string(data)
	idx := strings.LastIndex(content, ")")
	if idx == -1 || idx+2 >= len(content) {
		return cpuTime{}
	}

	fields := strings.Fields(content[idx+2:])
	if len(fields) < 12 {
		return cpuTime{}
	}

	// Fields 11 and 12 (0-indexed) are utime and stime
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)

	return cpuTime{utime: utime, stime: stime}
}

func getStartTime(pid int) int64 {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0
	}

	content := string(data)
	idx := strings.LastIndex(content, ")")
	if idx == -1 || idx+2 >= len(content) {
		return 0
	}

	fields := strings.Fields(content[idx+2:])
	if len(fields) < 20 {
		return 0
	}

	// Field 19 (0-indexed) is starttime in clock ticks since boot
	starttime, _ := strconv.ParseUint(fields[19], 10, 64)

	// Get boot time
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	uptimeFields := strings.Fields(string(uptimeData))
	if len(uptimeFields) < 1 {
		return 0
	}

	uptime, _ := strconv.ParseFloat(uptimeFields[0], 64)
	now := time.Now().Unix()
	bootTime := now - int64(uptime)

	// Convert starttime from clock ticks to seconds
	clkTck := 100.0 // sysconf(_SC_CLK_TCK), usually 100 on Linux
	startSeconds := float64(starttime) / clkTck

	return bootTime + int64(startSeconds)
}

func getOrdinalSuffix(n int) string {
	switch n {
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", n)
	}
}
