# Landing Page 开发指南

mcp-go-common 内嵌了一个 React SPA 作为 Landing Page，所有 MCP Server 共享。浏览器访问 `GET /` 即可看到 Setup 配置指引和 MCP Inspector 工具调试界面。

## 架构

```
mcp-go-common/
├── web/
│   ├── src/                ← React + TypeScript 源码
│   │   ├── App.tsx         ← 主页面（Header + Tabs）
│   │   ├── components/
│   │   │   ├── tabs/
│   │   │   │   ├── client-configs.tsx   ← Setup Tab：客户端配置指引
│   │   │   │   ├── mcp-inspector.tsx    ← Inspector Tab：MCP 工具调试
│   │   │   │   └── icons.tsx            ← 各客户端 SVG 图标
│   │   │   └── ui/                      ← shadcn/ui 通用组件
│   │   ├── hooks/
│   │   │   └── use-mcp-client.ts        ← MCP SDK 客户端 Hook
│   │   ├── styles.css                   ← Tailwind CSS 主题
│   │   └── main.tsx                     ← React 入口
│   ├── dist/               ← npm run build 产物（必须提交到 git）
│   ├── embed.go            ← go:embed dist/*
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
├── middleware.go            ← SPA handler + /api/info + Start()
└── result.go
```

## 为什么 dist/ 必须提交到 git

`web/embed.go` 使用 `//go:embed dist/*` 在编译时嵌入前端资源。如果 dist/ 不在仓库里，任何人 `go build` 都会报错。这是 Go embed 的限制——编译时文件必须在磁盘上。

mcp-victorialogs 等项目也采用同样方案。

## 前端开发流程

### 环境准备（首次）

```bash
cd web
npm install
```

需要 Node.js 18+。

### 本地开发

```bash
cd web
npm run dev
```

Vite dev server 启动后访问 `http://localhost:5173`，支持热更新。

注意：dev 模式下 `/api/info` 和 `/mcp` 端点指向 Vite 自己，不会代理到 Go 后端。要测试完整功能需要先构建再启动 Go 服务。

### 构建

```bash
cd web
npm run build
```

产物输出到 `web/dist/`：
- `index.html` — 入口
- `assets/index-*.js` — 打包后的 JS（带 content hash）
- `assets/index-*.css` — 打包后的 CSS

每次 build 文件名 hash 不同，旧文件需要手动删除或直接覆盖提交。

### 验证 Go 编译

```bash
cd ..                    # 回到 mcp-go-common 根目录
go build ./...           # 验证 embed 成功
```

### 提交 + 打 Tag

```bash
git add -A
git commit -m "xxx"
git tag -a v0.x.y -m "description"
git push origin master --tags
```

**版本号规则：**
- 必须用新版本号，Go module proxy 缓存旧 tag 不可覆盖
- patch 递增：`v0.3.2` → `v0.3.3`（小修）
- minor 递增：`v0.3.x` → `v0.4.0`（新功能）

## MCP Server 侧接入

### main.go

```go
import mcputil "github.com/bubua12/mcp-go-common"

port := mcputil.GetEnv("LISTEN_PORT", "18089")
mcputil.Start(mcpServer, mcputil.Config{
    Port:        port,
    APIKey:      os.Getenv("API_KEY"),
    LandingPage: mcputil.NewLandingConfig("server-name", "Server description"),
    HealthCheck: func() bool { return true },
    BeforeStart: func(port string) {
        log.Printf("server starting on :%s", port)
    },
})
```

`LandingPage` 传 `nil` 则不注册 `/` 路由。

### 本地调试（不用打 tag）

在 MCP Server 的 `go.mod` 里加 replace 指向本地：

```bash
go mod edit -replace github.com/bubua12/mcp-go-common=/path/to/mcp-go-common
go mod tidy
go build ./...
```

测完后删除 replace：

```bash
go mod edit -dropreplace github.com/bubua12/mcp-go-common
```

## 路由说明

| 路径 | 方法 | 说明 |
|------|------|------|
| `/` | GET | Landing Page（React SPA） |
| `/api/info` | GET | 返回 `{"name":"...","description":"..."}` |
| `/health` | GET | 健康检查 |
| `/mcp` | POST | MCP 协议端点（Inspector 通过此端口连接） |

## 更新 sre-agent-pro 中所有 MCP Server 的引用

```bash
cd sre-agent-pro

# 更新 go.mod 版本号
for dir in kubernetes-mcp-server loki-mcp-server prometheus-mcp-server \
           mysql-mcp-server redis-mcp-server arthas-mcp-server \
           docker-mcp-server victoria-logs-mcp-server \
           linux-node-controller/node-mcp-server log-mcp-server; do
  sed -i 's|mcp-go-common v旧版本|mcp-go-common v新版本|' "$dir/go.mod"
done

# 全量编译验证
for dir in kubernetes-mcp-server loki-mcp-server prometheus-mcp-server \
           mysql-mcp-server redis-mcp-server arthas-mcp-server \
           docker-mcp-server victoria-logs-mcp-server \
           linux-node-controller/node-mcp-server log-mcp-server; do
  (cd "$dir" && go mod tidy && go build ./...) && echo "OK: $dir" || echo "FAIL: $dir"
done
```

## 修改清单

| 改什么 | 改哪里 |
|--------|--------|
| 服务器名/描述 | MCP Server 的 `main.go` 里 `NewLandingConfig(name, description)` |
| 添加新客户端配置 | `web/src/components/tabs/client-configs.tsx` 的 `configs` 数组 |
| 客户端图标 | `web/src/components/tabs/icons.tsx` |
| Inspector 行为 | `web/src/hooks/use-mcp-client.ts` |
| 页面主题/样式 | `web/src/styles.css` |
| Header 布局 | `web/src/App.tsx` |
