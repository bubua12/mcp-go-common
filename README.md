# mcp-go-common

开箱即用的 MCP Server 开发模板，基于 [mcp-go](https://github.com/mark3labs/mcp-go)。

让你用 3 行代码启动一个完整的 MCP Server，内置认证、日志、健康检查。

## 功能

- **API Key 认证中间件** — `Authorization: Bearer <key>` 校验
- **分级日志中间件** — 解析 JSON-RPC body，按类型分类打印（TOOL CALL / LIFECYCLE / DISCOVERY）
- **客户端追踪** — 自动从 `initialize` 的 `clientInfo.name` 识别客户端，通过 `Mcp-Session-Id` 跨请求追踪
- **健康检查端点** — `/health` 支持自定义检查逻辑
- **工具结果构造** — `TextResult` / `ErrorResult`
- **通用启动模板** — `Start()` 一行搞定 mux、中间件、端口监听

## 快速开始

```bash
go get github.com/bubua12/mcp-go-common
```

### 最简用法（3 行启动）

```go
package main

import (
    "os"
    "github.com/mark3labs/mcp-go/mcp"
    mcputil "github.com/bubua12/mcp-go-common"
)

func main() {
    mcpServer := mcputil.NewServer("my-server", "1.0.0")

    mcpServer.AddTool(mcp.Tool{
        Name:        "hello",
        Description: "Say hello",
        InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]any{
            "name": map[string]any{"type": "string", "description": "Your name"},
        }},
    }, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        name := req.GetString("name", "world")
        return mcputil.TextResult("Hello, " + name + "!"), nil
    })

    mcputil.Start(mcpServer, mcputil.Config{
        Port:   mcputil.GetEnv("LISTEN_PORT", "8080"),
        APIKey: os.Getenv("API_KEY"),
    })
}
```

### 自定义健康检查

```go
mcputil.Start(mcpServer, mcputil.Config{
    Port:   "8080",
    APIKey: os.Getenv("API_KEY"),
    HealthCheck: func() bool {
        // 自定义检查逻辑，返回 false 则 /health 返回 503
        return db.Ping() == nil
    },
})
```

### 自定义启动逻辑

```go
mcputil.Start(mcpServer, mcputil.Config{
    Port:   "8080",
    APIKey: os.Getenv("API_KEY"),
    BeforeStart: func(port string) {
        log.Printf("🚀 my-server starting on :%s", port)
    },
})
```

### 只用中间件（不用 Start）

如果需要更多控制（如自定义路由、无认证等），可以单独使用中间件：

```go
httpServer := server.NewStreamableHTTPServer(mcpServer)
mux := http.NewServeMux()
mux.Handle("/mcp", mcputil.AuthMiddleware(apiKey, mcputil.LogMiddleware(httpServer)))
mux.HandleFunc("/health", myHealthHandler)
http.ListenAndServe(":8080", mux)
```

## API 参考

### 创建 Server

| 函数 | 说明 |
|------|------|
| `NewServer(name, version)` | 创建 MCP Server（等价于 `server.NewMCPServer` + `WithToolCapabilities(true)`） |
| `Start(mcpServer, cfg)` | 一键启动（mux + 中间件 + /health + /mcp + ListenAndServe） |

### Config

| 字段 | 类型 | 说明 |
|------|------|------|
| `Port` | `string` | 监听端口，如 `"8080"` |
| `APIKey` | `string` | 非空时启用 Bearer Token 认证 |
| `HealthCheck` | `func() bool` | 自定义健康检查，返回 false 则 /health 返回 503 |
| `BeforeStart` | `func(port string)` | 启动前回调，用于打印日志 |

### 中间件

| 函数 | 说明 |
|------|------|
| `AuthMiddleware(apiKey, next)` | API Key 认证（空 apiKey 则跳过） |
| `LogMiddleware(next)` | JSON-RPC 日志 + 客户端追踪 |

### 日志输出格式

```
[LIFECYCLE]   initialize               ← claude-code/10.0.0.1:54321  148µs
[LIFECYCLE]   initialized              ← claude-code/10.0.0.1:54321  79µs
[DISCOVERY]   tools/list               ← claude-code/10.0.0.1:54321  313µs
[TOOL CALL]  list_pods                  args={"namespace":"devops"}  ← cline/10.0.0.2:54321  12.3ms
[TOOL CALL]  get_deployment             ← claude-code/10.0.0.1:54321  8.1ms
[MCP]        resources/list            ← claude-code/10.0.0.1:54321  97µs
```

- `resources/*`、`prompts/*` 静默不打印
- 非 MCP 请求（健康检查等）静默不打印
- 客户端名从 `initialize` 的 `clientInfo.name` 自动识别

### 工具结果

| 函数 | 说明 |
|------|------|
| `TextResult(text)` | 成功结果（isError=false） |
| `ErrorResult(msg)` | 错误结果（isError=true，LLM 仍可读取错误信息） |

### 工具函数

| 函数 | 说明 |
|------|------|
| `FmtDuration(d)` | 格式化耗时：`148µs` / `2.0ms` / `1.23s` |
| `GetEnv(key, default)` | 读取环境变量，支持默认值 |
| `GetEnvInt(key, default)` | 读取整数环境变量，支持默认值 |

## 依赖

- Go 1.25+
- [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) v0.43.2+

## License

MIT
