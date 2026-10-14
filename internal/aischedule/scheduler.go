package aischedule

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/HP-jh/TSGMemOptimizer/internal/engine"
	"github.com/HP-jh/TSGMemOptimizer/internal/guardrail"
)

type Intent string

const (
	IntentClean   Intent = "clean"
	IntentKill    Intent = "kill"
	IntentBoost   Intent = "boost"
	IntentQuery   Intent = "query"
	IntentProfile Intent = "set_profile"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type Request struct {
	AgentID       string    `json:"agent_id"`
	Intent        string    `json:"intent"`
	RiskDeclared  RiskLevel `json:"risk_declared"`
	Level         string    `json:"level,omitempty"`
	TargetPID     int       `json:"target_pid,omitempty"`
	TargetName    string    `json:"target_name,omitempty"`
	ProfileName   string    `json:"profile_name,omitempty"`
	Reason        string    `json:"reason"`
	Protocol      string    `json:"protocol,omitempty"` // "openai" | "mcp" | "rest"
}

type Response struct {
	Allowed    bool                   `json:"allowed"`
	Action     string                 `json:"action,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Guardrails map[string]interface{} `json:"guardrails,omitempty"`
	RollbackID string                 `json:"rollback_id,omitempty"`
	Timestamp  int64                  `json:"timestamp"`
}

type Scheduler struct {
	guard    *guardrail.Guardrail
	cleaner  *engine.Cleaner
	pm       *engine.ProcessManager
	monitor  *engine.Monitor
}

func New(g *guardrail.Guardrail, c *engine.Cleaner, pm *engine.ProcessManager, m *engine.Monitor) *Scheduler {
	return &Scheduler{guard: g, cleaner: c, pm: pm, monitor: m}
}

func (s *Scheduler) Handle(req *Request) *Response {
	resp := &Response{Timestamp: time.Now().Unix()}

	// 统一前置检查
	allowed, reason := s.preCheck(req)
	if !allowed {
		resp.Allowed = false
		resp.Reason = reason
		resp.Guardrails = s.guard.Stats()
		return resp
	}

	switch req.Intent {
	case string(IntentClean):
		return s.handleClean(req)
	case string(IntentKill):
		return s.handleKill(req)
	case string(IntentBoost):
		return s.handleBoost(req)
	case string(IntentQuery):
		return s.handleQuery(req)
	case string(IntentProfile):
		return s.handleProfile(req)
	default:
		resp.Allowed = false
		resp.Reason = "未知意图: " + req.Intent
		return resp
	}
}

func (s *Scheduler) preCheck(req *Request) (bool, string) {
	if req.AgentID == "" {
		return false, "缺少 agent_id"
	}
	if req.Intent == "" {
		return false, "缺少 intent"
	}
	if req.RiskDeclared == "" {
		return false, "缺少风险声明 risk_declared"
	}
	return true, ""
}

func (s *Scheduler) handleClean(req *Request) *Response {
	resp := &Response{Timestamp: time.Now().Unix()}
	level := req.Level
	if level == "" {
		level = s.guard.Config().AIDefaultLevel
	}

	// 护栏检查
	ok, reason := s.guard.CheckClean(level, s.guard.Config().MaxCleanMBPerRun, true)
	if !ok {
		resp.Allowed = false
		resp.Reason = reason
		resp.Guardrails = s.guard.Stats()
		return resp
	}

	res, err := s.cleaner.Clean(level, nil, nil)
	s.guard.RecordClean()

	if err != nil {
		resp.Allowed = false
		resp.Reason = "清理执行失败: " + err.Error()
		return resp
	}

	resp.Allowed = true
	resp.Action = "clean"
	resp.Data = map[string]interface{}{
		"freedBytes":   res.FreedBytes,
		"freedMB":      fmt.Sprintf("%.1f", float64(res.FreedBytes)/1024/1024),
		"level":        res.Level,
		"durationMs":   res.DurationMs,
		"cleanedPaths": res.CleanedPaths,
	}
	resp.Guardrails = s.guard.Stats()
	resp.RollbackID = generateRollbackID()
	return resp
}

func (s *Scheduler) handleKill(req *Request) *Response {
	resp := &Response{Timestamp: time.Now().Unix()}
	if req.TargetPID == 0 && req.TargetName == "" {
		resp.Allowed = false
		resp.Reason = "kill 需要 target_pid 或 target_name"
		return resp
	}
	// 检查白名单
	name := req.TargetName
	if name == "" {
		for _, p := range s.pm.List() {
			if p.PID == req.TargetPID {
				name = p.Name
				break
			}
		}
	}
	if s.guard.IsWhitelisted(name) {
		resp.Allowed = false
		resp.Reason = fmt.Sprintf("进程 '%s' 在白名单中，禁止终止", name)
		return resp
	}
	pid := req.TargetPID
	if pid == 0 {
		for _, p := range s.pm.List() {
			if p.Name == req.TargetName {
				pid = p.PID
				break
			}
		}
	}
	if pid == 0 {
		resp.Allowed = false
		resp.Reason = "未找到目标进程"
		return resp
	}
	if err := s.pm.Kill(pid); err != nil {
		resp.Allowed = false
		resp.Reason = err.Error()
		return resp
	}
	resp.Allowed = true
	resp.Action = "kill"
	resp.Data = map[string]interface{}{"pid": pid, "name": name}
	return resp
}

func (s *Scheduler) handleBoost(req *Request) *Response {
	resp := &Response{Timestamp: time.Now().Unix()}
	resp.Allowed = false
	resp.Reason = "boost 暂未实现"
	return resp
}

func (s *Scheduler) handleQuery(req *Request) *Response {
	resp := &Response{Timestamp: time.Now().Unix(), Allowed: true, Action: "query"}
	mem := s.monitor.Latest()
	resp.Data = map[string]interface{}{
		"memory":     mem,
		"processes":  s.pm.List(),
		"guardrails": s.guard.Stats(),
	}
	return resp
}

func (s *Scheduler) handleProfile(req *Request) *Response {
	resp := &Response{Timestamp: time.Now().Unix()}
	if req.ProfileName == "" {
		resp.Allowed = false
		resp.Reason = "set_profile 需要 profile_name"
		return resp
	}
	if _, ok := engine.GetProfile(req.ProfileName); !ok {
		resp.Allowed = false
		resp.Reason = "未知职业: " + req.ProfileName
		return resp
	}
	resp.Allowed = true
	resp.Action = "set_profile"
	resp.Data = map[string]interface{}{"profile": req.ProfileName}
	return resp
}

func generateRollbackID() string {
	return fmt.Sprintf("rb-%d", time.Now().UnixNano())
}

// OpenAIFunctionSchema 返回 OpenAI function calling schema
func OpenAIFunctionSchema() map[string]interface{} {
	return map[string]interface{}{
		"name":        "tsgmemo_schedule",
		"description": "通过 TSGMemOptimizer 调度内存优化任务",
		"parameters": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id": map[string]interface{}{
					"type":        "string",
					"description": "调用方 AI agent 唯一标识",
				},
				"intent": map[string]interface{}{
					"type": "string",
					"enum": []string{"clean", "kill", "boost", "query", "set_profile"},
					"description": "调度意图",
				},
				"risk_declared": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"low", "medium", "high"},
					"description": "声明的风险等级",
				},
				"level": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"light", "standard", "deep"},
					"description": "清理级别（AI 默认仅 light）",
				},
				"target_pid": map[string]interface{}{
					"type":        "integer",
					"description": "目标进程 PID（kill 用）",
				},
				"target_name": map[string]interface{}{
					"type":        "string",
					"description": "目标进程名（kill 用）",
				},
				"profile_name": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"balanced", "office", "dev", "gaming", "ai"},
					"description": "职业模式名",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "调度原因",
				},
			},
			"required": []string{"agent_id", "intent", "risk_declared", "reason"},
		},
	}
}

// MCPSchema 返回 MCP protocol schema (simplified)
func MCPSchema() map[string]interface{} {
	return map[string]interface{}{
		"tool":   "tsgmemo_schedule",
		"schema": OpenAIFunctionSchema(),
	}
}

// ParseOpenAICall 解析 OpenAI function_call arguments
func ParseOpenAICall(argsJSON string) (*Request, error) {
	var req Request
	if err := json.Unmarshal([]byte(argsJSON), &req); err != nil {
		return nil, err
	}
	req.Protocol = "openai"
	return &req, nil
}
