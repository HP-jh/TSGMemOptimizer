package guardrail

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type Config struct {
	MaxCleanMBPerRun   int      `json:"maxCleanMBPerRun"`   // 单次最大释放 MB
	MaxCleanPerMinute  int      `json:"maxCleanPerMinute"`  // 每分钟最大次数
	MaxCleanPerHour    int      `json:"maxCleanPerHour"`    // 每小时最大次数
	Whitelist          []string `json:"whitelist"`          // 白名单进程名
	AIDefaultLevel     string   `json:"aiDefaultLevel"`     // AI 默认清理级别
	RollbackWindowSec  int      `json:"rollbackWindowSec"`  // 回滚窗口秒数
	SystemProcesses    []string `json:"systemProcesses"`    // 系统进程自动保护
}

var DefaultConfig = &Config{
	MaxCleanMBPerRun:  4096,
	MaxCleanPerMinute: 3,
	MaxCleanPerHour:   10,
	Whitelist:         []string{"tsgmemo", "TarsSecureGuard", "ollama", "llama", "python", "code", "idea"},
	AIDefaultLevel:    "light",
	RollbackWindowSec: 30,
	SystemProcesses:   []string{},
}

type Guardrail struct {
	cfg   *Config
	mu    sync.RWMutex
	hits  []time.Time // 清理动作时间戳
}

func New(cfg *Config) *Guardrail {
	if cfg == nil {
		cfg = DefaultConfig
	} else {
		cfg = mergeDefaults(cfg)
	}
	g := &Guardrail{cfg: cfg, hits: make([]time.Time, 0, 64)}
	g.initSystemProcesses()
	return g
}

func mergeDefaults(c *Config) *Config {
	d := *DefaultConfig
	if c.MaxCleanMBPerRun > 0 {
		d.MaxCleanMBPerRun = c.MaxCleanMBPerRun
	}
	if c.MaxCleanPerMinute > 0 {
		d.MaxCleanPerMinute = c.MaxCleanPerMinute
	}
	if c.MaxCleanPerHour > 0 {
		d.MaxCleanPerHour = c.MaxCleanPerHour
	}
	if c.AIDefaultLevel != "" {
		d.AIDefaultLevel = c.AIDefaultLevel
	}
	if c.RollbackWindowSec > 0 {
		d.RollbackWindowSec = c.RollbackWindowSec
	}
	if len(c.Whitelist) > 0 {
		d.Whitelist = c.Whitelist
	}
	return &d
}

func (g *Guardrail) initSystemProcesses() {
	sys := []string{
		"kernel", "system", "init", "systemd", "launchd", "wininit",
		"explorer", "finder", "dock", "WindowServer", "loginwindow",
		"sshd", "cron", "syslog", "kextd", "kernel_task",
	}
	if runtime.GOOS == "windows" {
		sys = append(sys, "csrss", "services", "lsass", "svchost", "smss")
	}
	g.cfg.SystemProcesses = uniqueStrings(append(g.cfg.SystemProcesses, sys...))
	g.cfg.Whitelist = uniqueStrings(append(g.cfg.Whitelist, g.cfg.SystemProcesses...))
}

func (g *Guardrail) Config() Config {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return *g.cfg
}

func (g *Guardrail) SetConfig(fn func(*Config)) {
	g.mu.Lock()
	defer g.mu.Unlock()
	fn(g.cfg)
}

// CheckClean 检查一次清理是否被允许，返回 (allowed, reason)
func (g *Guardrail) CheckClean(level string, requestedMB int, isAI bool) (bool, string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 1. 资源硬上限
	if requestedMB > g.cfg.MaxCleanMBPerRun {
		return false, fmt.Sprintf("请求释放 %d MB 超过硬上限 %d MB", requestedMB, g.cfg.MaxCleanMBPerRun)
	}

	// 2. 频率限制
	now := time.Now()
	cutoffMin := now.Add(-time.Minute)
	cutoffHour := now.Add(-time.Hour)
	var minCount, hourCount int
	newHits := g.hits[:0]
	for _, t := range g.hits {
		if t.After(cutoffHour) {
			newHits = append(newHits, t)
			if t.After(cutoffMin) {
				minCount++
			}
			hourCount++
		}
	}
	g.hits = newHits

	if minCount >= g.cfg.MaxCleanPerMinute {
		return false, fmt.Sprintf("已达每分钟清理上限 %d 次", g.cfg.MaxCleanPerMinute)
	}
	if hourCount >= g.cfg.MaxCleanPerHour {
		return false, fmt.Sprintf("已达每小时清理上限 %d 次", g.cfg.MaxCleanPerHour)
	}

	// 4. 渐进式清理：AI 默认只能用轻度
	if isAI && level != "light" && level != g.cfg.AIDefaultLevel {
		return false, fmt.Sprintf("AI 调度仅允许 '%s' 级别，请求 '%s' 被拒绝", g.cfg.AIDefaultLevel, level)
	}

	return true, ""
}

func (g *Guardrail) RecordClean() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.hits = append(g.hits, time.Now())
}

func (g *Guardrail) IsWhitelisted(name string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, w := range g.cfg.Whitelist {
		if w == name {
			return true
		}
	}
	return false
}

func (g *Guardrail) Whitelist() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]string, len(g.cfg.Whitelist))
	copy(out, g.cfg.Whitelist)
	return out
}

func (g *Guardrail) AddWhitelist(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, w := range g.cfg.Whitelist {
		if w == name {
			return
		}
	}
	g.cfg.Whitelist = append(g.cfg.Whitelist, name)
}

func (g *Guardrail) RemoveWhitelist(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	for _, w := range g.cfg.Whitelist {
		if w != name {
			out = append(out, w)
		}
	}
	g.cfg.Whitelist = out
}

func (g *Guardrail) Stats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()
	now := time.Now()
	cutoffMin := now.Add(-time.Minute)
	cutoffHour := now.Add(-time.Hour)
	var minCount, hourCount int
	for _, t := range g.hits {
		if t.After(cutoffMin) {
			minCount++
		}
		if t.After(cutoffHour) {
			hourCount++
		}
	}
	return map[string]interface{}{
		"maxCleanMBPerRun":  g.cfg.MaxCleanMBPerRun,
		"maxCleanPerMinute": g.cfg.MaxCleanPerMinute,
		"maxCleanPerHour":   g.cfg.MaxCleanPerHour,
		"aiDefaultLevel":    g.cfg.AIDefaultLevel,
		"rollbackWindowSec": g.cfg.RollbackWindowSec,
		"cleanCountMinute":  minCount,
		"cleanCountHour":    hourCount,
		"whitelistCount":    len(g.cfg.Whitelist),
	}
}

func uniqueStrings(a []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(a))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
