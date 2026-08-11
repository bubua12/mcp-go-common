package mcputil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSessionStore_IssueAndValid(t *testing.T) {
	s := NewSessionStore("secret-key", time.Hour)
	tok, exp, err := s.IssueToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" || exp.Before(time.Now()) {
		t.Fatalf("bad token/exp: %q %v", tok, exp)
	}
	if !s.Valid(tok) {
		t.Fatal("expected valid token")
	}
	if s.Valid(tok + "x") {
		t.Fatal("tampered token should be invalid")
	}
	other := NewSessionStore("other-key", time.Hour)
	if other.Valid(tok) {
		t.Fatal("token must not validate under different API key")
	}
}

func TestSessionStore_Expired(t *testing.T) {
	s := NewSessionStore("secret-key", time.Millisecond)
	tok, _, err := s.IssueToken()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if s.Valid(tok) {
		t.Fatal("expected expired")
	}
}

func TestAuthMiddleware_Bearer(t *testing.T) {
	h := AuthMiddleware("k1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("unauth code=%d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer k1")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 || rr.Body.String() != "ok" {
		t.Fatalf("bearer got %d %q", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddleware_NoSameOriginByDefault(t *testing.T) {
	h := AuthMiddleware("k1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Host = "127.0.0.1:18090"
	req.Header.Set("Origin", "http://127.0.0.1:18090")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("same-origin should NOT bypass by default, code=%d", rr.Code)
	}
}

func TestAuthMiddleware_SessionCookie(t *testing.T) {
	store := NewSessionStore("k1", time.Hour)
	tok, _, err := store.IssueToken()
	if err != nil {
		t.Fatal(err)
	}
	h := AuthMiddlewareOpts("k1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}), AuthOptions{Session: store})

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("session cookie should auth, code=%d", rr.Code)
	}
}

func TestWebAuthMiddleware_RedirectAndAPI(t *testing.T) {
	store := NewSessionStore("k1", time.Hour)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("secret"))
	})
	h := WebAuthMiddleware("k1", store, inner)

	// HTML navigation → redirect
	req := httptest.NewRequest(http.MethodGet, "/history/", nil)
	req.Header.Set("Accept", "text/html")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "/login?next=") {
		t.Fatalf("location=%q", loc)
	}

	// API → 401 json
	req = httptest.NewRequest(http.MethodGet, "/history/api/stats", nil)
	req.Header.Set("Accept", "application/json")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("api want 401, got %d", rr.Code)
	}

	// with session → ok
	tok, _, _ := store.IssueToken()
	req = httptest.NewRequest(http.MethodGet, "/history/api/stats", nil)
	req.Header.Set("Accept", "application/json")
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tok})
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 200 || rr.Body.String() != "secret" {
		t.Fatalf("authed api got %d %q", rr.Code, rr.Body.String())
	}
}

func TestLoginHandler_OK(t *testing.T) {
	store := NewSessionStore("k1", time.Hour)
	h := LoginHandler("k1", store)

	form := strings.NewReader("api_key=k1&next=/history/")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusFound {
		t.Fatalf("login want 302, got %d body=%s", rr.Code, rr.Body.String())
	}
	if rr.Header().Get("Location") != "/history/" {
		t.Fatalf("location=%q", rr.Header().Get("Location"))
	}
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == SessionCookieName && c.Value != "" && store.Valid(c.Value) {
			found = true
		}
	}
	if !found {
		t.Fatal("expected session cookie")
	}
}

func TestLoginHandler_BadKey(t *testing.T) {
	store := NewSessionStore("k1", time.Hour)
	h := LoginHandler("k1", store)
	form := strings.NewReader("api_key=wrong&next=/")
	req := httptest.NewRequest(http.MethodPost, "/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("want 401, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !strings.Contains(string(body), "Invalid API key") {
		t.Fatalf("body=%s", body)
	}
}

func TestSafeNextPath(t *testing.T) {
	cases := map[string]string{
		"":                   "/",
		"/history/":          "/history/",
		"/history/api/x":     "/history/api/x",
		"//evil.com":         "/",
		"http://evil.com":    "/",
		"https://evil.com/a": "/",
		"history":            "/",
		"/ok?x=1":            "/ok?x=1",
	}
	for in, want := range cases {
		if got := safeNextPath(in); got != want {
			t.Errorf("safeNextPath(%q)=%q want %q", in, got, want)
		}
	}
}

func TestEmptyAPIKey_NoOp(t *testing.T) {
	innerCalled := false
	h := WebAuthMiddleware("", nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "/history/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if !innerCalled || rr.Code != 200 {
		t.Fatal("empty api key should pass through")
	}
}
