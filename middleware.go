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
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	mcpweb "github.com/bubua12/mcp-go-common/web"
	"github.com/mark3labs/mcp-go/server"
)

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
	// instructions and MCP Inspector. Pass NewLandingConfig() to get the
	// standard page, or nil to skip.
	LandingPage *LandingConfig
}

// LandingConfig configures the landing page served at GET /.
type LandingConfig struct {
	// Name is the MCP server name shown on the page header.
	Name string
	// Description is a short description shown below the name.
	Description string
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

	// Landing page: React SPA + /api/info
	if cfg.LandingPage != nil {
		lp := cfg.LandingPage
		mux.Handle("/", spaHandler())
		mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(map[string]string{
				"name":        lp.Name,
				"description": lp.Description,
			})
		})
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

// NewLandingConfig creates a LandingConfig for the given server.
//
//	cfg.LandingPage = mcputil.NewLandingConfig("my-server", "统一日志查询 MCP Server")
func NewLandingConfig(name, description string) *LandingConfig {
	return &LandingConfig{Name: name, Description: description}
}

// spaHandler serves the embedded React SPA from web/dist.
// All non-file routes fall back to index.html (client-side routing).
func spaHandler() http.Handler {
	dist, err := fs.Sub(mcpweb.DistFS, "dist")
	if err != nil {
		log.Printf("WARNING: SPA dist not available: %v", err)
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "UI build not available", http.StatusInternalServerError)
		})
	}

	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		cleanPath := path.Clean(r.URL.Path)

		// Root → index.html
		if cleanPath == "/" {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		// Path with extension → try static file, 404 if missing
		if strings.Contains(path.Base(cleanPath), ".") {
			if _, err := fs.Stat(dist, strings.TrimPrefix(cleanPath, "/")); err == nil {
				r.URL.Path = cleanPath
				fileServer.ServeHTTP(w, r)
				return
			}
			http.NotFound(w, r)
			return
		}

		// All other paths → SPA fallback (index.html)
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// AuthMiddleware validates Bearer token when apiKey is non-empty.
// Same-origin requests (detected via Origin header matching Host) skip auth,
// allowing the browser-based MCP Inspector to connect without an API key.
func AuthMiddleware(apiKey string, next http.Handler) http.Handler {
	if apiKey == "" {
		return next
	}
	log.Println("API_KEY configured, authentication enabled")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Same-origin check: browser requests (e.g. MCP Inspector) send Origin = Host.
		// External clients (Claude Code, curl) typically don't send Origin, so they still need auth.
		if origin := r.Header.Get("Origin"); origin != "" {
			host := r.Host
			if host != "" && origin == "http://"+host || origin == "https://"+host {
				next.ServeHTTP(w, r)
				return
			}
		}

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
