package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"claude-monitor/internal/monitor"
)

// Settings represents user-configurable alert settings
type Settings struct {
	CPUThreshold  float64 `json:"cpuThreshold"`
	TempThreshold float64 `json:"tempThreshold"`
	AlertsEnabled bool    `json:"alertsEnabled"`
}

// DefaultSettings returns default settings
func DefaultSettings() Settings {
	return Settings{
		CPUThreshold:  90.0,
		TempThreshold: 85.0,
		AlertsEnabled: true,
	}
}

// Handler holds all API handlers
type Handler struct {
	processMonitor *monitor.ProcessMonitor
	tempMonitor    *monitor.TemperatureMonitor
	history        *monitor.HistoryBuffer
	settings       Settings
	settingsPath   string
}

// NewHandler creates a new API handler
func NewHandler(pm *monitor.ProcessMonitor, tm *monitor.TemperatureMonitor, hb *monitor.HistoryBuffer) *Handler {
	h := &Handler{
		processMonitor: pm,
		tempMonitor:    tm,
		history:        hb,
		settings:       DefaultSettings(),
	}

	// Set up settings path
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.Getenv("HOME")
	}
	h.settingsPath = filepath.Join(configDir, "claude-monitor", "settings.json")

	// Load settings
	h.loadSettings()

	return h
}

// RegisterRoutes registers all API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/processes", h.handleProcesses)
	mux.HandleFunc("/api/temperature", h.handleTemperature)
	mux.HandleFunc("/api/history", h.handleHistory)
	mux.HandleFunc("/api/kill/", h.handleKill)
	mux.HandleFunc("/api/settings", h.handleSettings)
}

func (h *Handler) handleProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	processes, err := h.processMonitor.GetProcesses()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(processes)
}

func (h *Handler) handleTemperature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	temps := h.tempMonitor.GetTemperatures()

	response := struct {
		Temperatures []monitor.Temperature `json:"temperatures"`
		MainTemp     float64               `json:"mainTemp"`
	}{
		Temperatures: temps,
		MainTemp:     h.tempMonitor.GetMainTemperature(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history := h.history.GetAll()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func (h *Handler) handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract PID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/kill/")
	pid, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid PID", http.StatusBadRequest)
		return
	}

	// Send SIGTERM
	process, err := os.FindProcess(pid)
	if err != nil {
		http.Error(w, "Process not found", http.StatusNotFound)
		return
	}

	err = process.Signal(syscall.SIGTERM)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "SIGTERM sent to process " + strconv.Itoa(pid),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.settings)

	case http.MethodPost:
		var newSettings Settings
		if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		h.settings = newSettings
		h.saveSettings()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(h.settings)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) loadSettings() {
	data, err := os.ReadFile(h.settingsPath)
	if err != nil {
		return // Use defaults
	}

	json.Unmarshal(data, &h.settings)
}

func (h *Handler) saveSettings() {
	dir := filepath.Dir(h.settingsPath)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(h.settings, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(h.settingsPath, data, 0644)
}

// GetSettings returns current settings
func (h *Handler) GetSettings() Settings {
	return h.settings
}

// RecordHistory records a history point
func (h *Handler) RecordHistory() {
	processes, err := h.processMonitor.GetProcesses()
	if err != nil {
		return
	}

	var snapshots []monitor.ProcessSnapshot
	for _, p := range processes {
		snapshots = append(snapshots, monitor.ProcessSnapshot{
			PID:        p.PID,
			Name:       p.Name,
			CPUPercent: p.CPUPercent,
			MemoryMB:   p.MemoryMB,
		})
	}

	point := monitor.HistoryPoint{
		Timestamp:   time.Now().Unix(),
		Temperature: h.tempMonitor.GetMainTemperature(),
		Processes:   snapshots,
	}

	h.history.Add(point)
}

// CheckAlerts checks if any alerts should be triggered
func (h *Handler) CheckAlerts() (cpuAlert bool, tempAlert bool, alertProcess string) {
	if !h.settings.AlertsEnabled {
		return false, false, ""
	}

	// Check temperature
	temp := h.tempMonitor.GetMainTemperature()
	if temp >= h.settings.TempThreshold {
		tempAlert = true
	}

	// Check CPU
	processes, _ := h.processMonitor.GetProcesses()
	for _, p := range processes {
		if p.CPUPercent >= h.settings.CPUThreshold {
			cpuAlert = true
			alertProcess = p.Name
			break
		}
	}

	return
}
