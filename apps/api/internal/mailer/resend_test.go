package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func client(t *testing.T, handler http.HandlerFunc) *Resend {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return NewAt("re_test_key", server.URL)
}

func TestSendPostsTheMessage(t *testing.T) {
	var body map[string]any
	var auth, contentType string
	r := client(t, func(w http.ResponseWriter, req *http.Request) {
		auth = req.Header.Get("Authorization")
		contentType = req.Header.Get("Content-Type")
		data, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(data, &body)
		w.Write([]byte(`{"id":"ae2014de-c168-4c61-8267-f689f1d6f1f3"}`))
	})
	if err := r.Send(context.Background(), "nao-responda@docping.com.br", "cliente@exemplo.com", "Confirme seu e-mail — DocPing", "Use este link", "<p>Use este link</p>"); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer re_test_key" || contentType != "application/json" {
		t.Fatalf("headers: %q %q", auth, contentType)
	}
	if body["from"] != "nao-responda@docping.com.br" || body["subject"] != "Confirme seu e-mail — DocPing" || body["text"] != "Use este link" {
		t.Fatalf("payload: %+v", body)
	}
	if body["html"] != "<p>Use este link</p>" {
		t.Fatalf("html part missing: %+v", body)
	}
	to, ok := body["to"].([]any)
	if !ok || len(to) != 1 || to[0] != "cliente@exemplo.com" {
		t.Fatalf("recipient: %+v", body["to"])
	}
}

// A refusal must carry the provider's own words to the log, so an operator
// sees "domain is not verified" instead of a generic failure.
func TestSendReportsTheProvidersRefusal(t *testing.T) {
	r := client(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"name":"validation_error","message":"The docping.com.br domain is not verified."}`))
	})
	err := r.Send(context.Background(), "a@b.com", "c@d.com", "x", "y", "")
	var refusal *Error
	if !errors.As(err, &refusal) {
		t.Fatalf("expected a typed refusal: %v", err)
	}
	if refusal.Status != 403 || !strings.Contains(refusal.Message, "not verified") {
		t.Fatalf("refusal lost its detail: %+v", refusal)
	}
}

// 200 with no id means the call did not do what it appeared to. Reporting
// success there would record a delivery that never happened.
func TestSendRefusesAnAcknowledgementWithoutAnID(t *testing.T) {
	r := client(t, func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(`{}`))
	})
	if err := r.Send(context.Background(), "a@b.com", "c@d.com", "x", "y", ""); err == nil {
		t.Fatal("an empty acknowledgement was taken as sent")
	}
}

// The rate limit is the one failure worth repeating: a burst of reminders can
// cross it, and a second attempt a second later costs nothing.
func TestSendRetriesOnceOnTheRateLimit(t *testing.T) {
	var calls atomic.Int32
	r := client(t, func(w http.ResponseWriter, req *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(429)
			w.Write([]byte(`{"name":"rate_limit_exceeded","message":"Too many requests."}`))
			return
		}
		w.Write([]byte(`{"id":"ok"}`))
	})
	if err := r.Send(context.Background(), "a@b.com", "c@d.com", "x", "y", ""); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected one retry, got %d call(s)", got)
	}
}

// Anything else is reported, never repeated: a repeated send is a duplicate
// message to a customer.
func TestSendDoesNotRetryOtherFailures(t *testing.T) {
	var calls atomic.Int32
	r := client(t, func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		w.WriteHeader(500)
		w.Write([]byte(`{"name":"internal_error","message":"boom"}`))
	})
	if err := r.Send(context.Background(), "a@b.com", "c@d.com", "x", "y", ""); err == nil {
		t.Fatal("a server error was swallowed")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("a non-rate-limit failure was repeated: %d calls", got)
	}
}

func TestSendGivesUpWhenTheContextEnds(t *testing.T) {
	r := client(t, func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"name":"rate_limit_exceeded"}`))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Send(ctx, "a@b.com", "c@d.com", "x", "y", ""); err == nil {
		t.Fatal("a cancelled context still sent")
	}
}

// A message with no html is sent as text alone; the field must be absent
// rather than empty, so the provider does not render a blank html part.
func TestSendOmitsAnEmptyHTMLPart(t *testing.T) {
	var body map[string]any
	r := client(t, func(w http.ResponseWriter, req *http.Request) {
		data, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(data, &body)
		w.Write([]byte(`{"id":"msg"}`))
	})
	if err := r.Send(context.Background(), "a@b.com", "c@d.com", "x", "y", ""); err != nil {
		t.Fatal(err)
	}
	if _, present := body["html"]; present {
		t.Fatalf("an empty html part was sent: %+v", body)
	}
}
