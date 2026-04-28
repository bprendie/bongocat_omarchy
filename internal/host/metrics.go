package host

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Metrics struct {
	lastCPUStats []uint64
	lastCPU      float64
	lastRAM      float64
	lastUpdate   time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) Snapshot() (float64, float64) {
	now := time.Now()
	if now.Sub(m.lastUpdate) < time.Second {
		return m.lastCPU, m.lastRAM
	}
	m.lastCPU = m.cpuPercent()
	m.lastRAM = ramPercent()
	m.lastUpdate = now
	return m.lastCPU, m.lastRAM
}

func (m *Metrics) cpuPercent() float64 {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return m.lastCPU
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return m.lastCPU
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, _ := strconv.ParseUint(field, 10, 64)
		values = append(values, value)
	}
	if len(m.lastCPUStats) == 0 {
		m.lastCPUStats = values
		return 0
	}

	var prevTotal, total uint64
	for _, value := range m.lastCPUStats {
		prevTotal += value
	}
	for _, value := range values {
		total += value
	}
	prevIdle := m.lastCPUStats[3]
	idle := values[3]
	m.lastCPUStats = values
	if total <= prevTotal {
		return m.lastCPU
	}
	totalDelta := total - prevTotal
	idleDelta := idle - prevIdle
	return 100.0 * float64(totalDelta-idleDelta) / float64(totalDelta)
}

func ramPercent() float64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var total, available uint64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = value
		case "MemAvailable:":
			available = value
		}
	}
	if total == 0 {
		return 0
	}
	return 100.0 * float64(total-available) / float64(total)
}
