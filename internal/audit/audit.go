// Package audit provides operation audit logging for TSGMemOptimizer.
// Records every clean, kill, AI schedule, config change with timestamp and result.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry represents a single audit log record.
type Entry struct {
	Timestamp int64                  `json:"timestamp"`
	Action    string                 `json:"action"`    // clean, kill, ai_schedule, config_change, rollback
	Level     string                 `json:"level,omitempty"`
	Allowed   bool                   `json:"allowed"`
	Reason    string                 `json:"reason,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// Logger writes audit entries to a rotating log file.
type Logger struct {
	mu       sync.Mutex
	path     string
	maxSize  int64 // bytes
	maxBackups int
}

// New creates an audit logger with defaults.
func New() *Logger {
	d, _ := os.UserConfigDir()
	return &Logger{
		path:       filepath.Join(d, "TSGMemOptimizer", "audit.jsonl"),
		maxSize:    2 * 1024 * 1024, // 2MB
		maxBackups: 3,
	}
}

// Log writes an audit entry immediately to disk.
func (l *Logger) Log(action string, allowed bool, reason string, data map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ent := Entry{
		Timestamp: time.Now().Unix(),
		Action:    action,
		Allowed:   allowed,
		Reason:    reason,
		Data:      data,
	}
	b, _ := json.Marshal(ent)
	b = append(b, '\n')

	_ = os.MkdirAll(filepath.Dir(l.path), 0755)
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(b)

	l.rotateIfNeeded()
}

// LogClean is a convenience wrapper for clean actions.
func (l *Logger) LogClean(level string, allowed bool, reason string, freedBytes uint64, durationMs int64) {
	l.Log("clean", allowed, reason, map[string]interface{}{
		"level":      level,
		"freedBytes": freedBytes,
		"durationMs": durationMs,
	})
}

// LogKill is a convenience wrapper for kill actions.
func (l *Logger) LogKill(pid int, name string, allowed bool, reason string) {
	l.Log("kill", allowed, reason, map[string]interface{}{
		"pid":  pid,
		"name": name,
	})
}

// LogAISchedule is a convenience wrapper for AI schedule actions.
func (l *Logger) LogAISchedule(intent string, allowed bool, reason string) {
	l.Log("ai_schedule", allowed, reason, map[string]interface{}{
		"intent": intent,
	})
}

// LogRollback records a rollback attempt.
func (l *Logger) LogRollback(id string, success bool, reason string) {
	l.Log("rollback", success, reason, map[string]interface{}{
		"snapshotID": id,
	})
}

// Recent returns the last N entries (newest first).
func (l *Logger) Recent(n int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil
	}
	var entries []Entry
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err == nil {
			entries = append(entries, e)
		}
	}
	if n > 0 && len(entries) > n {
		entries = entries[len(entries)-n:]
	}
	// reverse
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

// Stats returns summary counts by action.
func (l *Logger) Stats() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil
	}
	stats := make(map[string]int)
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err == nil {
			stats[e.Action]++
		}
	}
	return stats
}

func (l *Logger) rotateIfNeeded() {
	info, err := os.Stat(l.path)
	if err != nil || info.Size() < l.maxSize {
		return
	}
	// simple rotation: audit.jsonl -> audit.jsonl.1 -> audit.jsonl.2 ...
	for i := l.maxBackups - 1; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d", l.path, i)
		newPath := fmt.Sprintf("%s.%d", l.path, i+1)
		_ = os.Rename(oldPath, newPath)
	}
	_ = os.Rename(l.path, l.path+".1")
}

func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			out = append(out, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}
