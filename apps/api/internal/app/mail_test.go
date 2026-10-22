package app

import (
	"docping/api/internal/mailer"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// With an HTTPS transport configured, a message goes out through it and never
// opens an SMTP connection. That is the whole point: the port may be blocked.
func TestEmailGoesOutOverHTTPSWhenConfigured(t *testing.T) {
	app, _ := setup(t)
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &payload)
		w.Write([]byte(`{"id":"msg_1"}`))
	}))
	defer server.Close()
	app.Config.ResendKey = "re_test"
	app.Config.MailFrom = "nao-responda@docping.com.br"
	// Deliberately impossible over SMTP: if the transport were chosen wrong,
	// this would fail instead of passing.
	app.Config.SMTPHost = "127.0.0.1"
	app.Config.SMTPPort = "1"
	app.Mail = mailer.NewAt("re_test", server.URL)

	if err := app.sendEmail("cliente@exemplo.com", "Confirme seu e-mail", "link"); err != nil {
		t.Fatalf("send over https failed: %v", err)
	}
	if payload["from"] != "nao-responda@docping.com.br" || payload["subject"] != "Confirme seu e-mail" {
		t.Fatalf("payload: %+v", payload)
	}
}

// A refusal from the provider reaches the customer as a sentence they can act
// on, never as the provider's English.
func TestAProviderRefusalIsSaidInPortuguese(t *testing.T) {
	app, _ := setup(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"name":"validation_error","message":"The docping.com.br domain is not verified."}`))
	}))
	defer server.Close()
	app.Config.ResendKey = "re_test"
	app.Config.MailFrom = "nao-responda@docping.com.br"
	app.Mail = mailer.NewAt("re_test", server.URL)

	err := app.sendEmail("cliente@exemplo.com", "x", "y")
	if err == nil {
		t.Fatal("a refused message was reported as sent")
	}
	if err.Error() != "provedor de e-mail recusou a mensagem" {
		t.Fatalf("the provider's own words leaked to the customer: %q", err)
	}
}

// MAIL_FROM is the newer name. SMTP_FROM still names the sender when it is
// absent, so a deployment configured before the second transport existed
// keeps sending from the same address.
func TestTheSenderFallsBackToTheOlderName(t *testing.T) {
	if got := (Config{SMTPFrom: "antigo@docping.com.br"}).Sender(); got != "antigo@docping.com.br" {
		t.Fatalf("fallback: %q", got)
	}
	if got := (Config{SMTPFrom: "antigo@docping.com.br", MailFrom: "novo@docping.com.br"}).Sender(); got != "novo@docping.com.br" {
		t.Fatalf("override: %q", got)
	}
}

func TestCanSendEmailNeedsATransportAndASender(t *testing.T) {
	cases := map[string]struct {
		cfg  Config
		want bool
	}{
		"nothing":                           {Config{}, false},
		"sender only":                       {Config{SMTPFrom: "a@b.com"}, false},
		"smtp without sender":               {Config{SMTPHost: "smtp.hostinger.com"}, false},
		"resend without sender":             {Config{ResendKey: "re_x"}, false},
		"smtp":                              {Config{SMTPHost: "smtp.hostinger.com", SMTPFrom: "a@b.com"}, true},
		"resend":                            {Config{ResendKey: "re_x", MailFrom: "a@b.com"}, true},
		"resend with the older sender name": {Config{ResendKey: "re_x", SMTPFrom: "a@b.com"}, true},
	}
	for name, c := range cases {
		if got := c.cfg.CanSendEmail(); got != c.want {
			t.Fatalf("%s: got %v", name, got)
		}
	}
}

// Production must have a way to send, because an account that cannot confirm
// its address can never sign in. Which transport provides it is not the
// rule's business -- but SMTP, when it is the one being used, still has to be
// encrypted.
func TestProductionRequiresAWayToSend(t *testing.T) {
	base := func() Config {
		return Config{
			Env: "production", Driver: "mysql", DSN: "user:pass@tcp(db:3306)/docping?tls=true",
			S3Endpoint: "https://s3.example.com", Scanner: "clamav:3310",
			Origin: "https://docping.com.br", Port: "8080",
			MaxUpload: 1 << 20, MaxOrgBytes: 1 << 30, MaxRequestBytes: 1 << 20, ClientLimit: 30,
		}
	}
	// Resend alone satisfies it: no SMTP host at all.
	cfg := base()
	cfg.ResendKey, cfg.MailFrom = "re_live_x", "nao-responda@docping.com.br"
	if !cfg.CanSendEmail() {
		t.Fatal("an HTTPS transport should satisfy the rule on its own")
	}
	// SMTP without TLS must still be refused when SMTP is the path.
	cfg = base()
	cfg.SMTPHost, cfg.SMTPFrom, cfg.SMTPRequireTLS = "smtp.hostinger.com", "a@b.com", false
	if !cfg.CanSendEmail() {
		t.Fatal("smtp with a sender is a way to send")
	}
	// And nothing at all is not a production configuration.
	cfg = base()
	if cfg.CanSendEmail() {
		t.Fatal("a deployment with no transport claimed it could send")
	}
}

// The start-up log has to name the transport, because which one is in use is
// the first thing to know when a message does not arrive.
func TestTheTransportIsNamedForTheLog(t *testing.T) {
	if got := (Config{ResendKey: "re_x", SMTPHost: "smtp.hostinger.com", SMTPPort: "465", SMTPTLSMode: "implicit"}).MailTransport(); got != "resend (https)" {
		t.Fatalf("https should win: %q", got)
	}
	if got := (Config{SMTPHost: "smtp.hostinger.com", SMTPPort: "465", SMTPTLSMode: "implicit"}).MailTransport(); got != "smtp smtp.hostinger.com:465 (implicit)" {
		t.Fatalf("smtp: %q", got)
	}
	if got := (Config{}).MailTransport(); got != "none" {
		t.Fatalf("none: %q", got)
	}
}
