# TSGMemOptimizer

TSG 内存动态管理工具 — 一键释放内存、清理垃圾、优化本地 AI 运行环境。

## 特性

- **一键内存释放**：释放系统缓存、临时文件、回收站，支持轻度/标准/深度三档
- **职业模式**：均衡 / 文职 / 开发 / 游戏 / AI 运行，自动调整保护策略与清理强度
- **进程管理**：实时查看内存占用排行，支持手动结束进程与自动保护白名单
- **自动优化**：内存达到阈值自动清理，超限自动关闭高内存进程（可选）
- **优美 UI**：深色主题本地 Web 界面，一键启动浏览器
- **自动更新**：检测 GitHub Release 并后台下载
- **TSG 网关集成**：可作为独立工具运行，也可由 TSG 网关直接调用
- **AI 调度**：支持 OpenAI function calling / MCP / REST 三种形态调用
- **五道硬护栏**：资源上限、频率限制、白名单保护、渐进清理、回滚机制

## 快速开始

```bash
# 下载对应平台二进制，直接运行
./tsgmemo-v1.3.0-linux-amd64
# 浏览器自动打开 http://127.0.0.1:29471
```

## SDK 调用

提供 Go / Python / Node 三种客户端：

```go
import "github.com/HP-jh/TSGMemOptimizer/sdk/go"
client := sdk.New("http://127.0.0.1:29471")
client.Clean("light")
```

```python
from sdk.python.tsgmemo import TSGMemOptimizerClient
client = TSGMemOptimizerClient()
client.clean("light")
```

```js
const { TSGMemOptimizerClient } = require('./sdk/node');
const client = new TSGMemOptimizerClient();
client.clean('light');
```

## AI 调度示例

```bash
curl -X POST http://127.0.0.1:29471/api/ai-schedule \
  -H "Content-Type: application/json" \
  -d '{"agent_id":"my-ai","intent":"clean","risk_declared":"low","reason":"memory high","level":"light"}'
```

## 构建

```bash
./scripts/build.sh 1.3.0
```

## 版本

| 版本 | 重点 |
|------|------|
| v1.0.0 | 性能优化 — 核心引擎、跨平台监控、职业模式 |
| v1.1.0 | 小白化 — 引导流程、术语人话化、一键智能 |
| v1.2.0 | UI 美化 — 动效、主题、响应式、无障碍 |
| v1.3.0 | AI 调度 — 护栏系统、SDK、OpenAI/MCP/REST 三协议 |

## 与 TSG 网关的关系

TSGMemOptimizer 是独立仓库与独立二进制。TSG 网关（github.com/HP-jh/TarsSecureGuard）v4.8.0+ 可选内置本工具，网关 UI 自动显示「内存优化」入口。
