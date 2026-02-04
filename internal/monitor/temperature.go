package monitor

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Temperature represents a temperature reading
type Temperature struct {
	Label   string  `json:"label"`
	Current float64 `json:"current"`
	High    float64 `json:"high,omitempty"`
	Crit    float64 `json:"crit,omitempty"`
}

// TemperatureMonitor reads system temperatures
type TemperatureMonitor struct{}

// NewTemperatureMonitor creates a new temperature monitor
func NewTemperatureMonitor() *TemperatureMonitor {
	return &TemperatureMonitor{}
}

// GetTemperatures returns all available temperature readings
func (tm *TemperatureMonitor) GetTemperatures() []Temperature {
	// Try sensors command first
	temps := tm.getSensorsTemperatures()
	if len(temps) > 0 {
		return temps
	}

	// Fallback to sysfs
	return tm.getSysfsTemperatures()
}

// GetMainTemperature returns the main CPU temperature
func (tm *TemperatureMonitor) GetMainTemperature() float64 {
	temps := tm.GetTemperatures()

	// Priority order for main temp
	priorities := []string{"Tctl", "Tdie", "Package", "Core 0", "CPU", "temp1"}

	for _, prio := range priorities {
		for _, t := range temps {
			if strings.Contains(t.Label, prio) {
				return t.Current
			}
		}
	}

	// Return first temp if no priority match
	if len(temps) > 0 {
		return temps[0].Current
	}

	return 0
}

func (tm *TemperatureMonitor) getSensorsTemperatures() []Temperature {
	cmd := exec.Command("sensors", "-u")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	return parseSensorsOutput(string(output))
}

func parseSensorsOutput(output string) []Temperature {
	var temps []Temperature
	lines := strings.Split(output, "\n")

	var currentLabel string
	var currentTemp Temperature

	// Patterns for parsing sensors -u output
	labelPattern := regexp.MustCompile(`^([^:]+):$`)
	tempInputPattern := regexp.MustCompile(`^\s+temp\d+_input:\s+([\d.]+)$`)
	tempMaxPattern := regexp.MustCompile(`^\s+temp\d+_max:\s+([\d.]+)$`)
	tempCritPattern := regexp.MustCompile(`^\s+temp\d+_crit:\s+([\d.]+)$`)

	for _, line := range lines {
		// Check for label line
		if matches := labelPattern.FindStringSubmatch(line); len(matches) > 1 {
			// Save previous temp if valid
			if currentLabel != "" && currentTemp.Current > 0 {
				currentTemp.Label = currentLabel
				temps = append(temps, currentTemp)
			}
			currentLabel = matches[1]
			currentTemp = Temperature{}
			continue
		}

		// Check for temperature input
		if matches := tempInputPattern.FindStringSubmatch(line); len(matches) > 1 {
			val, _ := strconv.ParseFloat(matches[1], 64)
			if val > 0 && val < 150 { // Sanity check
				currentTemp.Current = val
			}
			continue
		}

		// Check for max temperature
		if matches := tempMaxPattern.FindStringSubmatch(line); len(matches) > 1 {
			val, _ := strconv.ParseFloat(matches[1], 64)
			currentTemp.High = val
			continue
		}

		// Check for critical temperature
		if matches := tempCritPattern.FindStringSubmatch(line); len(matches) > 1 {
			val, _ := strconv.ParseFloat(matches[1], 64)
			currentTemp.Crit = val
			continue
		}
	}

	// Don't forget the last one
	if currentLabel != "" && currentTemp.Current > 0 {
		currentTemp.Label = currentLabel
		temps = append(temps, currentTemp)
	}

	return temps
}

func (tm *TemperatureMonitor) getSysfsTemperatures() []Temperature {
	var temps []Temperature

	// Find thermal zones
	zones, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err != nil {
		return nil
	}

	for _, zonePath := range zones {
		data, err := os.ReadFile(zonePath)
		if err != nil {
			continue
		}

		tempVal, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
		if err != nil {
			continue
		}

		// Temperature is in millidegrees
		tempVal /= 1000.0

		// Get zone name
		zoneDir := filepath.Dir(zonePath)
		typeData, _ := os.ReadFile(filepath.Join(zoneDir, "type"))
		label := strings.TrimSpace(string(typeData))
		if label == "" {
			label = filepath.Base(zoneDir)
		}

		temps = append(temps, Temperature{
			Label:   label,
			Current: tempVal,
		})
	}

	return temps
}
