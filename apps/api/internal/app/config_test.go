package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCanonicalOriginNormalizesAndRejects(t *testing.T) {
	for raw, want := range map[string]string{
		"http://LocalHost:5173":        "http://localhost:5173",
		"https://app.example.com/":     "https://app.example.com",
		"https://app.example.com:443":  "https://app.example.com",
		"http://app.example.com:80":    "http://app.example.com",
		"  https://app.example.com  ":  "https://app.example.com",
		"http://192.168.0.14:5173":     "http://192.168.0.14:5173",
		"https://APP.Example.com:8443": "https://app.example.com:8443",
	} {
		got, err := canonicalOrigin(raw)
		if err != nil || got != want {
			t.Fatalf("canonicalOrigin(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"", "localhost:5173", "ftp://app.example.com", "https://app.example.com/base", "https://app.example.com?a=1", "https://"} {
		if got, err := canonicalOrigin(raw); err == nil {
			t.Fatalf("canonicalOrigin(%q) accepted as %q", raw, got)
		}
	}
}

// Development has to accept every loopback name that reaches the same server;
// production must not grow aliases on its own.
func TestParseOriginsLoopbackAliasesAndExtras(t *testing.T) {
	dev, err := parseOrigins("http://localhost:5173", "", "development", "8080")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://[::1]:5173", "http://localhost:8080", "http://127.0.0.1:8080"} {
		if !(Config{Origins: dev}).AllowedOrigin(want) {
			t.Fatalf("development rejected %q (allowed: %v)", want, dev)
		}
	}
	if dev[0] != "http://localhost:5173" {
		t.Fatalf("canonical origin must stay first for link building: %v", dev)
	}
	if (Config{Origins: dev}).AllowedOrigin("http://evil.invalid") {
		t.Fatal("development accepted an unrelated origin")
	}
	prod, err := parseOrigins("https://app.example.com", " https://www.app.example.com , https://admin.example.com ", "production", "8080")
	if err != nil {
		t.Fatal(err)
	}
	if len(prod) != 3 || !(Config{Origins: prod}).AllowedOrigin("https://admin.example.com") {
		t.Fatalf("production extras not honoured: %v", prod)
	}
	if (Config{Origins: prod}).AllowedOrigin("http://app.example.com") {
		t.Fatal("production accepted a plaintext alias")
	}
	if _, err = parseOrigins("https://app.example.com", "not-a-url", "production", "8080"); err == nil {
		t.Fatal("invalid WEB_ORIGINS entry accepted")
	}
}

// A Config built by hand, as tests and embedders do, must not reject everything.
func TestAllowedOriginFallsBackToCanonical(t *testing.T) {
	c := Config{Origin: "http://localhost:5173"}
	if !c.AllowedOrigin("http://localhost:5173") || c.AllowedOrigin("http://127.0.0.1:5173") {
		t.Fatal("fallback must match exactly the canonical origin")
	}
}

func TestClientIPTrustsForwardedForOnlyFromTrustedProxy(t *testing.T) {
	networks, err := parseTrustedProxies("172.16.0.0/12, 10.1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	trusted := Config{TrustedProxies: networks}
	if got := trusted.ClientIP("172.18.0.5:41234", "203.0.113.9"); got != "203.0.113.9" {
		t.Fatalf("trusted proxy: %q", got)
	}
	if got := trusted.ClientIP("10.1.2.3:41234", "203.0.113.9, 172.18.0.5"); got != "203.0.113.9" {
		t.Fatalf("chained proxies: %q", got)
	}
	// An untrusted peer cannot claim a different address and get a fresh bucket.
	if got := trusted.ClientIP("203.0.113.50:5000", "1.1.1.1"); got != "203.0.113.50" {
		t.Fatalf("untrusted peer: %q", got)
	}
	if got := trusted.ClientIP("172.18.0.5:41234", "garbage"); got != "172.18.0.5" {
		t.Fatalf("unparsable header: %q", got)
	}
	plain := Config{}
	if got := plain.ClientIP("203.0.113.50:5000", "1.1.1.1"); got != "203.0.113.50" {
		t.Fatalf("no trusted proxies configured: %q", got)
	}
	if got := plain.ClientIP("unix-socket", ""); got != "unix-socket" {
		t.Fatalf("address without a port: %q", got)
	}
	if _, err = parseTrustedProxies("nonsense"); err == nil {
		t.Fatal("invalid CIDR accepted")
	}
}

// The reported origin_rejected: the API is reached through a loopback alias or
// a second configured hostname, not only through the exact WEB_ORIGIN string.
func TestSecurityAcceptsConfiguredAliasesAndStillBlocksForeignOrigins(t *testing.T) {
	app, h := setup(t)
	origins, err := parseOrigins("http://localhost:5173", "http://192.168.0.14:5173", "development", "8080")
	if err != nil {
		t.Fatal(err)
	}
	app.Config.Origins = origins
	for _, origin := range []string{"http://localhost:5173", "http://127.0.0.1:5173", "http://localhost:8080", "http://192.168.0.14:5173"} {
		r := httptest.NewRequest("GET", "/api/health", nil)
		r.Header.Set("Origin", origin)
		// A SPA served from another port is labelled cross-site by the browser.
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("origin %q rejected: %d %s", origin, w.Code, w.Body)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Fatalf("origin %q not echoed: %q", origin, w.Header().Get("Access-Control-Allow-Origin"))
		}
	}
	for _, origin := range []string{"http://evil.invalid", "http://localhost:5174", "https://localhost:5173"} {
		r := httptest.NewRequest("POST", "/api/auth/login", nil)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("origin %q accepted: %d", origin, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("rejected origin %q was echoed", origin)
		}
	}
	// No Origin at all is still refused when the browser calls it cross-site.
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross-site request without Origin accepted: %d", w.Code)
	}
}

func TestOPTIONSPreflightOnlyForAllowedOrigin(t *testing.T) {
	app, h := setup(t)
	app.Config.Origins = []string{"http://localhost:5173"}
	r := httptest.NewRequest(http.MethodOptions, "/api/session", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("preflight: %d %v", w.Code, w.Header())
	}
}

// Enabling the automated review without its dependencies, or with a threshold
// a typo turned into zero, would approve documents nobody checked. Config is
// the safety boundary: it has to refuse to start.
func TestLoadConfigRefusesIncompleteAnalysisSetup(t *testing.T) {
	base := map[string]string{
		"APP_ENV": "development", "STORE_DRIVER": "json", "WEB_ORIGIN": "http://localhost:5173",
		"AI_REVIEW_ENABLED": "true", "TYPESAFE_API_KEY": "k", "DOCLING_URL": "http://docling:5001",
	}
	for name, override := range map[string]map[string]string{
		"sem chave":            {"TYPESAFE_API_KEY": ""},
		"sem extrator":         {"DOCLING_URL": "", "PADDLEOCR_URL": ""},
		"endpoint sem TLS":     {"TYPESAFE_API_URL": "http://api.typesafe.ai"},
		"limiar zerado":        {"AI_APPROVE_CONFIDENCE": "0"},
		"limiar fora da faixa": {"AI_REJECT_CONFIDENCE": "1.4"},
		"janela de texto nula": {"AI_MAX_CHARS": "0"},
	} {
		t.Run(name, func(t *testing.T) {
			for k, v := range base {
				t.Setenv(k, v)
			}
			for k, v := range override {
				t.Setenv(k, v)
			}
			if _, err := LoadConfig(); err == nil {
				t.Fatal("configuração incompleta foi aceita")
			}
		})
	}
}

func TestLoadConfigAcceptsACompleteAnalysisSetup(t *testing.T) {
	for k, v := range map[string]string{
		"APP_ENV": "development", "STORE_DRIVER": "json", "WEB_ORIGIN": "http://localhost:5173",
		"AI_REVIEW_ENABLED": "true", "TYPESAFE_API_KEY": "k", "PADDLEOCR_URL": "http://paddleocr:8080",
	} {
		t.Setenv(k, v)
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !c.AIReview || c.JevModel != "jev-latest" || c.AIApprove != 0.70 {
		t.Fatalf("config = %+v", c)
	}
}

// Leaving it unset must change nothing: analysis off, every upload reviewed
// by a person, exactly as before the feature existed.
func TestLoadConfigLeavesAnalysisOffByDefault(t *testing.T) {
	for k, v := range map[string]string{"APP_ENV": "development", "STORE_DRIVER": "json", "WEB_ORIGIN": "http://localhost:5173"} {
		t.Setenv(k, v)
	}
	c, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.AIReview {
		t.Fatal("análise automática ligada sem configuração explícita")
	}
}
