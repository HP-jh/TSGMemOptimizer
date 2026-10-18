# TSGMemOptimizer Changelog

## v1.3.2 · 集成版本（历史优点整合 + 审计日志）

**发布日期**: 2026-10-18

### 历史优点整合
本次版本汇总并继承了 v1.0.0 ~ v1.3.1 全部核心优点：
1. **v1.0.0 基础能力** — 实时内存监控、三档清理（light/standard/deep）、本地 Web UI、进程列表与终止
2. **v1.1.0 性能升级** — 清理引擎优化、平台适配（Linux/macOS/Windows）
3. **v1.2.0 UI & 小白化** — 引导流程、智能建议 /api/smart、职业模式（balanced/office/dev/gaming/ai）
4. **v1.3.0 安全与生态** — AI 调度 /api/ai-schedule（OpenAI Function Calling / MCP / REST）、五道硬护栏（资源上限 4GB、频率限制、白名单、渐进清理、回滚窗口 30s）、SDK 三客户端（Go/Python/Node）、自动更新检查、7 平台二进制、快照与回滚
5. **v1.3.1 文档与维护** — CHANGES.md 持续维护、MEMORY_TUNING.md 专项文档

### 新增功能
- **审计日志系统** (`internal/audit`)：
  - 每次清理、kill、AI 调度、回滚操作自动记录到 `~/.config/TSGMemOptimizer/audit.jsonl`
  - 支持日志轮转（2MB 上限、保留 3 份备份）
  - 新增 `/api/audit` 端点，支持 `?limit=N` 查询最近审计记录
  - 新增 `/api/audit` 统计接口（按 action 聚合计数）
- **健康检查增强** (`/api/health`)：
  - 返回运行时长 `uptimeSeconds`
  - 返回当前内存使用率、可用/总量
  - 返回审计统计 `auditStats`
- **修复 v1.3.1 版本号遗漏**：main.go 与 config 默认值同步至 `1.3.2`

### 测试
- 新增 `internal/audit/audit_test.go`（5 个测试用例）：
  - `TestLoggerLogAndRecent` — 日志写入与逆序读取
  - `TestLoggerStats` — 按 action 聚合统计
  - `TestLoggerRotate` — 日志轮转触发
  - `TestRecentLimit` — 查询数量限制
- 全部现有测试通过：`go test ./...`

### 安全
- 审计日志写入用户配置目录，权限 0644，不暴露敏感路径内容
- 审计查询端点无鉴权但仅限本地访问（127.0.0.1）
