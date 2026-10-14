package guardrail

import (
	"testing"
)

func TestGuardrailDefaults(t *testing.T) {
	g := New(nil)
	cfg := g.Config()
	if cfg.MaxCleanMBPerRun != 4096 {
		t.Errorf("default maxCleanMB = %d, want 4096", cfg.MaxCleanMBPerRun)
	}
	if cfg.AIDefaultLevel != "light" {
		t.Errorf("default ai level = %s, want light", cfg.AIDefaultLevel)
	}
	if len(cfg.Whitelist) == 0 {
		t.Error("expected non-empty whitelist")
	}
}

func TestCheckCleanHardCap(t *testing.T) {
	g := New(&Config{MaxCleanMBPerRun: 100})
	ok, reason := g.CheckClean("light", 200, false)
	if ok {
		t.Error("expected reject over cap")
	}
	if reason == "" {
		t.Error("expected reason")
	}
}

func TestCheckCleanFreqLimit(t *testing.T) {
	g := New(&Config{MaxCleanPerMinute: 1, MaxCleanPerHour: 10})
	g.RecordClean()
	ok, _ := g.CheckClean("light", 10, false)
	if ok {
		t.Error("expected reject over freq")
	}
}

func TestCheckCleanAILevel(t *testing.T) {
	g := New(&Config{AIDefaultLevel: "light"})
	ok, reason := g.CheckClean("deep", 100, true)
	if ok {
		t.Error("expected reject ai deep")
	}
	if reason == "" {
		t.Error("expected reason")
	}
	ok, _ = g.CheckClean("light", 100, true)
	if !ok {
		t.Error("expected allow ai light")
	}
}

func TestWhitelist(t *testing.T) {
	g := New(&Config{Whitelist: []string{"foo", "bar"}})
	if !g.IsWhitelisted("foo") {
		t.Error("expected foo whitelisted")
	}
	if g.IsWhitelisted("baz") {
		t.Error("expected baz not whitelisted")
	}
	g.AddWhitelist("baz")
	if !g.IsWhitelisted("baz") {
		t.Error("expected baz whitelisted after add")
	}
	g.RemoveWhitelist("foo")
	if g.IsWhitelisted("foo") {
		t.Error("expected foo removed")
	}
}

func TestStats(t *testing.T) {
	g := New(&Config{MaxCleanPerMinute: 5, MaxCleanPerHour: 20})
	g.RecordClean()
	st := g.Stats()
	if st["cleanCountMinute"].(int) != 1 {
		t.Errorf("minute count = %d, want 1", st["cleanCountMinute"])
	}
}

func TestUniqueStrings(t *testing.T) {
	in := []string{"a", "b", "a"}
	out := uniqueStrings(in)
	if len(out) != 2 {
		t.Errorf("expected 2, got %d", len(out))
	}
}
