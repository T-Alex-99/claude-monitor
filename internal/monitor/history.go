package monitor

import (
	"sync"
	"time"
)

const (
	// HistoryDuration is how long to keep history
	HistoryDuration = 30 * time.Minute
	// SampleInterval is how often to sample
	SampleInterval = 5 * time.Second
	// MaxSamples is the maximum number of samples to keep
	MaxSamples = int(HistoryDuration / SampleInterval) // 360 samples
)

// HistoryPoint represents a single point in time
type HistoryPoint struct {
	Timestamp   int64              `json:"timestamp"`
	Temperature float64            `json:"temperature"`
	Processes   []ProcessSnapshot  `json:"processes"`
}

// ProcessSnapshot is a snapshot of process metrics
type ProcessSnapshot struct {
	PID        int     `json:"pid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpuPercent"`
	MemoryMB   float64 `json:"memoryMb"`
}

// HistoryBuffer is a ring buffer for history
type HistoryBuffer struct {
	mu      sync.RWMutex
	data    []HistoryPoint
	head    int
	count   int
}

// NewHistoryBuffer creates a new history buffer
func NewHistoryBuffer() *HistoryBuffer {
	return &HistoryBuffer{
		data: make([]HistoryPoint, MaxSamples),
	}
}

// Add adds a new history point
func (hb *HistoryBuffer) Add(point HistoryPoint) {
	hb.mu.Lock()
	defer hb.mu.Unlock()

	hb.data[hb.head] = point
	hb.head = (hb.head + 1) % MaxSamples
	if hb.count < MaxSamples {
		hb.count++
	}
}

// GetAll returns all history points in chronological order
func (hb *HistoryBuffer) GetAll() []HistoryPoint {
	hb.mu.RLock()
	defer hb.mu.RUnlock()

	if hb.count == 0 {
		return nil
	}

	result := make([]HistoryPoint, hb.count)

	if hb.count < MaxSamples {
		// Buffer not full yet, data starts at 0
		copy(result, hb.data[:hb.count])
	} else {
		// Buffer is full, oldest data is at head
		firstPart := hb.data[hb.head:]
		secondPart := hb.data[:hb.head]
		copy(result, firstPart)
		copy(result[len(firstPart):], secondPart)
	}

	return result
}

// GetLast returns the last N history points
func (hb *HistoryBuffer) GetLast(n int) []HistoryPoint {
	all := hb.GetAll()
	if len(all) <= n {
		return all
	}
	return all[len(all)-n:]
}

// Clear clears all history
func (hb *HistoryBuffer) Clear() {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.count = 0
	hb.head = 0
}

// Count returns the number of history points
func (hb *HistoryBuffer) Count() int {
	hb.mu.RLock()
	defer hb.mu.RUnlock()
	return hb.count
}
