package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

type ProcessInfo struct {
	PID       int     `json:"pid"`
	Name      string  `json:"name"`
	RSS       uint64  `json:"rss"`       // bytes
	RSSMB     float64 `json:"rssMB"`
	CPU       float64 `json:"cpu"`       // percent
	Command   string  `json:"command"`
	Priority  int     `json:"priority"`
}

type ProcessManager struct {
	mu       sync.RWMutex
	list     []ProcessInfo
	ticker   *time.Ticker
	stop     chan struct{}
	started  bool
}

func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		stop: make(chan struct{}),
	}
}

func (pm *ProcessManager) Start(ctx context.Context) {
	pm.mu.Lock()
	if pm.started {
		pm.mu.Unlock()
		return
	}
	pm.started = true
	pm.ticker = time.NewTicker(5 * time.Second)
	pm.mu.Unlock()

	_ = pm.refresh()
	go func() {
		var done <-chan struct{}
		if ctx != nil {
			done = ctx.Done()
		}
		for {
			select {
			case <-pm.ticker.C:
				_ = pm.refresh()
			case <-pm.stop:
				return
			case <-done:
				return
			}
		}
	}()
}

func (pm *ProcessManager) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if !pm.started {
		return
	}
	pm.started = false
	pm.ticker.Stop()
	close(pm.stop)
}

func (pm *ProcessManager) refresh() error {
	list, err := ListProcesses()
	if err != nil {
		return err
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].RSS > list[j].RSS
	})
	pm.mu.Lock()
	pm.list = list
	pm.mu.Unlock()
	return nil
}

func (pm *ProcessManager) List() []ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make([]ProcessInfo, len(pm.list))
	copy(out, pm.list)
	return out
}

func (pm *ProcessManager) Top(n int) []ProcessInfo {
	all := pm.List()
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

func (pm *ProcessManager) Kill(pid int) error {
	return KillProcess(pid)
}

func (pm *ProcessManager) AutoKill(thresholdMB int, whitelist []string) ([]int, error) {
	all := pm.List()
	var killed []int
	for _, p := range all {
		if p.PID == 0 {
			continue
		}
		if p.RSSMB < float64(thresholdMB) {
			continue
		}
		skipped := false
		for _, w := range whitelist {
			if p.Name == w {
				skipped = true
				break
			}
		}
		if skipped {
			continue
		}
		if err := pm.Kill(p.PID); err == nil {
			killed = append(killed, p.PID)
		}
	}
	return killed, nil
}

func (pm *ProcessManager) Summary() map[string]interface{} {
	all := pm.List()
	var totalRSS uint64
	for _, p := range all {
		totalRSS += p.RSS
	}
	return map[string]interface{}{
		"count":    len(all),
		"totalRSS": totalRSS,
		"totalRSSMB": fmt.Sprintf("%.0f", float64(totalRSS)/1024/1024),
	}
}
