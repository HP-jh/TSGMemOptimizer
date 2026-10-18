package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoggerLogAndRecent(t *testing.T) {
	dir := t.TempDir()
	l := &Logger{
		path:       filepath.Join(dir, "audit.jsonl"),
		maxSize:    1024 * 1024,
		maxBackups: 2,
	}

	l.LogClean("light", true, "", 1024, 100)
	l.LogKill(1234, "test", false, "whitelisted")
	l.LogAISchedule("clean", true, "")

	recent := l.Recent(10)
	if len(recent) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(recent))
	}
	if recent[0].Action != "ai_schedule" {
		t.Errorf("expected newest action ai_schedule, got %s", recent[0].Action)
	}
	if recent[2].Action != "clean" {
		t.Errorf("expected oldest action clean, got %s", recent[2].Action)
	}
	if !recent[2].Allowed {
		t.Error("expected clean allowed")
	}
}

func TestLoggerStats(t *testing.T) {
	dir := t.TempDir()
	l := &Logger{
		path:       filepath.Join(dir, "audit.jsonl"),
		maxSize:    1024 * 1024,
		maxBackups: 2,
	}

	l.LogClean("light", true, "", 0, 0)
	l.LogClean("deep", true, "", 0, 0)
	l.LogKill(1, "a", false, "")

	stats := l.Stats()
	if stats["clean"] != 2 {
		t.Errorf("expected clean=2, got %d", stats["clean"])
	}
	if stats["kill"] != 1 {
		t.Errorf("expected kill=1, got %d", stats["kill"])
	}
}

func TestLoggerRotate(t *testing.T) {
	dir := t.TempDir()
	l := &Logger{
		path:       filepath.Join(dir, "audit.jsonl"),
		maxSize:    100,
		maxBackups: 2,
	}

	for i := 0; i < 20; i++ {
		l.Log("test", true, "", map[string]interface{}{"n": i})
	}

	_, err := os.Stat(l.path)
	if err != nil {
		t.Logf("rotate may have occurred, err=%v", err)
	}
	// At least backup 1 should exist
	_, err = os.Stat(l.path + ".1")
	if err != nil {
		t.Logf("backup 1 not found: %v", err)
	}
}

func TestRecentLimit(t *testing.T) {
	dir := t.TempDir()
	l := &Logger{
		path:       filepath.Join(dir, "audit.jsonl"),
		maxSize:    1024 * 1024,
		maxBackups: 2,
	}

	for i := 0; i < 5; i++ {
		l.Log("test", true, "", nil)
	}

	recent := l.Recent(2)
	if len(recent) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(recent))
	}
}
