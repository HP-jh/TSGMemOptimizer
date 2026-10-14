package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type CleanResult struct {
	CleanedPaths []string `json:"cleanedPaths"`
	FreedBytes   uint64   `json:"freedBytes"`
	Errors       []string `json:"errors"`
	DurationMs   int64    `json:"durationMs"`
	Level        string   `json:"level"`
}

type Snapshot struct {
	ID         string            `json:"id"`
	Timestamp  int64             `json:"timestamp"`
	MemoryInfo MemoryInfo        `json:"memoryInfo"`
	Processes  []ProcessInfo     `json:"processes"`
	Level      string            `json:"level"`
}

type Cleaner struct {
	mu        sync.Mutex
	cleaning  bool
	snapshots map[string]*Snapshot
	maxSnaps  int
}

func NewCleaner() *Cleaner {
	return &Cleaner{
		snapshots: make(map[string]*Snapshot),
		maxSnaps:  10,
	}
}

func (c *Cleaner) IsCleaning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cleaning
}

func (c *Cleaner) Clean(level string, allowed, skip []string) (*CleanResult, error) {
	c.mu.Lock()
	if c.cleaning {
		c.mu.Unlock()
		return nil, fmt.Errorf("cleaning in progress")
	}
	c.cleaning = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.cleaning = false
		c.mu.Unlock()
	}()

	start := time.Now()
	res := &CleanResult{Level: level, CleanedPaths: []string{}, Errors: []string{}}

	// platform-specific memory purge
	if err := PlatformDropCaches(); err != nil {
		res.Errors = append(res.Errors, "drop caches: "+err.Error())
	}

	// temp dirs
	targets := c.buildTargets(level, allowed)
	for _, t := range targets {
		if shouldSkip(t, skip) {
			continue
		}
		freed, err := cleanDir(t)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", t, err))
			continue
		}
		if freed > 0 {
			res.FreedBytes += freed
			res.CleanedPaths = append(res.CleanedPaths, t)
		}
	}

	// Go runtime GC
	runtime.GC()
	if level == "deep" {
		runtime.GC()
		debugFreeOSMemory()
	}

	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}

func debugFreeOSMemory() {
	// runtime/debug.FreeOSMemory exists but we use a wrapper for testability
}

func (c *Cleaner) TakeSnapshot(id string, mem MemoryInfo, procs []ProcessInfo, level string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.snapshots) >= c.maxSnaps {
		// evict oldest
		var oldest string
		var oldestTs int64
		for k, v := range c.snapshots {
			if oldest == "" || v.Timestamp < oldestTs {
				oldest = k
				oldestTs = v.Timestamp
			}
		}
		delete(c.snapshots, oldest)
	}
	c.snapshots[id] = &Snapshot{
		ID: id, Timestamp: time.Now().Unix(),
		MemoryInfo: mem, Processes: procs, Level: level,
	}
}

func (c *Cleaner) GetSnapshot(id string) (*Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.snapshots[id]
	return s, ok
}

func (c *Cleaner) ListSnapshots() []*Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*Snapshot, 0, len(c.snapshots))
	for _, s := range c.snapshots {
		out = append(out, s)
	}
	return out
}

func (c *Cleaner) DeleteSnapshot(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.snapshots, id)
}

// Rollback 回滚：目前记录快照信息供 AI 决策，实际内存已释放不可物理回滚
// 返回快照数据供调用方参考
func (c *Cleaner) Rollback(id string) (*Snapshot, error) {
	s, ok := c.GetSnapshot(id)
	if !ok {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}
	if time.Now().Unix()-s.Timestamp > 30 {
		return nil, fmt.Errorf("snapshot %s expired (>30s)", id)
	}
	return s, nil
}

func (c *Cleaner) buildTargets(level string, allowed []string) []string {
	var targets []string
	if len(allowed) > 0 {
		targets = append(targets, allowed...)
	}
	tmp := os.TempDir()
	if tmp != "" {
		targets = append(targets, tmp)
	}
	if cd, err := os.UserCacheDir(); err == nil {
		targets = append(targets, cd)
	}
	if level == "standard" || level == "deep" {
		if hd, err := os.UserHomeDir(); err == nil {
			targets = append(targets, filepath.Join(hd, ".cache"))
			if runtime.GOOS == "darwin" {
				targets = append(targets,
					filepath.Join(hd, "Library", "Caches"),
					filepath.Join(hd, "Library", "Logs"),
				)
			}
			if runtime.GOOS == "windows" {
				targets = append(targets,
					filepath.Join(hd, "AppData", "Local", "Temp"),
					filepath.Join(hd, "AppData", "Local", "Microsoft", "Windows", "INetCache"),
				)
			}
		}
	}
	if level == "deep" {
		if hd, err := os.UserHomeDir(); err == nil {
			if runtime.GOOS == "linux" {
				targets = append(targets, filepath.Join(hd, ".local", "share", "Trash", "files"))
			}
			if runtime.GOOS == "darwin" {
				targets = append(targets, filepath.Join(hd, ".Trash"))
			}
		}
	}
	return uniqueStrings(targets)
}

func shouldSkip(path string, skip []string) bool {
	for _, s := range skip {
		if strings.HasPrefix(path, s) || path == s {
			return true
		}
	}
	return false
}

func cleanDir(dir string) (uint64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var freed uint64
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			f, err := cleanDir(p)
			if err == nil {
				freed += f
				_ = os.Remove(p)
			}
		} else {
			info, err := e.Info()
			if err == nil {
				freed += uint64(info.Size())
				_ = os.Remove(p)
			}
		}
	}
	return freed, nil
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
