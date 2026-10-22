package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"docping/api/internal/analysis"
	"docping/api/internal/domain"
	"docping/api/internal/persistence"
	"docping/api/internal/storage"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type App struct {
	Config  Config
	Store   *persistence.Store
	Objects *storage.Objects
	limiter limiter
	// Analyzer and analysisJobs are nil unless AI_REVIEW_ENABLED is set, so
	// an unconfigured deployment behaves exactly as it did before.
	Analyzer     *analysis.Analyzer
	analysisJobs chan analysisJob
}
type APIError struct {
	Status        int
	Code, Message string
}

func (e *APIError) Error() string                 { return e.Message }
func fail(status int, code, message string) error { return &APIError{status, code, message} }
func hash(v string) string                        { sum := sha256.Sum256([]byte(v)); return hex.EncodeToString(sum[:]) }
func token() string                               { return domain.ID() + domain.ID() }

type call struct {
	A       *App
	W       http.ResponseWriter
	R       *http.Request
	S       *domain.State
	User    domain.User
	Session domain.Session
	Org     domain.Organization
	effects []func() error
}
type handler func(*call) (any, error)

func (a *App) routes() http.Handler {
	m := http.NewServeMux()
	a.handle(m, "GET /api/session", false, true, a.session)
	a.handle(m, "POST /api/auth/register", true, false, a.register)
	a.handle(m, "POST /api/auth/login", true, false, a.login)
	a.handle(m, "POST /api/auth/demo", true, false, a.demo)
	a.handle(m, "POST /api/auth/logout", true, true, a.logout)
	a.handle(m, "POST /api/auth/forgot-password", true, false, a.forgotPassword)
	a.handle(m, "POST /api/auth/reset-password", true, false, a.resetPassword)
	a.handle(m, "POST /api/auth/verify-email", true, false, a.verifyEmail)
	a.handle(m, "POST /api/auth/resend-verification", true, false, a.resendVerification)
	a.handle(m, "GET /api/clients", false, true, a.clients)
	a.handle(m, "GET /api/clients/{id}", false, true, a.client)
	a.handle(m, "POST /api/clients", true, true, a.createClient)
	a.handle(m, "PUT /api/clients/{id}", true, true, a.updateClient)
	a.handle(m, "POST /api/clients/import", true, true, a.importClients)
	a.handle(m, "GET /api/templates", false, true, a.templates)
	a.handle(m, "GET /api/templates/{id}", false, true, a.template)
	a.handle(m, "POST /api/templates", true, true, a.createTemplate)
	a.handle(m, "PUT /api/templates/{id}", true, true, a.updateTemplate)
	a.handle(m, "GET /api/requests", false, true, a.requests)
	a.handle(m, "POST /api/requests", true, true, a.createRequests)
	a.handle(m, "GET /api/requests/{id}", false, true, a.request)
	a.handle(m, "POST /api/requests/{id}/share", true, true, a.share)
	a.handle(m, "POST /api/requests/{id}/revoke", true, true, a.revoke)
	a.handle(m, "POST /api/requests/{id}/notify", true, true, a.notify)
	a.handle(m, "POST /api/requests/{id}/cancel", true, true, a.cancel)
	a.handle(m, "POST /api/requests/{requestID}/items/{itemID}/review", true, true, a.review)
	a.handle(m, "POST /api/requests/{requestID}/items/{itemID}/analyze", true, true, a.analyze)
	a.handle(m, "GET /api/portal/{token}", false, false, a.portal)
	a.handle(m, "POST /api/portal/{token}/code", true, false, a.portalCode)
	a.handle(m, "POST /api/portal/{token}/verify", true, false, a.portalVerify)
	m.HandleFunc("POST /api/portal/{token}/items/{itemID}/upload", a.uploadHTTP)
	a.handle(m, "GET /api/schedules", false, true, a.schedules)
	a.handle(m, "POST /api/schedules", true, true, a.createSchedule)
	a.handle(m, "PUT /api/schedules/{id}", true, true, a.updateSchedule)
	a.handle(m, "GET /api/schedules/{id}/runs", false, true, a.scheduleRuns)
	a.handle(m, "GET /api/organization", false, true, a.organization)
	a.handle(m, "PUT /api/organization", true, true, a.updateOrganization)
	a.handle(m, "GET /api/notifications", false, true, a.notifications)
	a.handle(m, "GET /api/audit", false, true, a.audits)
	m.HandleFunc("GET /api/files/{id}", a.file)
	m.HandleFunc("GET /api/requests/{id}/export", a.export)
	m.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, fail(404, "not_found", "Recurso não encontrado."))
	})
	return a.security(m)
}
func (a *App) Handler() http.Handler { return a.routes() }

// originError keeps the production message opaque while development names the
// expected origin, which is the single most common cause of a blank screen on
// a freshly configured machine.
func (a *App) originError(origin string) error {
	if a.Config.Env == "production" {
		return fail(403, "origin_rejected", "Origem não permitida.")
	}
	if origin == "" {
		return fail(403, "origin_rejected", "Origem não permitida. Abra o aplicativo em "+a.Config.Origin+" (requisição sem cabeçalho Origin e marcada como cross-site).")
	}
	return fail(403, "origin_rejected", "Origem não permitida: "+origin+". Abra o aplicativo em "+strings.Join(a.Config.AllowedOriginList(), " ou ")+", ou ajuste WEB_ORIGIN/WEB_ORIGINS.")
}
func (a *App) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; sandbox")
		if a.Config.Env == "production" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		w.Header().Set("Vary", "Origin")
		origin := r.Header.Get("Origin")
		allowed := origin != "" && a.Config.AllowedOrigin(origin)
		if origin != "" && !allowed {
			// Logged with the rejected value because a mismatch between the
			// browser address and WEB_ORIGIN is otherwise invisible from the
			// response alone.
			log.Printf("origin rejected: %q is not in the allowed set %v", origin, a.Config.AllowedOriginList())
			writeError(w, a.originError(origin))
			return
		}
		// An allowed Origin is this product's own app even when the browser
		// labels it cross-site, which happens whenever the SPA and the API are
		// served from different ports or hostnames. Writes still require the
		// CSRF token, so the check only has to stop unidentified cross-site
		// traffic.
		if !allowed && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeError(w, a.originError(origin))
			return
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
			w.WriteHeader(204)
			return
		}
		group, max, window := "api", 600, time.Minute
		if strings.Contains(r.URL.Path, "/auth/") {
			group, max = "auth", 20
		}
		// Creating a demonstration costs an organization with a dozen sample
		// clients and requests, so it is budgeted per hour rather than per
		// minute: a prospect needs one, not twenty.
		if r.URL.Path == "/api/auth/demo" {
			group, max, window = "demo", a.Config.DemoRateHourly, time.Hour
		}
		if strings.Contains(r.URL.Path, "/portal/") {
			group, max = "portal", 120
			if strings.HasSuffix(r.URL.Path, "/upload") {
				group, max = "upload", 20
			}
		}
		ip := a.Config.ClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
		if !a.limiter.Allow(group+":"+ip, max, window) {
			if group == "demo" {
				w.Header().Set("Retry-After", "3600")
				writeError(w, fail(429, "rate_limited", "Você já abriu demonstrações demais deste dispositivo. Tente novamente em uma hora."))
				return
			}
			w.Header().Set("Retry-After", "60")
			writeError(w, fail(429, "rate_limited", "Muitas tentativas. Aguarde um minuto."))
			return
		}
		defer func() {
			if recover() != nil {
				log.Print("request panic recovered")
				writeError(w, fail(500, "internal_error", "Não foi possível concluir a operação."))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func (a *App) handle(m *http.ServeMux, pattern string, write, auth bool, fn handler) {
	m.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if write {
			data, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if e != nil {
				writeError(w, fail(413, "payload_too_large", "Os dados excedem o limite permitido."))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(data))
		}
		var result any
		effects := []func() error{}
		ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
		defer cancel()
		err := a.Store.Run(ctx, write, func(s *domain.State) error {
			c := &call{A: a, W: w, R: r, S: s}
			if auth {
				if err := c.authenticate(write); err != nil {
					return err
				}
			}
			var err error
			result, err = fn(c)
			effects = c.effects
			return err
		})
		if err != nil {
			writeError(w, err)
			return
		}
		for _, effect := range effects {
			if err = effect(); err != nil {
				writeError(w, err)
				return
			}
		}
		writeJSON(w, 200, result)
	})
}
func (c *call) authenticate(write bool) error {
	cookie, err := c.R.Cookie("docping_session")
	if err != nil {
		return fail(401, "unauthorized", "Entre na sua conta para continuar.")
	}
	s, ok := c.S.Sessions[hash(cookie.Value)]
	if !ok || s.Revoked || expired(s.ExpiresAt) {
		return fail(401, "session_expired", "Sua sessão expirou. Entre novamente.")
	}
	u, ok := c.S.Users[s.UserID]
	if !ok {
		return fail(401, "unauthorized", "Sessão inválida.")
	}
	o, ok := c.S.Organizations[u.OrgID]
	if !ok {
		return fail(401, "unauthorized", "Escritório não encontrado.")
	}
	if write && rEqual(c.R.Header.Get("X-CSRF-Token"), s.CSRF) == false {
		return fail(403, "csrf_invalid", "Atualize a página e tente novamente.")
	}
	c.User = u
	c.Session = s
	c.Org = o
	return nil
}
func rEqual(a, b string) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	var equal byte
	for i := range a {
		equal |= a[i] ^ b[i]
	}
	return equal == 0
}
func expired(v string) bool {
	t, err := time.Parse(time.RFC3339, v)
	return err != nil || !time.Now().Before(t)
}
func decode(r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	if err := d.Decode(dst); err != nil {
		return fail(400, "invalid_json", "Verifique os dados enviados.")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fail(400, "invalid_json", "Envie um único objeto JSON.")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		v = map[string]bool{"ok": true}
	}
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	var api *APIError
	if !errors.As(err, &api) {
		log.Print("request failed (details suppressed)")
		api = &APIError{500, "internal_error", "Não foi possível concluir a operação. Tente novamente."}
	}
	writeJSON(w, api.Status, map[string]any{"error": map[string]string{"code": api.Code, "message": api.Message}})
}
func missing() error         { return fail(404, "not_found", "Recurso não encontrado.") }
func invalid(s string) error { return fail(422, "validation_error", s) }
func (a *App) StartWorker(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	a.tick(ctx, time.Now())
	a.cleanup(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.tick(ctx, now)
			a.cleanup(ctx, now)
		}
	}
}

type limitEntry struct {
	count int
	until time.Time
}
type limiter struct {
	mu      sync.Mutex
	entries map[string]limitEntry
}

func (l *limiter) Allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = map[string]limitEntry{}
	}
	now := time.Now()
	if len(l.entries) > 10000 {
		for k, e := range l.entries {
			if now.After(e.until) {
				delete(l.entries, k)
			}
		}
		if len(l.entries) > 10000 {
			return false
		}
	}
	e := l.entries[key]
	if now.After(e.until) {
		e = limitEntry{until: now.Add(window)}
	}
	e.count++
	l.entries[key] = e
	return e.count <= max
}
func (c *call) ownRequest(id string) (domain.Request, error) {
	r, ok := c.S.Requests[id]
	if !ok || r.OrgID != c.Org.ID {
		return r, missing()
	}
	return r, nil
}
func title(v string, n int) bool { return len(strings.TrimSpace(v)) > 0 && len(v) <= n }
