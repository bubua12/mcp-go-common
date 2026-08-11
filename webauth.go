package mcputil

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// SessionCookieName is the browser session cookie used by WebAuthMiddleware.
	SessionCookieName = "mcp_web_session"

	defaultSessionTTL = 24 * time.Hour
)

// SessionStore issues and validates HMAC-signed session cookies derived from API_KEY.
// Safe for concurrent use. Use NewSessionStore.
type SessionStore struct {
	apiKey []byte
	salt   []byte
	ttl    time.Duration
}

// NewSessionStore creates a store bound to apiKey. ttl <= 0 uses 24h.
func NewSessionStore(apiKey string, ttl time.Duration) *SessionStore {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		sum := sha256.Sum256([]byte("mcp-go-common/session|" + apiKey))
		copy(salt, sum[:16])
	}
	return &SessionStore{
		apiKey: []byte(apiKey),
		salt:   salt,
		ttl:    ttl,
	}
}

// TTL returns the configured session lifetime.
func (s *SessionStore) TTL() time.Duration {
	if s == nil || s.ttl <= 0 {
		return defaultSessionTTL
	}
	return s.ttl
}

func (s *SessionStore) signingKey() []byte {
	h := hmac.New(sha256.New, s.apiKey)
	_, _ = h.Write([]byte("mcp-go-common/web-session/v1|"))
	_, _ = h.Write(s.salt)
	return h.Sum(nil)
}

// IssueToken returns a signed session token valid for TTL.
func (s *SessionStore) IssueToken() (string, time.Time, error) {
	if s == nil || len(s.apiKey) == 0 {
		return "", time.Time{}, fmt.Errorf("session store not configured")
	}
	exp := time.Now().Add(s.TTL())
	payload := strconv.FormatInt(exp.Unix(), 10)
	mac := hmac.New(sha256.New, s.signingKey())
	_, _ = mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
	return token, exp, nil
}

// Valid reports whether token is a non-expired session issued by this store.
func (s *SessionStore) Valid(token string) bool {
	if s == nil || token == "" || len(s.apiKey) == 0 {
		return false
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, s.signingKey())
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(sig, expected) != 1 {
		return false
	}
	expUnix, err := strconv.ParseInt(string(payload), 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < expUnix
}

// SetSessionCookie writes the session cookie on the response.
func (s *SessionStore) SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, exp time.Time) {
	maxAge := int(time.Until(exp).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   maxAge,
		Expires:  exp,
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsHTTPS(r),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func requestIsHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// SessionFromRequest returns the session cookie value if present.
func SessionFromRequest(r *http.Request) string {
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c == nil {
		return ""
	}
	return c.Value
}

// BearerToken extracts the Bearer token from Authorization, or empty.
func BearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) >= len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return strings.TrimSpace(h[len(prefix):])
	}
	return ""
}

func constantTimeEqualString(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// AuthOptions configures MCP/API authentication behavior.
type AuthOptions struct {
	// AllowSameOrigin, when true, skips auth for browser same-origin requests
	// (Origin scheme://Host matches). Default false — prefer session cookie after /login.
	AllowSameOrigin bool
	// Session, when non-nil, accepts a valid web session cookie as authentication
	// (in addition to Bearer API_KEY). Used so Landing Inspector works after login.
	Session *SessionStore
}

// AuthMiddleware validates API_KEY when non-empty.
// Accepted credentials:
//   - Authorization: Bearer <API_KEY>
//
// Same-origin bypass is OFF by default. Prefer AuthMiddlewareOpts / AuthMiddlewareFromEnv
// when you need session cookies or legacy same-origin.
func AuthMiddleware(apiKey string, next http.Handler) http.Handler {
	return AuthMiddlewareOpts(apiKey, next, AuthOptions{})
}

// AuthMiddlewareFromEnv like AuthMiddlewareOpts but reads MCP_ALLOW_SAME_ORIGIN.
func AuthMiddlewareFromEnv(apiKey string, next http.Handler, session *SessionStore) http.Handler {
	return AuthMiddlewareOpts(apiKey, next, AuthOptions{
		AllowSameOrigin: EnvTruthy("MCP_ALLOW_SAME_ORIGIN"),
		Session:         session,
	})
}

// AuthMiddlewareOpts is AuthMiddleware with explicit options.
func AuthMiddlewareOpts(apiKey string, next http.Handler, opts AuthOptions) http.Handler {
	if apiKey == "" {
		return next
	}
	log.Println("API_KEY configured, authentication enabled")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if AuthorizeRequest(apiKey, r, opts) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
}

// AuthorizeRequest reports whether r carries valid credentials for apiKey.
func AuthorizeRequest(apiKey string, r *http.Request, opts AuthOptions) bool {
	if apiKey == "" {
		return true
	}
	if tok := BearerToken(r); tok != "" && constantTimeEqualString(tok, apiKey) {
		return true
	}
	if opts.Session != nil && opts.Session.Valid(SessionFromRequest(r)) {
		return true
	}
	if opts.AllowSameOrigin && isSameOriginRequest(r) {
		return true
	}
	return false
}

func isSameOriginRequest(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || r.Host == "" {
		return false
	}
	// Keep parentheses so https://host is not accepted when Origin is only http://host partially matching.
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}

// EnvTruthy reports whether env key is a common true string.
func EnvTruthy(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// ParseSessionTTL parses durations like "24h", "12h", "30m". Empty/invalid → default 24h.
func ParseSessionTTL(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultSessionTTL
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return defaultSessionTTL
	}
	return d
}

// WebAuthMiddleware protects browser UI and XHR routes when apiKey is set.
//
//   - valid session cookie OR Bearer API_KEY → allow
//   - HTML navigations → 302 /login?next=...
//   - API / other → 401 JSON
//
// When apiKey is empty, this is a no-op.
// loginPath defaults to "/login".
func WebAuthMiddleware(apiKey string, session *SessionStore, next http.Handler) http.Handler {
	return WebAuthMiddlewarePath(apiKey, session, "/login", next)
}

// WebAuthMiddlewarePath is WebAuthMiddleware with a custom login path.
func WebAuthMiddlewarePath(apiKey string, session *SessionStore, loginPath string, next http.Handler) http.Handler {
	if apiKey == "" {
		return next
	}
	if session == nil {
		session = NewSessionStore(apiKey, defaultSessionTTL)
	}
	if loginPath == "" {
		loginPath = "/login"
	}
	opts := AuthOptions{Session: session}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == loginPath || r.URL.Path == "/logout" {
			next.ServeHTTP(w, r)
			return
		}
		if AuthorizeRequest(apiKey, r, opts) {
			next.ServeHTTP(w, r)
			return
		}
		if wantsHTMLLoginRedirect(r) {
			nextURL := r.URL.RequestURI()
			if nextURL == "" {
				nextURL = "/"
			}
			http.Redirect(w, r, loginPath+"?next="+queryEscape(nextURL), http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
}

func wantsHTMLLoginRedirect(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		return false
	}
	if strings.Contains(r.URL.Path, "/api/") {
		return false
	}
	// Browser document navigations and plain curl to pages should hit /login.
	if r.Header.Get("Sec-Fetch-Mode") == "navigate" {
		return true
	}
	accept := r.Header.Get("Accept")
	if accept == "" || accept == "*/*" {
		return true
	}
	if strings.Contains(accept, "text/html") {
		return true
	}
	if strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html") {
		return false
	}
	// e.g. text/html,application/xhtml+xml,... or broad browser Accept lists
	return true
}
