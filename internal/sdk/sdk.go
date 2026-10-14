package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client 是 TSGMemOptimizer 的 SDK 客户端
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:29471"
	}
	return &Client{BaseURL: baseURL, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) get(path string, out interface{}) error {
	resp, err := c.HTTP.Get(c.BaseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) post(path string, body, out interface{}) error {
	b, _ := json.Marshal(body)
	resp, err := c.HTTP.Post(c.BaseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("http %d: %v", resp.StatusCode, e)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// --- 进程级 API ---

func (c *Client) ListProcesses() (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.get("/api/processes", &out)
	return out, err
}

func (c *Client) KillProcess(pid int) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.post(fmt.Sprintf("/api/kill?pid=%d", pid), nil, &out)
	return out, err
}

// --- 应用级 API ---

func (c *Client) GetMemory() (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.get("/api/memory", &out)
	return out, err
}

func (c *Client) GetHistory() ([]interface{}, error) {
	var out []interface{}
	err := c.get("/api/memory/history", &out)
	return out, err
}

func (c *Client) Clean(level string) (map[string]interface{}, error) {
	var out map[string]interface{}
	path := "/api/clean"
	if level != "" {
		path += "?level=" + level
	}
	err := c.post(path, nil, &out)
	return out, err
}

// --- 职业级 API ---

func (c *Client) ListProfiles() (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.get("/api/profiles", &out)
	return out, err
}

func (c *Client) SetProfile(name string) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.post("/api/profile?name="+name, nil, &out)
	return out, err
}

// --- 风险等级 API ---

func (c *Client) GetGuardrails() (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.get("/api/guardrails", &out)
	return out, err
}

func (c *Client) GetHealth() (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.get("/api/health", &out)
	return out, err
}

// --- AI 调度 API ---

func (c *Client) AISchedule(req map[string]interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}
	err := c.post("/api/ai-schedule", req, &out)
	return out, err
}

// --- OpenAI Function Calling 快捷方法 ---

func (c *Client) OpenAIFunctionCall(argsJSON string) (map[string]interface{}, error) {
	return c.AISchedule(map[string]interface{}{
		"protocol": "openai",
		"args":     argsJSON,
	})
}
