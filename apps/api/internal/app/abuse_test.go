package app

import (
	"context"
	"docping/api/internal/domain"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// backdate moves an organization's creation time into the past, which is how
// these tests reach an age the cleanup job considers expired.
func backdate(t *testing.T, app *App, org string, age time.Duration) {
	t.Helper()
	err := app.Store.Run(context.Background(), true, func(s *domain.State) error {
		o := s.Organizations[org]
		if o.ID == "" {
			t.Fatal("organization not found")
		}
		o.CreatedAt = time.Now().Add(-age).UTC().Format(time.RFC3339)
		s.Organizations[o.ID] = o
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func organizationCount(t *testing.T, app *App) int {
	t.Helper()
	n := 0
	_ = app.Store.Run(context.Background(), false, func(s *domain.State) error {
		n = len(s.Organizations)
		return nil
	})
	return n
}
func openDemo(t *testing.T, h http.Handler) (int, testSession) {
	t.Helper()
	status, data, w := req(t, h, "POST", "/api/auth/demo", map[string]string{}, testSession{})
	if status != 200 {
		return status, testSession{}
	}
	var out struct {
		CSRFToken    string `json:"csrfToken"`
		Organization domain.Organization
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("demo did not open a session")
	}
	return status, testSession{Cookie: cookies[0], CSRF: out.CSRFToken, Org: out.Organization.ID}
}

// A demonstration is a real organization, so the number of them that may
// exist has to be bounded even when the requests come from many addresses.
func TestDemoCeiling(t *testing.T) {
	app, h := setup(t)
	app.Config.DemoMaxActive = 2
	for n := 0; n < 2; n++ {
		if status, _ := openDemo(t, h); status != 200 {
			t.Fatalf("demo %d refused: %d", n+1, status)
		}
	}
	status, body, _ := req(t, h, "POST", "/api/auth/demo", map[string]string{}, testSession{})
	if status != 503 || !strings.Contains(string(body), "demo_unavailable") {
		t.Fatalf("third demo should be refused: %d %s", status, body)
	}
	// Once one expires and is collected, the slot is free again.
	var oldest string
	_ = app.Store.Run(context.Background(), false, func(s *domain.State) error {
		for id := range s.Organizations {
			oldest = id
			break
		}
		return nil
	})
	backdate(t, app, oldest, 48*time.Hour)
	app.cleanup(context.Background(), time.Now())
	if status, _ := openDemo(t, h); status != 200 {
		t.Fatalf("slot not freed after cleanup: %d", status)
	}
}

// The hourly budget is what stops one source from opening demonstrations in a
// loop; the per-minute auth budget was far too generous for a route this
// expensive.
func TestDemoRateLimitIsHourly(t *testing.T) {
	app, h := setup(t)
	app.Config.DemoRateHourly = 2
	for n := 0; n < 2; n++ {
		if status, _ := openDemo(t, h); status != 200 {
			t.Fatalf("demo %d refused: %d", n+1, status)
		}
	}
	status, _, w := req(t, h, "POST", "/api/auth/demo", map[string]string{}, testSession{})
	if status != 429 {
		t.Fatalf("expected 429, got %d", status)
	}
	if w.Header().Get("Retry-After") != "3600" {
		t.Fatalf("Retry-After should name the hour, got %q", w.Header().Get("Retry-After"))
	}
}

// An expired demonstration must disappear completely: the organization, its
// sample data and the session someone was using.
func TestExpiredDemoIsPurged(t *testing.T) {
	app, h := setup(t)
	_, demo := openDemo(t, h)
	_, keep := openDemo(t, h)
	if status, _, _ := req(t, h, "GET", "/api/session", nil, demo); status != 200 {
		t.Fatal("demo session should work before expiry")
	}
	backdate(t, app, demo.Org, 25*time.Hour)
	app.cleanup(context.Background(), time.Now())
	if status, _, _ := req(t, h, "GET", "/api/session", nil, demo); status != 401 {
		t.Fatalf("expired demo session still authenticates: %d", status)
	}
	if status, _, _ := req(t, h, "GET", "/api/session", nil, keep); status != 200 {
		t.Fatal("a demo inside its TTL must be left alone")
	}
	_ = app.Store.Run(context.Background(), false, func(s *domain.State) error {
		if _, ok := s.Organizations[demo.Org]; ok {
			t.Fatal("organization survived the purge")
		}
		for _, r := range s.Requests {
			if r.OrgID == demo.Org {
				t.Fatal("requests survived the purge")
			}
		}
		for _, c := range s.Clients {
			if c.OrgID == demo.Org {
				t.Fatal("clients survived the purge")
			}
		}
		for _, u := range s.Users {
			if u.OrgID == demo.Org {
				t.Fatal("user survived the purge")
			}
		}
		return nil
	})
}

// verificationMessage returns the address a confirmation was sent to and the
// token in its link, so a test with more than one pending account can tell
// the messages apart.
func verificationMessage(t *testing.T, messages <-chan string) (string, string) {
	t.Helper()
	select {
	case message := <-messages:
		link := regexp.MustCompile(`/confirmar-email\?token=([0-9a-f]+)`).FindStringSubmatch(message)
		to := regexp.MustCompile(`(?i)\r?\nTo: *([^\r\n]+)`).FindStringSubmatch(message)
		if link == nil || to == nil {
			t.Fatalf("message is not a confirmation: %s", message)
		}
		return strings.TrimSpace(to[1]), link[1]
	case <-time.After(5 * time.Second):
		t.Fatal("no confirmation e-mail was sent")
		return "", ""
	}
}
func verificationLink(t *testing.T, messages <-chan string) string {
	t.Helper()
	_, raw := verificationMessage(t, messages)
	return raw
}

// Registration must not hand out a workspace before the address is confirmed:
// that is what let one person open accounts in series.
func TestRegistrationRequiresConfirmedEmail(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost, app.Config.SMTPPort = host, port
	app.Config.SMTPFrom = "docping@example.invalid"
	app.Config.VerifyEmail = true

	credentials := map[string]string{"organizationName": "Escritório Teste", "name": "Mariana", "email": "pendente@example.invalid", "password": "uma senha segura 123"}
	status, data, w := req(t, h, "POST", "/api/auth/register", credentials, testSession{})
	if status != 200 || !strings.Contains(string(data), "verification_sent") {
		t.Fatalf("register: %d %s", status, data)
	}
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("registration opened a session before confirmation")
	}
	login := map[string]string{"email": credentials["email"], "password": credentials["password"]}
	status, data, _ = req(t, h, "POST", "/api/auth/login", login, testSession{})
	if status != 403 || !strings.Contains(string(data), "email_unverified") {
		t.Fatalf("unconfirmed login should be refused: %d %s", status, data)
	}
	raw := verificationLink(t, messages)
	status, data, w = req(t, h, "POST", "/api/auth/verify-email", map[string]string{"token": raw}, testSession{})
	if status != 200 {
		t.Fatalf("verify: %d %s", status, data)
	}
	var out struct{ CSRFToken string }
	_ = json.Unmarshal(data, &out)
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("confirmation did not open the session")
	}
	if status, _, _ = req(t, h, "GET", "/api/session", nil, testSession{Cookie: cookies[0], CSRF: out.CSRFToken}); status != 200 {
		t.Fatal("session from the confirmation link does not work")
	}
	// Single use, so a forwarded or leaked link cannot be replayed.
	if status, _, _ = req(t, h, "POST", "/api/auth/verify-email", map[string]string{"token": raw}, testSession{}); status != 410 {
		t.Fatalf("confirmation link should be single use: %d", status)
	}
	if status, data, _ = req(t, h, "POST", "/api/auth/login", login, testSession{}); status != 200 {
		t.Fatalf("login after confirmation: %d %s", status, data)
	}
}

// A registration nobody ever confirmed is exactly the leftover that used to
// accumulate; a confirmed one of the same age must not be touched.
func TestUnconfirmedRegistrationIsPurged(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost, app.Config.SMTPPort = host, port
	app.Config.SMTPFrom = "docping@example.invalid"
	app.Config.VerifyEmail = true

	pending := map[string]string{"organizationName": "Nunca Confirmou", "name": "Alguém", "email": "nunca@example.invalid", "password": "uma senha segura 123"}
	if status, data, _ := req(t, h, "POST", "/api/auth/register", pending, testSession{}); status != 200 {
		t.Fatalf("register: %d %s", status, data)
	}
	confirmed := map[string]string{"organizationName": "Confirmou", "name": "Outra", "email": "confirmou@example.invalid", "password": "uma senha segura 123"}
	if status, data, _ := req(t, h, "POST", "/api/auth/register", confirmed, testSession{}); status != 200 {
		t.Fatalf("register: %d %s", status, data)
	}
	// Confirm only the account that is supposed to confirm: the other link is
	// left unused, which is the state this test is about.
	confirmedLink := ""
	for n := 0; n < 2; n++ {
		to, raw := verificationMessage(t, messages)
		if to == "confirmou@example.invalid" {
			confirmedLink = raw
		}
	}
	if confirmedLink == "" {
		t.Fatal("no confirmation was addressed to the account that confirms")
	}
	if status, data, _ := req(t, h, "POST", "/api/auth/verify-email", map[string]string{"token": confirmedLink}, testSession{}); status != 200 {
		t.Fatalf("verify: %d %s", status, data)
	}
	var pendingOrg, confirmedOrg string
	_ = app.Store.Run(context.Background(), false, func(s *domain.State) error {
		for _, u := range s.Users {
			if u.Email == "nunca@example.invalid" {
				pendingOrg = u.OrgID
			}
			if u.Email == "confirmou@example.invalid" && !u.EmailPending {
				confirmedOrg = u.OrgID
			}
		}
		return nil
	})
	if pendingOrg == "" || confirmedOrg == "" {
		t.Fatal("fixture did not produce one pending and one confirmed account")
	}
	backdate(t, app, pendingOrg, 8*24*time.Hour)
	backdate(t, app, confirmedOrg, 8*24*time.Hour)
	app.cleanup(context.Background(), time.Now())
	_ = app.Store.Run(context.Background(), false, func(s *domain.State) error {
		if _, ok := s.Organizations[pendingOrg]; ok {
			t.Fatal("unconfirmed registration survived its TTL")
		}
		if _, ok := s.Organizations[confirmedOrg]; !ok {
			t.Fatal("a confirmed workspace was deleted")
		}
		return nil
	})
}

// Resending must not become a way to mail-bomb an address, and must not
// reveal whether an address has an account.
func TestResendVerificationThrottlesAndStaysNeutral(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost, app.Config.SMTPPort = host, port
	app.Config.SMTPFrom = "docping@example.invalid"
	app.Config.VerifyEmail = true

	credentials := map[string]string{"organizationName": "Escritório", "name": "Mariana", "email": "reenvio@example.invalid", "password": "uma senha segura 123"}
	if status, data, _ := req(t, h, "POST", "/api/auth/register", credentials, testSession{}); status != 200 {
		t.Fatalf("register: %d %s", status, data)
	}
	verificationLink(t, messages)

	status, first, _ := req(t, h, "POST", "/api/auth/resend-verification", map[string]string{"email": credentials["email"]}, testSession{})
	if status != 200 {
		t.Fatalf("resend: %d %s", status, first)
	}
	select {
	case <-messages:
		t.Fatal("a second link was sent within the throttle window")
	case <-time.After(300 * time.Millisecond):
	}
	// An address with no account gets the same answer as one that has it.
	_, unknown, _ := req(t, h, "POST", "/api/auth/resend-verification", map[string]string{"email": "ninguem@example.invalid"}, testSession{})
	if string(unknown) != string(first) {
		t.Fatalf("response reveals whether the account exists:\n%s\n%s", first, unknown)
	}
	// Past the window, a new link is sent.
	if err := app.Store.Run(context.Background(), true, func(s *domain.State) error {
		for h, v := range s.Verifications {
			v.SentAt = time.Now().Add(-10 * time.Minute).UTC().Format(time.RFC3339)
			s.Verifications[h] = v
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if status, data, _ := req(t, h, "POST", "/api/auth/resend-verification", map[string]string{"email": credentials["email"]}, testSession{}); status != 200 {
		t.Fatalf("resend after window: %d %s", status, data)
	}
	verificationLink(t, messages)
}
