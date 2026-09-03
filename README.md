# mcp-go-common

开箱即用的 MCP（Model Context Protocol）Server 核心基础库，基于 [mcp-go](https://github.com/mark3labs/mcp-go)。

让你用 3 行代码启动一个完整的生产级 MCP Server，内置现代 React 调试面板（Inspector）、API Key 与 Web 登录鉴权、高危操作二次确认（Elicitation）、分级日志与健康检查。

---

## 核心特性

- **现代 React 交互面板（Web Landing & Inspector）** — 内嵌 Vite + React + TS 现代化单页应用，直观可视化调试 Tools、Resources、Prompts
- **API Key & Web 登录双模鉴权** — 支持 `Authorization: Bearer <key>` 接口鉴权与基于 HttpOnly Cookie 的浏览器端登录拦截防护
- **高危操作二次确认（Elicitation）** — 封装标准人机协同（Human in the loop）弹窗确认机制，支持四态结果精细审计
- **分级日志中间件** — 解析 JSON-RPC 报文，按类型分类彩色打印（TOOL CALL / LIFECYCLE / DISCOVERY）
- **客户端自动追踪** — 自动从 `initialize` 的 `clientInfo.name` 识别调用方，通过 `Mcp-Session-Id` 实现全链路追踪
- **健康检查与优雅退出** — 自动挂载 `/health` 端点，支持注入业务级自定义健康检查逻辑
- **开箱即用启动器** — `mcputil.Start()` 一行代码搞定 Mux、中间件链、静态资源托管与 HTTP 端口监听

---

## 版本选型指南（Version Selection Guide）

`mcp-go-common` 历经多个版本的迭代演进。你可以根据下游 MCP Server 的具体安全和功能需求选择最适合的版本：

### 🎯 快速决策树

* **新服务 / 知识库 / 包含文档与提示词的服务**：👉 **`v0.7.0`（强烈推荐）**
  * 拥有完整 MCP 三原语（Tools + Resources + Prompts）的在线预览与交互能力，且向下严格兼容纯 Tools 服务。
* **暴露在公网 / 共享网络，需要保护调试页面的服务**：👉 **`v0.6.0+`**
  * 引入 Web 登录页与 HttpOnly Session Cookie，非授权用户无法访问 Inspector 与自定义后台（如 `/history`）。
* **涉及写库、重启、远程命令等高危运维操作的服务**：👉 **`v0.5.1+`**
  * 包含 Elicitation 弹窗确认机制，并支持四态（Accept / Reject / Cancel / Timeout）结果审计。
* **纯内网、简单只读工具调用的存量服务**：👉 **`v0.4.2`**
  * 轻量稳定，具备基础的 API Key 校验、同源免密和复制兼容。

---

### 📊 版本功能支持矩阵

| 版本 (Tag) | 基础启动与日志 | React Inspector | Elicitation 人机确认 | Web 登录鉴权 (Cookie) | Resources & Prompts 预览 | 适用场景与代表服务 |
| :--- | :---: | :---: | :---: | :---: | :---: | :--- |
| **`v0.7.0`** *(最新)* | ✅ | ✅ | ✅ (四态枚举) | ✅ (Session Cookie) | **✅ (智能条件渲染)** | **全功能推荐**（如 `dev-knowledge-mcp-server`） |
| **`v0.6.0`** | ✅ | ✅ (仅Tools) | ✅ (四态枚举) | **✅ (新增登录防护)** | ❌ | **带管理后台/公网访问**（如 `ssh-mcp-server`） |
| **`v0.5.1`** | ✅ | ✅ (仅Tools) | **✅ (支持四态枚举)** | ❌ (同源免密) | ❌ | **需要严格命令审核的服务** |
| **`v0.5.0`** | ✅ | ✅ (仅Tools) | **✅ (首次引入)** | ❌ (同源免密) | ❌ | 支持二次弹窗确认与 ExtraRoutes 扩展 |
| **`v0.4.2`** | ✅ | ✅ (仅Tools) | ❌ | ❌ (同源免密) | ❌ | **纯内网存量服务**（如 `mysql`, `redis`, `loki`） |
| **`v0.4.1`** | ✅ | ✅ (仅Tools) | ❌ | ❌ (同源免密) | ❌ | 首次支持 Inspector 同源直连免 Bearer |
| **`v0.4.0`** | ✅ | ✅ (仅Tools) | ❌ | ❌ | ❌ | 升级 mcp-go 至 v0.55.1、Go 1.25.5 |
| **`v0.3.0`** | ✅ | **✅ (首发SPA)** | ❌ | ❌ | ❌ | 首次由单文件 html 升级为 React SPA |
| **`v0.2.0`** | ✅ | ❌ (旧静态html) | ❌ | ❌ | ❌ | 最初的脚手架版本 |

---

## 快速开始

```bash
go get github.com/bubua12/mcp-go-common@v0.7.0
```

### 最简用法（3 行启动）

```go
package main

import (
    "context"
    "os"

    mcputil "github.com/bubua12/mcp-go-common"
    "github.com/mark3labs/mcp-go/mcp"
)

func main() {
    // 1. 创建 MCP Server
    mcpServer := mcputil.NewServer("my-server", "1.0.0")

    // 2. 注册工具
    mcpServer.AddTool(mcp.Tool{
        Name:        "hello",
        Description: "Say hello to someone",
        InputSchema: mcp.ToolInputSchema{
            Type: "object",
            Properties: map[string]any{
                "name": map[string]any{"type": "string", "description": "Your name"},
            },
        },
    }, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        name := req.GetString("name", "world")
        return mcputil.TextResult("Hello, " + name + "!"), nil
    })

    // 3. 一键启动 HTTP 服务（自动挂载 /mcp, /health 和 Landing/Inspector）
    mcputil.Start(mcpServer, mcputil.Config{
        Port:        mcputil.GetEnv("LISTEN_PORT", "8080"),
        APIKey:      os.Getenv("API_KEY"),
        LandingPage: mcputil.NewLandingConfig("my-server", "My Custom MCP Server"),
    })
}
```

---

### 高级功能配置

#### 1. 自定义健康检查
```go
mcputil.Start(mcpServer, mcputil.Config{
    Port:   "8080",
    APIKey: os.Getenv("API_KEY"),
    HealthCheck: func() bool {
        // 自定义检查逻辑，返回 false 时 /health 将自动响应 HTTP 503
        return db.Ping() == nil
    },
})
```

#### 2. 自定义业务路由挂载（ExtraRoutes）
```go
mcputil.Start(mcpServer, mcputil.Config{
    Port:   "8080",
    APIKey: os.Getenv("API_KEY"),
    ExtraRoutes: func(mux *http.ServeMux) {
        // 挂载你自己的业务端点，会自动受 Web 登录认证保护
        mux.HandleFunc("/custom-metrics", myMetricsHandler)
    },
})
```

#### 3. Elicitation 人机二次确认调用示例
```go
// 在高危 Tool 处理函数内部发起弹窗确认
result, err := mcputil.RequestConfirmation(ctx, "即将执行 rm -rf 操作，请确认是否授权？")
switch result {
case mcputil.ConfirmAccepted:
    // 用户在 UI 上点击确认，继续执行
case mcputil.ConfirmRejected:
    return mcputil.ErrorResult("操作已被用户拒绝"), nil
case mcputil.ConfirmTimeout:
    return mcputil.ErrorResult("确认超时，操作已取消"), nil
}
```

---

## API 与配置参考

### Config 字段清单

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| `Port` | `string` | 监听端口，如 `"18080"` |
| `APIKey` | `string` | 非空时启用安全鉴权：`/mcp` 要求 Bearer 令牌或 Session Cookie；浏览器访问强制要求 `/login` |
| `SessionTTL` | `string` | Web 登录 Cookie 凭证有效期，如 `"24h"`（默认 24 小时） |
| `AllowSameOriginMCP` | `*bool` | 是否允许浏览器同源免 Bearer 直连 `/mcp`；默认跟随环境变量 `MCP_ALLOW_SAME_ORIGIN`（默认关闭） |
| `ProtectExtraRoutes` | `*bool` | 是否将 `ExtraRoutes` 挂载的自定义路由纳入 Web 鉴权保护（默认 `true`） |
| `HealthCheck` | `func() bool` | 业务自定义健康检查回调函数，返回 `false` 则 `/health` 响应 503 |
| `BeforeStart` | `func(port string)` | 启动监听前的生命周期回调，常用于打印就绪日志 |
| `ExtraRoutes` | `func(mux *http.ServeMux)` | 注入业务自定义端点的钩子函数 |
| `LandingPage` | `*LandingConfig` | 欢迎页与 Inspector 配置，传入 `nil` 则关闭 Web 调试页面 |

---

### 日志格式规范

内置的 `LogMiddleware` 会自动解析 JSON-RPC 载荷并输出结构化耗时追踪：

```text
[LIFECYCLE]   initialize               ← claude-code/10.0.0.1:54321  148µs
[LIFECYCLE]   initialized              ← claude-code/10.0.0.1:54321  79µs
[DISCOVERY]   tools/list               ← claude-code/10.0.0.1:54321  313µs
[TOOL CALL]  list_pods                  args={"namespace":"devops"}  ← cline/10.0.0.2:54321  12.3ms
[TOOL CALL]  get_deployment             ← claude-code/10.0.0.1:54321  8.1ms
[MCP]        resources/list            ← claude-code/10.0.0.1:54321  97µs
```

---

## 核心依赖基准

- **Go 语言**: `Go 1.25+`
- **MCP 官方 SDK**: [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) `v0.55.1+`
- **前端工具链**: Vite 5 + React 18 + TailwindCSS + Lucide Icons

## 开源协议

[MIT](LICENSE)
