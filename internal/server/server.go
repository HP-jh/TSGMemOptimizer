package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/HP-jh/TSGMemOptimizer/internal/aischedule"
	"github.com/HP-jh/TSGMemOptimizer/internal/config"
	"github.com/HP-jh/TSGMemOptimizer/internal/engine"
	"github.com/HP-jh/TSGMemOptimizer/internal/guardrail"
	"github.com/HP-jh/TSGMemOptimizer/internal/updater"
)

//go:embed all:web
var webFS embed.FS

type Server struct {
	monitor   *engine.Monitor
	cleaner   *engine.Cleaner
	pm        *engine.ProcessManager
	cfg       *config.Config
	updater   *updater.Updater
	guard     *guardrail.Guardrail
	scheduler *aischedule.Scheduler
}

func New(m *engine.Monitor, c *engine.Cleaner, pm *engine.ProcessManager, cfg *config.Config) *Server {
	g := guardrailFromConfig(cfg)
	s := &Server{
		monitor: m, cleaner: c, pm: pm, cfg: cfg,
		updater: updater.New("HP-jh", "TSGMemOptimizer"),
		guard:   g,
	}
	s.scheduler = aischedule.New(g, c, pm, m)
	return s
}

func guardrailFromConfig(cfg *config.Config) *guardrail.Guardrail {
	c := cfg.Get()
	gcfg := &guardrail.Config{
		MaxCleanMBPerRun:  c.MaxCleanMBPerRun,
		MaxCleanPerMinute: c.MaxCleanPerMinute,
		MaxCleanPerHour:   c.MaxCleanPerHour,
		Whitelist:         c.GuardWhitelist,
		AIDefaultLevel:    c.AIDefaultLevel,
		RollbackWindowSec: c.RollbackWindowSec,
	}
	return guardrail.New(gcfg)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/memory", s.handleMemory)
	mux.HandleFunc("/api/memory/history", s.handleHistory)
	mux.HandleFunc("/api/processes", s.handleProcesses)
	mux.HandleFunc("/api/clean", s.handleClean)
	mux.HandleFunc("/api/kill", s.handleKill)
	mux.HandleFunc("/api/profiles", s.handleProfiles)
	mux.HandleFunc("/api/profile", s.handleSetProfile)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/update/check", s.handleUpdateCheck)
	mux.HandleFunc("/api/update/download", s.handleUpdateDownload)
	mux.HandleFunc("/api/smart", s.handleSmart)
	mux.HandleFunc("/api/tour", s.handleTour)
	mux.HandleFunc("/api/health", s.handleHealth)
	// v1.3.0
	mux.HandleFunc("/api/ai-schedule", s.handleAISchedule)
	mux.HandleFunc("/api/guardrails", s.handleGuardrails)
	mux.HandleFunc("/api/guardrails/whitelist", s.handleWhitelist)
	mux.HandleFunc("/api/rollback", s.handleRollback)
	mux.HandleFunc("/api/snapshots", s.handleSnapshots)
	mux.HandleFunc("/api/openai/functions", s.handleOpenAIFunctions)
	mux.HandleFunc("/api/mcp/tools", s.handleMCPTools)
	mux.HandleFunc("/", s.handleStatic)
	return mux
}

func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mem := s.monitor.Latest()
	gomem := engine.GetGoMemory()
	writeJSON(w, map[string]interface{}{
		"system": mem,
		"go":     gomem,
		"formatted": map[string]string{
			"total": engine.FormatBytes(mem.Total),
			"used":  engine.FormatBytes(mem.Used),
			"free":  engine.FormatBytes(mem.Free),
		},
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.monitor.History())
}

func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{
		"processes": s.pm.List(),
		"summary":   s.pm.Summary(),
	})
}

func (s *Server) handleClean(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	level := s.cfg.Get().CleanLevel
	if r.URL.Query().Get("level") != "" {
		level = r.URL.Query().Get("level")
	}

	// v1.3.0 护栏检查（非 AI 调用，isAI=false）
	ok, reason := s.guard.CheckClean(level, s.guard.Config().MaxCleanMBPerRun, false)
	if !ok {
		writeJSON(w, map[string]interface{}{"allowed": false, "reason": reason, "guardrails": s.guard.Stats()})
		return
	}

	// 回滚快照
	rollbackID := fmt.Sprintf("rb-%d", time.Now().UnixNano())
	s.cleaner.TakeSnapshot(rollbackID, s.monitor.Latest(), s.pm.List(), level)

	res, err := s.cleaner.Clean(level, s.cfg.Get().AllowedPaths, s.cfg.Get().SkipPaths)
	s.guard.RecordClean()

	if err != nil {
		writeJSON(w, map[string]interface{}{"allowed": false, "reason": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{
		"allowed":    true,
		"result":     res,
		"rollbackID": rollbackID,
		"guardrails": s.guard.Stats(),
	})
}

func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pidStr := r.URL.Query().Get("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		http.Error(w, "invalid pid", http.StatusBadRequest)
		return
	}
	// 查找进程名检查白名单
	name := ""
	for _, p := range s.pm.List() {
		if p.PID == pid {
			name = p.Name
			break
		}
	}
	if s.guard.IsWhitelisted(name) {
		writeJSON(w, map[string]interface{}{"allowed": false, "reason": fmt.Sprintf("进程 '%s' 在白名单中", name)})
		return
	}
	if err := s.pm.Kill(pid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]int{"killed": pid})
}

func (s *Server) handleProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var out []engine.Profile
	for _, name := range engine.ProfileNames() {
		if p, ok := engine.GetProfile(name); ok {
			out = append(out, p)
		}
	}
	writeJSON(w, map[string]interface{}{
		"profiles": out,
		"current":  s.cfg.Get().CurrentProfile,
	})
}

func (s *Server) handleSetProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	p, ok := engine.GetProfile(name)
	if !ok {
		http.Error(w, "unknown profile", http.StatusBadRequest)
		return
	}
	s.cfg.Set(func(c *config.Config) {
		c.CurrentProfile = name
		c.CleanLevel = p.CleanLevel
		c.AutoCleanThreshold = p.AutoThreshold
	})
	_ = config.Save(s.cfg)
	writeJSON(w, map[string]string{"profile": name})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.cfg.Get())
	case http.MethodPost:
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.cfg.Set(func(c *config.Config) {
			if v, ok := body["autoCleanThreshold"].(float64); ok {
				c.AutoCleanThreshold = int(v)
			}
			if v, ok := body["autoKillEnabled"].(bool); ok {
				c.AutoKillEnabled = v
			}
			if v, ok := body["autoKillThreshold"].(float64); ok {
				c.AutoKillThreshold = int(v)
			}
			if v, ok := body["cleanLevel"].(string); ok {
				c.CleanLevel = v
			}
			// v1.3.0
			if v, ok := body["maxCleanMBPerRun"].(float64); ok {
				c.MaxCleanMBPerRun = int(v)
			}
			if v, ok := body["maxCleanPerMinute"].(float64); ok {
				c.MaxCleanPerMinute = int(v)
			}
			if v, ok := body["maxCleanPerHour"].(float64); ok {
				c.MaxCleanPerHour = int(v)
			}
			if v, ok := body["aiDefaultLevel"].(string); ok {
				c.AIDefaultLevel = v
			}
			if v, ok := body["rollbackWindowSec"].(float64); ok {
				c.RollbackWindowSec = int(v)
			}
		})
		_ = config.Save(s.cfg)
		// sync guardrail
		s.guard = guardrailFromConfig(s.cfg)
		writeJSON(w, s.cfg.Get())
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := s.updater.Check(s.cfg.Get().Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, info)
}

func (s *Server) handleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := s.updater.Check(s.cfg.Get().Version)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !info.HasUpdate {
		writeJSON(w, map[string]string{"status": "up to date"})
		return
	}
	go func() {
		_ = s.updater.Download(info.URL, info.Version)
	}()
	writeJSON(w, map[string]string{"status": "downloading", "version": info.Version})
}

func (s *Server) handleSmart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	mem := s.monitor.Latest()
	cfg := s.cfg.Get()
	var actions []string
	var advice string
	if mem.UsagePercent >= 90 {
		advice = "内存严重不足，建议立即深度清理或关闭大内存程序"
		actions = append(actions, "clean", "kill")
	} else if mem.UsagePercent >= 75 {
		advice = "内存占用较高，建议标准清理释放缓存"
		actions = append(actions, "clean")
	} else if mem.UsagePercent >= 50 {
		advice = "内存状态正常，可轻度清理保持流畅"
		actions = append(actions, "clean")
	} else {
		advice = "内存充裕，无需操作"
	}
	p, _ := engine.GetProfile(cfg.CurrentProfile)
	writeJSON(w, map[string]interface{}{
		"advice":       advice,
		"actions":      actions,
		"profile":      cfg.CurrentProfile,
		"profileLabel": p.Label,
		"memory":       mem,
		"guardrails":   s.guard.Stats(),
	})
}

func (s *Server) handleTour(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.cfg.Set(func(c *config.Config) { c.SeenTour = true })
		_ = config.Save(s.cfg)
		writeJSON(w, map[string]bool{"seen": true})
		return
	}
	writeJSON(w, map[string]bool{"seen": s.cfg.Get().SeenTour})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "version": s.cfg.Get().Version})
}

// ---- v1.3.0 AI Schedule ----

func (s *Server) handleAISchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req aischedule.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// try parse OpenAI wrapper
		var wrap map[string]interface{}
		if err2 := json.NewDecoder(r.Body).Decode(&wrap); err2 == nil {
			if args, ok := wrap["args"].(string); ok {
				parsed, err3 := aischedule.ParseOpenAICall(args)
				if err3 == nil {
					req = *parsed
				}
			}
		}
		if req.AgentID == "" {
			http.Error(w, "invalid body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	resp := s.scheduler.Handle(&req)
	writeJSON(w, resp)
}

func (s *Server) handleGuardrails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.guard.Stats())
}

func (s *Server) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{"whitelist": s.guard.Whitelist()})
	case http.MethodPost:
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if name := body["add"]; name != "" {
			s.guard.AddWhitelist(name)
		}
		if name := body["remove"]; name != "" {
			s.guard.RemoveWhitelist(name)
		}
		writeJSON(w, map[string]interface{}{"whitelist": s.guard.Whitelist()})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	snap, err := s.cleaner.Rollback(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"success": false, "reason": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{
		"success":  true,
		"snapshot": snap,
		"note":     "内存已释放，快照供参考。如需恢复请重启相关进程或等待自然回涨。",
	})
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	list := s.cleaner.ListSnapshots()
	writeJSON(w, map[string]interface{}{"snapshots": list, "count": len(list)})
}

func (s *Server) handleOpenAIFunctions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{
		"functions": []map[string]interface{}{aischedule.OpenAIFunctionSchema()},
	})
}

func (s *Server) handleMCPTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{
		"tools": []map[string]interface{}{aischedule.MCPSchema()},
	})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(v)
	w.Write(b)
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	data, err := webFS.ReadFile("web/" + path)
	if err != nil {
		data, err = webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}
	ct := "application/octet-stream"
	switch {
	case strings.HasSuffix(path, ".html"):
		ct = "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".css"):
		ct = "text/css; charset=utf-8"
	case strings.HasSuffix(path, ".js"):
		ct = "application/javascript; charset=utf-8"
	case strings.HasSuffix(path, ".png"):
		ct = "image/png"
	case strings.HasSuffix(path, ".svg"):
		ct = "image/svg+xml"
	}
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}

func (s *Server) RunBackground(ctx context.Context) {
	go s.autoLoop(ctx)
}

func (s *Server) autoLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		cfg := s.cfg.Get()
		mem := s.monitor.Latest()
		if mem.UsagePercent >= float64(cfg.AutoCleanThreshold) && !s.cleaner.IsCleaning() {
			p, _ := engine.GetProfile(cfg.CurrentProfile)
			if ok, _ := s.guard.CheckClean(p.CleanLevel, s.guard.Config().MaxCleanMBPerRun, false); ok {
				_, _ = s.cleaner.Clean(p.CleanLevel, cfg.AllowedPaths, cfg.SkipPaths)
				s.guard.RecordClean()
			}
		}
		if cfg.AutoKillEnabled && mem.UsagePercent >= float64(cfg.AutoCleanThreshold+5) {
			p, _ := engine.GetProfile(cfg.CurrentProfile)
			_, _ = s.pm.AutoKill(cfg.AutoKillThreshold, p.ProtectPIDs)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func init() {
	_ = context.Background
	_ = fs.FileMode(0)
	_ = fmt.Sprintf
}
