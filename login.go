package mcputil

import (
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// RegisterAuthRoutes mounts /login and /logout when apiKey is non-empty.
// session must be non-nil when apiKey is set.
func RegisterAuthRoutes(mux *http.ServeMux, apiKey string, session *SessionStore) {
	if apiKey == "" {
		return
	}
	if session == nil {
		session = NewSessionStore(apiKey, defaultSessionTTL)
	}
	mux.Handle("/login", LoginHandler(apiKey, session))
	mux.Handle("/logout", LogoutHandler(session))
}

// LoginHandler serves GET login form and POST credential check against apiKey.
func LoginHandler(apiKey string, session *SessionStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next := safeNextPath(r.URL.Query().Get("next"))
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			if session != nil && session.Valid(SessionFromRequest(r)) {
				http.Redirect(w, r, next, http.StatusFound)
				return
			}
			writeLoginPage(w, next, "")
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				writeLoginPage(w, next, "Invalid form")
				return
			}
			if n := safeNextPath(r.FormValue("next")); n != "" {
				next = n
			}
			key := r.FormValue("api_key")
			if !constantTimeEqualString(key, apiKey) {
				log.Printf("[webauth] login failed from %s", r.RemoteAddr)
				writeLoginPage(w, next, "Invalid API key")
				return
			}
			token, exp, err := session.IssueToken()
			if err != nil {
				http.Error(w, "session error", http.StatusInternalServerError)
				return
			}
			session.SetSessionCookie(w, r, token, exp)
			log.Printf("[webauth] login ok from %s", r.RemoteAddr)
			http.Redirect(w, r, next, http.StatusFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

// LogoutHandler clears the session cookie and redirects home.
func LogoutHandler(session *SessionStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ClearSessionCookie(w, r)
		http.Redirect(w, r, "/login?next=/", http.StatusFound)
	})
}

// safeNextPath allows only same-site relative paths.
func safeNextPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}
	// reject absolute / protocol-relative URLs
	if strings.HasPrefix(raw, "//") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" {
		return "/"
	}
	if !strings.HasPrefix(u.Path, "/") {
		return "/"
	}
	// rebuild path+query+fragment only
	out := u.Path
	if u.RawQuery != "" {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.Fragment
	}
	return out
}

func queryEscape(s string) string {
	return url.QueryEscape(s)
}

func writeLoginPage(w http.ResponseWriter, next, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if errMsg != "" {
		w.WriteHeader(http.StatusUnauthorized)
	}
	escNext := html.EscapeString(next)
	errHTML := ""
	if errMsg != "" {
		errHTML = fmt.Sprintf(`<div class="err">%s</div>`, html.EscapeString(errMsg))
	}
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>Login · MCP Server</title>
<style>
  :root {
    --bg:#0d1117; --card:#161b22; --border:#30363d; --text:#e6edf3;
    --mute:#8b949e; --blue:#58a6ff; --red:#f85149;
  }
  *{box-sizing:border-box}
  body{
    margin:0; min-height:100vh; display:flex; align-items:center; justify-content:center;
    font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;
    background:var(--bg); color:var(--text);
  }
  .card{
    width:min(400px,92vw); background:var(--card); border:1px solid var(--border);
    border-radius:12px; padding:28px 26px 24px; box-shadow:0 16px 40px rgba(0,0,0,.45);
  }
  h1{margin:0 0 6px; font-size:18px; font-weight:600}
  p{margin:0 0 18px; color:var(--mute); font-size:13px; line-height:1.5}
  label{display:block; font-size:12px; color:var(--mute); margin-bottom:6px}
  input[type=password],input[type=text]{
    width:100%%; padding:10px 12px; border-radius:8px; border:1px solid var(--border);
    background:#0d1117; color:var(--text); font-size:14px; outline:none;
  }
  input:focus{border-color:var(--blue); box-shadow:0 0 0 3px rgba(56,139,253,.2)}
  button{
    margin-top:16px; width:100%%; padding:10px 14px; border:none; border-radius:8px;
    background:var(--blue); color:#fff; font-size:14px; font-weight:600; cursor:pointer;
  }
  button:hover{filter:brightness(1.08)}
  .err{
    margin:0 0 14px; padding:8px 10px; border-radius:6px; font-size:13px;
    background:rgba(248,81,73,.12); color:var(--red); border:1px solid rgba(248,81,73,.35);
  }
  .hint{margin-top:14px; font-size:11px; color:var(--mute); line-height:1.45}
  code{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace; font-size:11px}
</style>
</head>
<body>
  <div class="card">
    <h1>MCP Server Login</h1>
    <p>Enter the same <code>API_KEY</code> used by MCP clients (<code>Authorization: Bearer …</code>).</p>
    %s
    <form method="post" action="/login" autocomplete="current-password">
      <input type="hidden" name="next" value="%s"/>
      <label for="api_key">API Key</label>
      <input id="api_key" name="api_key" type="password" required autofocus placeholder="API_KEY"/>
      <button type="submit">Sign in</button>
    </form>
    <div class="hint">Session cookie is HttpOnly · valid until expiry or server key change.</div>
  </div>
</body>
</html>`, errHTML, escNext)
}
