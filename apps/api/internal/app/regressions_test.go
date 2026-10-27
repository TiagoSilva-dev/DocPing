package app

import (
	"context"
	"docping/api/internal/domain"
	"strings"
	"testing"
	"time"
)

// An archived model used to vanish from the only screen that lists models, so
// it could never be reactivated.
func TestArchivedTemplateStaysListedAndCanBeReactivated(t *testing.T) {
	_, h := setup(t)
	s := registerTest(t, h, "templates-archive@example.invalid")
	_, data, _ := req(t, h, "GET", "/api/templates", nil, s)
	var before []domain.Template
	mustJSON(t, data, &before)
	if len(before) < 2 {
		t.Fatalf("starter templates missing: %d", len(before))
	}
	target := before[0]
	status, data, _ := req(t, h, "PUT", "/api/templates/"+target.ID, map[string]any{"active": false}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	_, data, _ = req(t, h, "GET", "/api/templates", nil, s)
	var after []domain.Template
	mustJSON(t, data, &after)
	if len(after) != len(before) {
		t.Fatalf("archived template disappeared: %d of %d", len(after), len(before))
	}
	var archived *domain.Template
	for n := range after {
		if after[n].ID == target.ID {
			archived = &after[n]
		}
	}
	if archived == nil || archived.Active {
		t.Fatalf("archived template not reported as archived: %+v", archived)
	}
	// A partial update must not wipe the fields it leaves out.
	if archived.Name != target.Name || archived.Description != target.Description || len(archived.Items) != len(target.Items) {
		t.Fatalf("partial update changed unrelated fields: %+v", archived)
	}
	status, data, _ = req(t, h, "PUT", "/api/templates/"+target.ID, map[string]any{"active": true}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	_, data, _ = req(t, h, "GET", "/api/templates", nil, s)
	after = nil
	mustJSON(t, data, &after)
	for _, v := range after {
		if v.ID == target.ID && !v.Active {
			t.Fatal("template could not be reactivated")
		}
	}
	// Archived models still may not start a request.
	_, data, _ = req(t, h, "POST", "/api/clients", map[string]string{"name": "Cliente Arquivo"}, s)
	var client domain.Client
	mustJSON(t, data, &client)
	_, _, _ = req(t, h, "PUT", "/api/templates/"+target.ID, map[string]any{"active": false}, s)
	status, data, _ = req(t, h, "POST", "/api/requests", map[string]any{"clientIds": []string{client.ID}, "templateId": target.ID, "title": "Teste", "competence": "2026-09", "dueDate": "2026-10-25"}, s)
	if status != 422 {
		t.Fatalf("archived template accepted for a new request: %d %s", status, data)
	}
}

// Omitting "active" used to archive the client as a side effect of the zero
// value, and omitting any other field used to clear it.
func TestClientUpdateKeepsFieldsAbsentFromPayload(t *testing.T) {
	_, h := setup(t)
	s := registerTest(t, h, "client-merge@example.invalid")
	_, data, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": "Horizonte", "cnpj": "12345678000199", "email": "contato@example.invalid", "contactName": "Ana", "phone": "5511999990000"}, s)
	var client domain.Client
	mustJSON(t, data, &client)
	status, data, _ := req(t, h, "PUT", "/api/clients/"+client.ID, map[string]string{"name": "Horizonte Arquitetura"}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var updated domain.Client
	mustJSON(t, data, &updated)
	if !updated.Active {
		t.Fatal("client archived by a payload that never mentioned active")
	}
	if updated.Name != "Horizonte Arquitetura" || updated.Email != client.Email || updated.CNPJ != client.CNPJ || updated.ContactName != client.ContactName || updated.Phone != client.Phone {
		t.Fatalf("partial update lost data: %+v", updated)
	}
	// An explicit empty string still clears the field.
	status, data, _ = req(t, h, "PUT", "/api/clients/"+client.ID, map[string]any{"name": "Horizonte Arquitetura", "cnpj": "", "active": false}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	mustJSON(t, data, &updated)
	if updated.CNPJ != "" || updated.Active {
		t.Fatalf("explicit values ignored: %+v", updated)
	}
}

// Recent iOS stamps mif1/msf1 as the major brand, so a real iPhone photo was
// being refused as an invalid file.
func TestDetectFileAcceptsIPhoneHEIFBrands(t *testing.T) {
	heif := func(brand string) []byte {
		data := make([]byte, 32)
		copy(data[4:], "ftyp")
		copy(data[8:], brand)
		return data
	}
	for _, brand := range []string{"heic", "heix", "heim", "heis", "hevc", "hevx", "mif1", "msf1"} {
		kind, err := detectFile("IMG_0042.HEIC", heif(brand))
		if err != nil || kind != "image/heic" {
			t.Fatalf("brand %q: %q %v", brand, kind, err)
		}
	}
	if _, err := detectFile("IMG_0042.heic", heif("avif")); err == nil {
		t.Fatal("unknown brand accepted as HEIC")
	}
	if _, err := detectFile("IMG_0042.pdf", heif("heic")); err == nil {
		t.Fatal("HEIC content accepted with a PDF extension")
	}
	if _, err := detectFile("extrato.pdf", samplePDF()); err != nil {
		t.Fatal("PDF rejected:", err)
	}
	if _, err := detectFile("extrato.png", samplePDF()); err == nil {
		t.Fatal("PDF accepted with a PNG extension")
	}
}

// A request whose documents are all in review is waiting on the office; a
// reminder there reads as if the upload had never arrived.
func TestNoReminderWhileEverythingIsUnderReview(t *testing.T) {
	app, h := setup(t)
	s := registerTest(t, h, "reminder-review@example.invalid")
	r := createFlow(t, h, s)
	now := time.Now()
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		v := st.Requests[r.ID]
		v.CreatedAt = now.AddDate(0, 0, -8).UTC().Format(time.RFC3339)
		for n := range v.Items {
			v.Items[n].Status = "uploaded"
		}
		domain.Recalculate(&v)
		if v.Status != "under_review" {
			t.Fatalf("precondition: %q", v.Status)
		}
		st.Requests[v.ID] = v
		return nil
	})
	app.tick(context.Background(), now)
	_ = app.Store.Run(context.Background(), false, func(st *domain.State) error {
		if len(st.Deliveries) != 0 {
			t.Fatalf("reminded a client who had already sent everything: %d", len(st.Deliveries))
		}
		return nil
	})
	// A document sent back for correction is the client's turn again.
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		v := st.Requests[r.ID]
		v.Items[0].Status = "correction_required"
		domain.Recalculate(&v)
		st.Requests[v.ID] = v
		return nil
	})
	app.tick(context.Background(), now)
	_ = app.Store.Run(context.Background(), false, func(st *domain.State) error {
		if len(st.Deliveries) != 1 {
			t.Fatalf("correction did not resume reminders: %d", len(st.Deliveries))
		}
		return nil
	})
}

func TestRecalculateCompletesSnapshotWithoutRequiredItems(t *testing.T) {
	r := domain.Request{Items: []domain.RequestItem{
		{TemplateItem: domain.TemplateItem{Name: "Opcional 1"}, Status: "approved"},
		{TemplateItem: domain.TemplateItem{Name: "Opcional 2"}, Status: "approved"},
	}}
	domain.Recalculate(&r)
	if r.Status != "completed" {
		t.Fatalf("all-optional request stuck at %q", r.Status)
	}
	r.Items[1].Status = "uploaded"
	domain.Recalculate(&r)
	if r.Status != "under_review" {
		t.Fatalf("pending review reported as %q", r.Status)
	}
}

// Asking for another code had to wait out the full ten minutes of validity,
// which strands a client who never received the first message.
func TestSensitiveCodeResendAfterShortCooldown(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost = host
	app.Config.SMTPPort = port
	app.Config.SMTPFrom = "docping@example.invalid"
	app.Config.SMTPRequireTLS = false
	s := registerTest(t, h, "code-resend@example.invalid")
	r := createFlow(t, h, s)
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		v := st.Requests[r.ID]
		v.Sensitive = true
		st.Requests[v.ID] = v
		return nil
	})
	_, data, _ := req(t, h, "POST", "/api/requests/"+r.ID+"/share", nil, s)
	var link struct{ Token string }
	mustJSON(t, data, &link)
	if status, data, _ := req(t, h, "POST", "/api/portal/"+link.Token+"/code", nil, testSession{}); status != 200 {
		t.Fatal(status, string(data))
	}
	select {
	case <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("first code not sent")
	}
	status, data, _ := req(t, h, "POST", "/api/portal/"+link.Token+"/code", nil, testSession{})
	if status != 429 {
		t.Fatalf("burst allowed: %d %s", status, data)
	}
	// Past the cooldown, still inside the ten-minute validity.
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		for key, token := range st.Tokens {
			token.CodeSentAt = time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
			st.Tokens[key] = token
		}
		return nil
	})
	status, data, _ = req(t, h, "POST", "/api/portal/"+link.Token+"/code", nil, testSession{})
	if status != 200 {
		t.Fatalf("resend refused after the cooldown: %d %s", status, data)
	}
	select {
	case <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("second code not sent")
	}
	// After a few quick retries the wait grows, so the endpoint cannot be used
	// to flood the client's inbox.
	for n := 0; n < 3; n++ {
		_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
			for key, tok := range st.Tokens {
				tok.CodeSentAt = time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)
				st.Tokens[key] = tok
			}
			return nil
		})
		status, data, _ = req(t, h, "POST", "/api/portal/"+link.Token+"/code", nil, testSession{})
		if status == 429 {
			return
		}
		if status != 200 {
			t.Fatalf("resend %d: %d %s", n, status, data)
		}
		select {
		case <-messages:
		case <-time.After(2 * time.Second):
			t.Fatal("code not sent")
		}
	}
	t.Fatal("resend cooldown never escalated")
}

// Accented subjects must travel as RFC 2047 encoded words, not raw UTF-8 in a
// header, and the message needs the Date/Message-ID a provider expects.
func TestEmailHeadersAreEncoded(t *testing.T) {
	app, _ := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost = host
	app.Config.SMTPPort = port
	app.Config.SMTPFrom = "docping@example.invalid"
	app.Config.SMTPRequireTLS = false
	if err := app.sendEmail("cliente@example.invalid", "Documentos solicitados — Escritório", "Olá, tudo bem?"); err != nil {
		t.Fatal(err)
	}
	var message string
	select {
	case message = <-messages:
	case <-time.After(2 * time.Second):
		t.Fatal("message not received")
	}
	headers, _, _ := strings.Cut(message, "\r\n\r\n")
	for _, want := range []string{"Subject: =?utf-8?", "Date: ", "Message-ID: <", "@example.invalid>"} {
		if !strings.Contains(headers, want) {
			t.Fatalf("missing %q in headers:\n%s", want, headers)
		}
	}
	for _, line := range strings.Split(headers, "\r\n") {
		for _, r := range line {
			if r > 127 {
				t.Fatalf("raw non-ASCII byte in header: %q", line)
			}
		}
	}
	if !strings.Contains(message, "Olá, tudo bem?") {
		t.Fatalf("body altered:\n%s", message)
	}
}

// A reset token pointing at a user that no longer exists used to create a
// phantom account keyed on the empty string.
func TestResetWithMissingUserDoesNotCreateAccount(t *testing.T) {
	app, h := setup(t)
	raw := token()
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		st.Resets[hash(raw)] = domain.PasswordReset{Hash: hash(raw), UserID: "nao-existe", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
		return nil
	})
	status, data, _ := req(t, h, "POST", "/api/auth/reset-password", map[string]string{"token": raw, "password": "uma senha segura 123"}, testSession{})
	if status != 410 {
		t.Fatalf("dangling reset accepted: %d %s", status, data)
	}
	_ = app.Store.Run(context.Background(), false, func(st *domain.State) error {
		if len(st.Users) != 0 {
			t.Fatalf("phantom user created: %+v", st.Users)
		}
		return nil
	})
}

// A newly registered office must not start deciding on its own: the thresholds
// are uncalibrated (docs/06-validacao-da-entrega.md) and the landing page tells
// prospects that "sugerir" is the default. The demo organization is deliberately
// created in auto so the feature is visible while exploring.
func TestRegisterStartsInSuggestMode(t *testing.T) {
	a, h := setup(t)
	registerTest(t, h, "padrao@escritorio.test")
	var mode string
	if err := a.Store.Run(context.Background(), false, func(s *domain.State) error {
		for _, o := range s.Organizations {
			if !o.Demo {
				mode = o.AIReview
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if mode != domain.AIReviewSuggest {
		t.Fatalf("escritório novo começou em %q, esperado %q", mode, domain.AIReviewSuggest)
	}
}
