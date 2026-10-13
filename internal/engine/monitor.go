package engine

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	Available   uint64  `json:"available"`
	Cached      uint64  `json:"cached"`
	Buffers     uint64  `json:"buffers"`
	SwapTotal   uint64  `json:"swapTotal"`
	SwapUsed    uint64  `json:"swapUsed"`
	UsagePercent float64 `json:"usagePercent"`
	Load        [3]float64 `json:"load"`
	Timestamp   int64   `json:"timestamp"`
}

type Monitor struct {
	mu       sync.RWMutex
	latest   MemoryInfo
	history  []MemoryInfo
	maxHist  int
	ticker   *time.Ticker
	stop     chan struct{}
	started  bool
}

func NewMonitor() *Monitor {
	return &Monitor{
		history: make([]MemoryInfo, 0, 180),
		maxHist: 180,
		stop:    make(chan struct{}),
	}
}

func (m *Monitor) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.ticker = time.NewTicker(2 * time.Second)
	m.mu.Unlock()

	_ = m.sample()
	go func() {
		for {
			select {
			case <-m.ticker.C:
				_ = m.sample()
			case <-m.stop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started {
		return
	}
	m.started = false
	m.ticker.Stop()
	close(m.stop)
}

func (m *Monitor) sample() error {
	info, err := GetSystemMemory()
	if err != nil {
		return err
	}
	info.Load = GetLoad()
	info.Timestamp = time.Now().Unix()

	m.mu.Lock()
	m.latest = info
	m.history = append(m.history, info)
	if len(m.history) > m.maxHist {
		m.history = m.history[len(m.history)-m.maxHist:]
	}
	m.mu.Unlock()
	return nil
}

func (m *Monitor) Latest() MemoryInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latest
}

func (m *Monitor) History() []MemoryInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]MemoryInfo, len(m.history))
	copy(out, m.history)
	return out
}

func FormatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func GetGoMemory() map[string]interface{} {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return map[string]interface{}{
		"alloc":       ms.Alloc,
		"totalAlloc":  ms.TotalAlloc,
		"sys":         ms.Sys,
		"numGC":       ms.NumGC,
		"heapAlloc":   ms.HeapAlloc,
		"heapSys":     ms.HeapSys,
		"heapInuse":   ms.HeapInuse,
		"heapIdle":    ms.HeapIdle,
		"heapReleased": ms.HeapReleased,
	}
}
