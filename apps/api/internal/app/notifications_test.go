package app

import (
	"context"
	"docping/api/internal/domain"
	"encoding/json"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"
)

func smtpFixture(t *testing.T) (string, string, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("local SMTP listener unavailable:", err)
	}
	t.Cleanup(func() { listener.Close() })
	messages := make(chan string, 10)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveSMTPConn(conn, messages)
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	return host, port, messages
}
func TestSMTPAcceptedAndSensitiveCode(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost = host
	app.Config.SMTPPort = port
	app.Config.SMTPFrom = "docping@example.invalid"
	app.Config.SMTPRequireTLS = false
	s := registerTest(t, h, "sensitive@example.invalid")
	r := createFlow(t, h, s)
	status, data, _ := req(t, h, "POST", "/api/requests/"+r.ID+"/notify", map[string]string{"channel": "email"}, s)
	if status != 200 || !strings.Contains(string(data), `"status":"accepted"`) {
		t.Fatalf("SMTP acceptance: %d %s", status, data)
	}
	select {
	case <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP did not receive message")
	}
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		v := st.Requests[r.ID]
		v.Sensitive = true
		st.Requests[r.ID] = v
		return nil
	})
	_, data, _ = req(t, h, "POST", "/api/requests/"+r.ID+"/share", nil, s)
	var link struct{ Token string }
	mustJSON(t, data, &link)
	status, _, _ = req(t, h, "GET", "/api/portal/"+link.Token, nil, testSession{})
	if status != 403 {
		t.Fatal("sensitive request exposed")
	}
	status, data, _ = req(t, h, "POST", "/api/portal/"+link.Token+"/code", nil, testSession{})
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var email string
	select {
	case email = <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("code not sent")
	}
	code := regexp.MustCompile(`\b\d{6}\b`).FindString(email)
	if code == "" {
		t.Fatal("no code in SMTP message")
	}
	status, data, verifiedResponse := req(t, h, "POST", "/api/portal/"+link.Token+"/verify", map[string]string{"code": code}, testSession{})
	if status != 200 || !strings.Contains(string(data), `"verified":true`) {
		t.Fatal(status, string(data))
	}
	status, _, _ = req(t, h, "GET", "/api/portal/"+link.Token, nil, testSession{})
	if status != 403 {
		t.Fatal("second browser bypassed sensitive confirmation", status)
	}
	portalCookie := verifiedResponse.Result().Cookies()[0]
	status, _, _ = req(t, h, "GET", "/api/portal/"+link.Token, nil, testSession{Cookie: portalCookie})
	if status != 200 {
		t.Fatal("verified portal blocked", status)
	}
}

func mustJSON(t *testing.T, data []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatal(err)
	}
}

func TestDemoCannotSendSensitiveCodes(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost = host
	app.Config.SMTPPort = port
	app.Config.SMTPFrom = "docping@example.invalid"
	s := registerTest(t, h, "demo-policy@example.invalid")
	r := createFlow(t, h, s)
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		o := st.Organizations[s.Org]
		o.Demo = true
		st.Organizations[o.ID] = o
		v := st.Requests[r.ID]
		v.Sensitive = true
		st.Requests[v.ID] = v
		return nil
	})
	_, data, _ := req(t, h, "POST", "/api/requests/"+r.ID+"/share", nil, s)
	var link struct{ Token string }
	mustJSON(t, data, &link)
	status, data, _ := req(t, h, "POST", "/api/portal/"+link.Token+"/code", nil, testSession{})
	if status != 403 || !strings.Contains(string(data), "demo_email_disabled") {
		t.Fatal(status, string(data))
	}
	select {
	case <-messages:
		t.Fatal("demo emitted external email")
	default:
	}
}

func TestCompletedBeforeDispatchIsNotEmailed(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost = host
	app.Config.SMTPPort = port
	app.Config.SMTPFrom = "docping@example.invalid"
	s := registerTest(t, h, "dispatch@example.invalid")
	r := createFlow(t, h, s)
	var effect func() error
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		_, effect = app.prepareDelivery(st, r, "reminder_3", "email")
		return nil
	})
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		v := st.Requests[r.ID]
		v.Status = "completed"
		st.Requests[v.ID] = v
		return nil
	})
	if err := effect(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-messages:
		t.Fatal("request completed before dispatch was emailed")
	default:
	}
}
