// Package mcputil provides reusable middleware, helpers, and startup templates
// for building MCP servers with mcp-go.
//
// Usage:
//
//	mcpServer := mcputil.NewServer("my-server", "1.0.0")
//	mcpServer.AddTool(myTool, myHandler)
//	mcputil.Start(mcpServer, mcputil.Config{
//	    Port:        "8080",
//	    APIKey:      os.Getenv("API_KEY"),
//	    HealthCheck: func() bool { return true },
//	})
package mcputil

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

//go:embed landing.html
var landingHTML embed.FS

// Config holds the configuration for Start().
type Config struct {
	// Port is the listen port (e.g. "18080").
	Port string
	// APIKey enables Bearer token authentication when non-empty.
	APIKey string
	// HealthCheck is called for GET /health. Return true for 200 OK.
	HealthCheck func() bool
	// BeforeStart is called before http.ListenAndServe.
	BeforeStart func(port string)
	// LandingPage enables a browser-facing landing page at GET / with setup
	// instructions and tool inspection. Pass DefaultLandingPage() to get the
	// standard page, or nil to skip.
	LandingPage *LandingConfig
}

// LandingConfig configures the landing page served at GET /.
type LandingConfig struct {
	// Name is the MCP server name shown on the page (e.g. "log-mcp-server").
	Name string
	// Version is the server version string (e.g. "1.0.0").
	Version string
	// Endpoint is the full MCP endpoint URL shown for client configuration
	// (e.g. "http://localhost:18089/mcp").
	Endpoint string
	// ToolLister returns the list of registered tools. Called on each /api/tools request.
	// Pass nil to skip the tools API endpoint.
	ToolLister func() []ToolInfo
}

// ToolInfo describes a registered MCP tool for the landing page.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// clientSessions tracks clientInfo.name per MCP session (Mcp-Session-Id).
var clientSessions sync.Map

// NewServer creates a new MCP server with tool capabilities enabled.
func NewServer(name, version string) *server.MCPServer {
	return server.NewMCPServer(name, version, server.WithToolCapabilities(true))
}

// Start creates the HTTP mux, applies middleware, registers /health and /mcp,
// and starts listening. This is a blocking call.
func Start(mcpServer *server.MCPServer, cfg Config) {
	httpServer := server.NewStreamableHTTPServer(mcpServer)
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if cfg.HealthCheck != nil && !cfg.HealthCheck() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("unhealthy"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Landing page: GET / + GET /api/tools
	if cfg.LandingPage != nil {
		lp := cfg.LandingPage
		handler := renderLandingPage(lp)
		mux.Handle("/", handler)

		if lp.ToolLister != nil {
			mux.HandleFunc("/api/tools", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				json.NewEncoder(w).Encode(map[string]any{
					"name":    lp.Name,
					"version": lp.Version,
					"tools":   lp.ToolLister(),
				})
			})
		}
	}

	mux.Handle("/mcp", AuthMiddleware(cfg.APIKey, LogMiddleware(httpServer)))

	if cfg.BeforeStart != nil {
		cfg.BeforeStart(cfg.Port)
	}

	log.Printf("listening on :%s/mcp", cfg.Port)
	if cfg.LandingPage != nil {
		log.Printf("landing page: http://localhost:%s/", cfg.Port)
	}
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}

// AuthMiddleware validates Bearer token when apiKey is non-empty.
func AuthMiddleware(apiKey string, next http.Handler) http.Handler {
	if apiKey == "" {
		return next
	}
	log.Println("API_KEY configured, authentication enabled")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != apiKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LogMiddleware parses JSON-RPC body and prints structured logs with client tracking:
//   - [TOOL CALL]  — tools/call with tool name, args, duration
//   - [LIFECYCLE]  — initialize / notifications/initialized
//   - [DISCOVERY]  — tools/list
//   - [MCP]        — other MCP methods
//   - (silent)     — resources/*, prompts/*, health checks
//
// Client identification: parses clientInfo.name from initialize request,
// then tracks it via Mcp-Session-Id header for subsequent requests.
// Log format: "← clientName/remoteAddr" or "← remoteAddr" if unknown.
func LogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		remoteAddr := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			remoteAddr = strings.Split(xff, ",")[0]
		} else if xri := r.Header.Get("X-Real-IP"); xri != "" {
			remoteAddr = xri
		}

		mcpMethod := ""
		toolName := ""
		toolArgs := ""
		clientName := ""

		if r.Body != nil {
			bodyBytes, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			var rpcReq struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if json.Unmarshal(bodyBytes, &rpcReq) == nil && rpcReq.Method != "" {
				mcpMethod = rpcReq.Method
				if mcpMethod == "tools/call" {
					var callParams struct {
						Name      string                 `json:"name"`
						Arguments map[string]interface{} `json:"arguments"`
					}
					if json.Unmarshal(rpcReq.Params, &callParams) == nil {
						toolName = callParams.Name
						if len(callParams.Arguments) > 0 {
							argsBytes, _ := json.Marshal(callParams.Arguments)
							toolArgs = string(argsBytes)
						}
					}
				} else if mcpMethod == "initialize" {
					var initParams struct {
						ClientInfo struct {
							Name string `json:"name"`
						} `json:"clientInfo"`
					}
					if json.Unmarshal(rpcReq.Params, &initParams) == nil {
						clientName = initParams.ClientInfo.Name
					}
				}
			}
		}

		// Lookup client name from session ID (for non-initialize requests)
		sessionID := r.Header.Get("Mcp-Session-Id")
		if clientName == "" && sessionID != "" {
			if v, ok := clientSessions.Load(sessionID); ok {
				clientName = v.(string)
			}
		}

		// Capture response headers to get Mcp-Session-Id
		hw := &headerCaptureWriter{ResponseWriter: w}
		next.ServeHTTP(hw, r)
		dur := FmtDuration(time.Since(startTime))

		// Store client name for this session (on initialize response)
		if mcpMethod == "initialize" && clientName != "" {
			if sid := hw.Header().Get("Mcp-Session-Id"); sid != "" {
				clientSessions.Store(sid, clientName)
			}
		}

		// Build source string
		source := remoteAddr
		if clientName != "" {
			source = clientName + "/" + remoteAddr
		}

		switch {
		case toolName != "":
			if toolArgs != "" {
				log.Printf("[TOOL CALL]  %-28s args=%s  ← %s  %s", toolName, toolArgs, source, dur)
			} else {
				log.Printf("[TOOL CALL]  %-28s ← %s  %s", toolName, source, dur)
			}
		case mcpMethod == "initialize":
			log.Printf("[LIFECYCLE]   initialize               ← %s  %s", source, dur)
		case mcpMethod == "notifications/initialized":
			log.Printf("[LIFECYCLE]   initialized              ← %s  %s", source, dur)
		case mcpMethod == "tools/list":
			log.Printf("[DISCOVERY]   tools/list               ← %s  %s", source, dur)
		case strings.HasPrefix(mcpMethod, "resources"), strings.HasPrefix(mcpMethod, "prompts"):
			// silent
		case mcpMethod != "":
			log.Printf("[MCP]        %-28s ← %s  %s", mcpMethod, source, dur)
		}
	})
}

// headerCaptureWriter wraps ResponseWriter to capture response headers.
type headerCaptureWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *headerCaptureWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *headerCaptureWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// FmtDuration formats duration for logs: 148µs / 2.0ms / 1.23s / 90.0s
func FmtDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000)
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", d.Seconds())
	default:
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
}

// GetEnv returns env var value or default.
func GetEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// GetEnvInt returns env var as int or default.
func GetEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// NewLandingConfig creates a LandingConfig with a ToolLister that reads from
// the given MCPServer. Convenient one-liner for adding a landing page:
//
//	cfg.LandingPage = mcputil.NewLandingConfig(mcpServer, "my-server", "1.0.0", "18080")
func NewLandingConfig(s *server.MCPServer, name, version, port string) *LandingConfig {
	toolsMap := s.ListTools()
	tools := make([]ToolInfo, 0, len(toolsMap))
	for _, t := range toolsMap {
		tools = append(tools, ToolInfo{Name: t.Tool.Name, Description: t.Tool.Description})
	}
	return &LandingConfig{
		Name:     name,
		Version:  version,
		Endpoint: fmt.Sprintf("http://localhost:%s/mcp", strings.TrimPrefix(port, ":")),
		ToolLister: func() []ToolInfo {
			return tools
		},
	}
}

// renderLandingPage renders the embedded landing.html template.
func renderLandingPage(cfg *LandingConfig) http.Handler {
	tmpl, err := template.ParseFS(landingHTML, "landing.html")
	if err != nil {
		log.Printf("WARNING: failed to parse landing.html template: %v", err)
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "landing page template error", http.StatusInternalServerError)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, cfg); err != nil {
			log.Printf("WARNING: landing page render error: %v", err)
		}
	})
}
