package engine

import (
	"context"
	"testing"
	"time"
)

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in  uint64
		out string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.out {
			t.Errorf("FormatBytes(%d) = %s, want %s", c.in, got, c.out)
		}
	}
}

func TestMonitorSample(t *testing.T) {
	m := NewMonitor()
	if err := m.sample(); err != nil {
		t.Skipf("sample requires platform support: %v", err)
	}
	latest := m.Latest()
	if latest.Total == 0 {
		t.Error("expected non-zero total memory")
	}
	hist := m.History()
	if len(hist) == 0 {
		t.Error("expected history after sample")
	}
}

func TestMonitorStartStop(t *testing.T) {
	m := NewMonitor()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	m.Stop()
	if m.started {
		t.Error("expected monitor stopped")
	}
}

func TestCleanerBuildTargets(t *testing.T) {
	c := NewCleaner()
	targets := c.buildTargets("light", nil)
	if len(targets) == 0 {
		t.Error("expected some light targets")
	}
	targets = c.buildTargets("deep", nil)
	if len(targets) == 0 {
		t.Error("expected some deep targets")
	}
}

func TestCleanerClean(t *testing.T) {
	c := NewCleaner()
	res, err := c.Clean("light", nil, nil)
	if err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if res.DurationMs < 0 {
		t.Error("expected non-negative duration")
	}
	if c.IsCleaning() {
		t.Error("expected not cleaning after completion")
	}
}

func TestCleanerConcurrent(t *testing.T) {
	c := NewCleaner()
	_, _ = c.Clean("light", nil, nil)
	_, err := c.Clean("light", nil, nil)
	if err != nil {
		t.Fatalf("second clean should succeed: %v", err)
	}
}

func TestProcessManager(t *testing.T) {
	pm := NewProcessManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pm.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	list := pm.List()
	if len(list) == 0 {
		t.Skip("no processes returned on this platform")
	}
	top := pm.Top(5)
	if len(top) == 0 {
		t.Error("expected top processes")
	}
	pm.Stop()
}

func TestProfiles(t *testing.T) {
	for _, name := range ProfileNames() {
		p, ok := GetProfile(name)
		if !ok {
			t.Errorf("profile %s not found", name)
		}
		if p.Name == "" {
			t.Error("expected non-empty profile name")
		}
	}
}

func TestUniqueStrings(t *testing.T) {
	in := []string{"a", "b", "a", "c", "b"}
	out := uniqueStrings(in)
	if len(out) != 3 {
		t.Errorf("expected 3 unique, got %d", len(out))
	}
}

func TestShouldSkip(t *testing.T) {
	if !shouldSkip("/foo/bar", []string{"/foo"}) {
		t.Error("expected skip")
	}
	if shouldSkip("/foo/bar", []string{"/baz"}) {
		t.Error("expected not skip")
	}
}

func TestCleanerSnapshot(t *testing.T) {
	c := NewCleaner()
	c.TakeSnapshot("snap1", MemoryInfo{Total: 8000}, []ProcessInfo{{PID: 1, Name: "test"}}, "light")
	s, ok := c.GetSnapshot("snap1")
	if !ok {
		t.Fatal("expected snapshot")
	}
	if s.Level != "light" {
		t.Error("expected light level")
	}
	list := c.ListSnapshots()
	if len(list) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(list))
	}
}

func TestCleanerRollback(t *testing.T) {
	c := NewCleaner()
	c.TakeSnapshot("rb1", MemoryInfo{Total: 8000}, nil, "standard")
	s, err := c.Rollback("rb1")
	if err != nil {
		t.Fatal(err)
	}
	if s.Level != "standard" {
		t.Error("expected standard")
	}
	// test not found
	_, err = c.Rollback("rb-missing")
	if err == nil {
		t.Error("expected not found")
	}
}


