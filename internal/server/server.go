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

	"github.com/HP-jh/TSGMemOptimizer/internal/config"
	"github.com/HP-jh/TSGMemOptimizer/internal/engine"
	"github.com/HP-jh/TSGMemOptimizer/internal/updater"
)

//go:embed all:web
var webFS embed.FS

type Server struct {
	monitor *engine.Monitor
	cleaner *engine.Cleaner
	pm      *engine.ProcessManager
	cfg     *config.Config
	updater *updater.Updater
}

func New(m *engine.Monitor, c *engine.Cleaner, pm *engine.ProcessManager, cfg *config.Config) *Server {
	return &Server{
		monitor: m, cleaner: c, pm: pm, cfg: cfg,
		updater: updater.New("HP-jh", "TSGMemOptimizer"),
	}
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
	mux.HandleFunc("/api/health", s.handleHealth)
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
		"system":   mem,
		"go":       gomem,
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
	res, err := s.cleaner.Clean(level, s.cfg.Get().AllowedPaths, s.cfg.Get().SkipPaths)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	writeJSON(w, res)
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
		})
		_ = config.Save(s.cfg)
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "version": s.cfg.Get().Version})
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
		case strings.HasSuffix(path, ".html"): ct = "text/html; charset=utf-8"
		case strings.HasSuffix(path, ".css"): ct = "text/css; charset=utf-8"
		case strings.HasSuffix(path, ".js"): ct = "application/javascript; charset=utf-8"
		case strings.HasSuffix(path, ".png"): ct = "image/png"
		case strings.HasSuffix(path, ".svg"): ct = "image/svg+xml"
	}
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(v)
	w.Write(b)
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
			_, _ = s.cleaner.Clean(p.CleanLevel, cfg.AllowedPaths, cfg.SkipPaths)
		}
		if cfg.AutoKillEnabled && mem.UsagePercent >= float64(cfg.AutoCleanThreshold+5) {
			p, _ := engine.GetProfile(cfg.CurrentProfile)
			_, _ = s.pm.AutoKill(cfg.AutoKillThreshold, p.ProtectPIDs)
		}
		// sleep 10s
		select {
		case <-ctx.Done():
			return
		case <-ctx.Done():
			return
		default:
		}
	}
}

func init() {
	_ = context.Background
	_ = fs.FileMode(0)
	_ = fmt.Sprintf
}
