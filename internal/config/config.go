package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	mu sync.RWMutex `json:"-"`

	AutoCleanThreshold int      `json:"autoCleanThreshold"` // 内存使用率阈值(%)
	AutoKillEnabled    bool     `json:"autoKillEnabled"`    // 自动关闭高内存进程
	AutoKillThreshold  int      `json:"autoKillThreshold"`  // 自动关闭阈值(MB)
	CurrentProfile     string   `json:"currentProfile"`     // 当前职业
	CleanLevel         string   `json:"cleanLevel"`         // light|standard|deep
	SkipPaths          []string `json:"skipPaths"`          // 跳过路径
	AllowedPaths       []string `json:"allowedPaths"`       // 允许清理的路径
	SeenTour           bool     `json:"seenTour"`           // 是否看过引导
	Version            string   `json:"version"`

	// v1.3.0 护栏配置
	MaxCleanMBPerRun  int      `json:"maxCleanMBPerRun"`  // 单次最大释放 MB
	MaxCleanPerMinute int      `json:"maxCleanPerMinute"` // 每分钟最大次数
	MaxCleanPerHour   int      `json:"maxCleanPerHour"`   // 每小时最大次数
	GuardWhitelist    []string `json:"guardWhitelist"`    // 白名单
	AIDefaultLevel    string   `json:"aiDefaultLevel"`    // AI 默认级别
	RollbackWindowSec int      `json:"rollbackWindowSec"` // 回滚窗口
}

var Default = &Config{
	AutoCleanThreshold: 85,
	AutoKillEnabled:    false,
	AutoKillThreshold:  2048,
	CurrentProfile:     "balanced",
	CleanLevel:         "standard",
	SkipPaths:          []string{},
	AllowedPaths:       []string{},
	Version:            "1.3.0",
	SeenTour:           false,
	MaxCleanMBPerRun:   4096,
	MaxCleanPerMinute:  3,
	MaxCleanPerHour:    10,
	GuardWhitelist:     []string{},
	AIDefaultLevel:     "light",
	RollbackWindowSec:  30,
}

func Path() string {
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "TSGMemOptimizer", "config.json")
}

func Load() (*Config, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(filepath.Dir(p), 0755)
			_ = Save(Default)
			return Default, nil
		}
		return nil, err
	}
	c := &Config{}
	if err := json.Unmarshal(data, c); err != nil {
		return Default, nil
	}
	if c.Version == "" {
		c.Version = "1.0.0"
	}
	return c, nil
}

func Save(c *Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := Path()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func (c *Config) Get() Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c
}

func (c *Config) Set(fn func(*Config)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c)
}
