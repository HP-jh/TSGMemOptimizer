package aischedule

import (
	"testing"

	"github.com/HP-jh/TSGMemOptimizer/internal/engine"
	"github.com/HP-jh/TSGMemOptimizer/internal/guardrail"
)

func TestPreCheck(t *testing.T) {
	g := guardrail.New(nil)
	s := New(g, engine.NewCleaner(), engine.NewProcessManager(), engine.NewMonitor())

	if ok, _ := s.preCheck(&Request{AgentID: "a", Intent: "clean", RiskDeclared: "low"}); !ok {
		t.Error("expected ok")
	}
	if ok, _ := s.preCheck(&Request{AgentID: "", Intent: "clean", RiskDeclared: "low"}); ok {
		t.Error("expected fail on empty agent")
	}
	if ok, _ := s.preCheck(&Request{AgentID: "a", Intent: "clean"}); ok {
		t.Error("expected fail on empty risk")
	}
}

func TestHandleClean(t *testing.T) {
	g := guardrail.New(&guardrail.Config{MaxCleanMBPerRun: 4096, MaxCleanPerMinute: 10, MaxCleanPerHour: 100, AIDefaultLevel: "light"})
	s := New(g, engine.NewCleaner(), engine.NewProcessManager(), engine.NewMonitor())

	resp := s.Handle(&Request{AgentID: "test", Intent: "clean", RiskDeclared: "low", Reason: "test", Level: "deep"})
	if resp.Allowed {
		t.Error("expected reject ai deep")
	}

	resp = s.Handle(&Request{AgentID: "test", Intent: "clean", RiskDeclared: "low", Reason: "test", Level: "light"})
	if !resp.Allowed {
		t.Errorf("expected allow light, got: %s", resp.Reason)
	}
	if resp.RollbackID == "" {
		t.Error("expected rollback id")
	}
}

func TestHandleQuery(t *testing.T) {
	g := guardrail.New(nil)
	m := engine.NewMonitor()
	m.Start(nil)
	defer m.Stop()
	s := New(g, engine.NewCleaner(), engine.NewProcessManager(), m)

	resp := s.Handle(&Request{AgentID: "test", Intent: "query", RiskDeclared: "low", Reason: "info"})
	if !resp.Allowed {
		t.Error("expected allow query")
	}
	if resp.Data["guardrails"] == nil {
		t.Error("expected guardrails in data")
	}
}

func TestHandleKillWhitelist(t *testing.T) {
	g := guardrail.New(&guardrail.Config{Whitelist: []string{"systemd"}})
	s := New(g, engine.NewCleaner(), engine.NewProcessManager(), engine.NewMonitor())

	resp := s.Handle(&Request{AgentID: "test", Intent: "kill", RiskDeclared: "high", Reason: "test", TargetName: "systemd"})
	if resp.Allowed {
		t.Error("expected reject whitelisted kill")
	}
}

func TestOpenAIFunctionSchema(t *testing.T) {
	schema := OpenAIFunctionSchema()
	if schema["name"] != "tsgmemo_schedule" {
		t.Error("expected tsgmemo_schedule")
	}
}

func TestParseOpenAICall(t *testing.T) {
	req, err := ParseOpenAICall(`{"agent_id":"x","intent":"query","risk_declared":"low","reason":"test"}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.AgentID != "x" {
		t.Error("expected x")
	}
	if req.Protocol != "openai" {
		t.Error("expected openai protocol")
	}
}
