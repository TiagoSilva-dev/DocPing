package app

import (
	"bytes"
	"context"
	"docping/api/internal/domain"
	"docping/api/internal/persistence"
	"docping/api/internal/storage"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testSession struct {
	Cookie *http.Cookie
	CSRF   string
	Org    string
}

func setup(t *testing.T) (*App, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	store, err := persistence.Open("json", "", filepath.Join(dir, "state.json"), "development")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := storage.New(context.Background(), dir, "", "", "", "", "", "development", true)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	app := &App{Config: Config{Env: "development", Origin: "http://localhost:5173", ClientLimit: 30, MaxUpload: 20 << 20, Location: loc}, Store: store, Objects: obj}
	return app, app.Handler()
}
func req(t *testing.T, h http.Handler, method, path string, body any, s testSession) (int, []byte, *httptest.ResponseRecorder) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, e := json.Marshal(body)
		if e != nil {
			t.Fatal(e)
		}
		reader = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, reader)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost:5173")
	if s.Cookie != nil {
		r.AddCookie(s.Cookie)
	}
	if s.CSRF != "" {
		r.Header.Set("X-CSRF-Token", s.CSRF)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes(), w
}
func registerTest(t *testing.T, h http.Handler, email string) testSession {
	t.Helper()
	status, data, w := req(t, h, "POST", "/api/auth/register", map[string]string{"organizationName": "Escritório Teste", "name": "Mariana", "email": email, "password": "uma senha segura 123"}, testSession{})
	if status != 200 {
		t.Fatalf("register: %d %s", status, data)
	}
	var out struct {
		CSRFToken    string `json:"csrfToken"`
		Organization domain.Organization
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return testSession{Cookie: w.Result().Cookies()[0], CSRF: out.CSRFToken, Org: out.Organization.ID}
}
func createFlow(t *testing.T, h http.Handler, s testSession) domain.Request {
	t.Helper()
	status, data, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": "Cliente Exemplo", "email": "cliente@example.invalid"}, s)
	if status != 200 {
		t.Fatalf("client: %d %s", status, data)
	}
	var client domain.Client
	_ = json.Unmarshal(data, &client)
	_, data, _ = req(t, h, "GET", "/api/templates", nil, s)
	var templates []domain.Template
	_ = json.Unmarshal(data, &templates)
	status, data, _ = req(t, h, "POST", "/api/requests", map[string]any{"clientIds": []string{client.ID}, "templateId": templates[0].ID, "title": "Fechamento", "competence": "2026-09", "dueDate": "2026-10-25", "items": []domain.TemplateItem{{Name: "Extrato", Required: true, MaxFiles: 2}, {Name: "Opcional", Required: false, MaxFiles: 1}}}, s)
	if status != 200 {
		t.Fatalf("request: %d %s", status, data)
	}
	var requests []domain.Request
	_ = json.Unmarshal(data, &requests)
	return requests[0]
}
func TestAuthCSRFIsolationAndPersistence(t *testing.T) {
	app, h := setup(t)
	alice := registerTest(t, h, "alice@example.invalid")
	bob := registerTest(t, h, "bob@example.invalid")
	request := createFlow(t, h, alice)
	for _, path := range []string{"/api/requests/" + request.ID, "/api/clients/" + request.ClientID} {
		status, _, _ := req(t, h, "GET", path, nil, bob)
		if status != 404 {
			t.Fatalf("tenant leak at %s: %d", path, status)
		}
	}
	status, _, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": "Teste"}, testSession{Cookie: alice.Cookie})
	if status != 403 {
		t.Fatalf("missing CSRF: %d", status)
	}
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.invalid")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("cross origin: %d", w.Code)
	}
	status, _, _ = req(t, h, "POST", "/api/auth/login", map[string]string{"email": "alice@example.invalid", "password": "senha errada"}, testSession{})
	if status != 401 {
		t.Fatalf("incorrect password: %d", status)
	}
	status, data, _ := req(t, h, "POST", "/api/auth/login", map[string]string{"email": "alice@example.invalid", "password": "uma senha segura 123"}, testSession{})
	if status != 200 || bytes.Contains(data, []byte("passwordHash")) {
		t.Fatalf("login: %d %s", status, data)
	}
	var state *domain.State
	_ = app.Store.Run(context.Background(), false, func(s *domain.State) error { state = s; return nil })
	for _, u := range state.Users {
		if !strings.HasPrefix(u.StoredHash, "argon2id$") {
			t.Fatal("password is not Argon2id")
		}
	}
	status, _, _ = req(t, h, "POST", "/api/auth/logout", nil, alice)
	if status != 200 {
		t.Fatal(status)
	}
	status, _, _ = req(t, h, "GET", "/api/session", nil, alice)
	if status != 401 {
		t.Fatal("logout did not revoke session")
	}
}
func uploadTest(t *testing.T, h http.Handler, token, item string, contents []byte) (int, []byte) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "extrato.pdf")
	_, _ = part.Write(contents)
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/api/portal/"+token+"/items/"+item+"/upload", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code, w.Body.Bytes()
}
func TestUploadCorrectionApprovalAndPrivateDownloads(t *testing.T) {
	_, h := setup(t)
	s := registerTest(t, h, "review@example.invalid")
	r := createFlow(t, h, s)
	status, data, _ := req(t, h, "POST", "/api/requests/"+r.ID+"/share", map[string]any{}, s)
	if status != 200 {
		t.Fatalf("share %d %s", status, data)
	}
	var link struct{ Token string }
	_ = json.Unmarshal(data, &link)
	status, data = uploadTest(t, h, link.Token, r.Items[0].ID, samplePDF())
	if status != 200 {
		t.Fatalf("upload: %d %s", status, data)
	}
	var uploaded domain.Request
	_ = json.Unmarshal(data, &uploaded)
	if uploaded.Status != "under_review" {
		t.Fatal(uploaded.Status)
	}
	fileID := uploaded.Items[0].Files[0].ID
	status, _, _ = req(t, h, "GET", "/api/files/"+fileID, nil, testSession{})
	if status != 401 {
		t.Fatal("anonymous file access")
	}
	status, data, _ = req(t, h, "GET", "/api/files/"+fileID+"?token="+link.Token, nil, testSession{})
	if status != 200 || !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatal("authorized file unavailable")
	}
	status, data, _ = req(t, h, "POST", "/api/requests/"+r.ID+"/items/"+r.Items[0].ID+"/review", map[string]string{"action": "correct", "reason": "A imagem está cortada."}, s)
	if status != 200 {
		t.Fatalf("correction %d %s", status, data)
	}
	status, data = uploadTest(t, h, link.Token, r.Items[0].ID, samplePDF())
	if status != 200 {
		t.Fatal(status, string(data))
	}
	_ = json.Unmarshal(data, &uploaded)
	if len(uploaded.Items[0].Files) != 2 || !uploaded.Items[0].Files[0].Superseded {
		t.Fatal("history destroyed")
	}
	status, data, _ = req(t, h, "POST", "/api/requests/"+r.ID+"/items/"+r.Items[0].ID+"/review", map[string]string{"action": "approve"}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	_ = json.Unmarshal(data, &uploaded)
	if uploaded.Status != "completed" {
		t.Fatal("required completion failed", uploaded.Status)
	}
	status, _ = uploadTest(t, h, link.Token, r.Items[0].ID, samplePDF())
	if status != 409 {
		t.Fatal("completed request accepted upload")
	}
	status, _, _ = req(t, h, "GET", "/api/requests/"+r.ID+"/export", nil, s)
	if status != 200 {
		t.Fatal("export failed")
	}
	status, _, _ = req(t, h, "POST", "/api/requests/"+r.ID+"/revoke", nil, s)
	if status != 200 {
		t.Fatal(status)
	}
	status, _, _ = req(t, h, "GET", "/api/portal/"+link.Token, nil, testSession{})
	if status != 410 {
		t.Fatal("revoked token accepted")
	}
}
func TestExpiredTokensCSVAtomicAndUploadValidation(t *testing.T) {
	app, h := setup(t)
	s := registerTest(t, h, "csv@example.invalid")
	r := createFlow(t, h, s)
	_, data, _ := req(t, h, "POST", "/api/requests/"+r.ID+"/share", map[string]any{}, s)
	var link struct{ Token string }
	_ = json.Unmarshal(data, &link)
	status, _ := uploadTest(t, h, link.Token, r.Items[0].ID, []byte("this is not a PDF"))
	if status != 422 {
		t.Fatal("spoofed PDF accepted", status)
	}
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		v := st.Tokens[hash(link.Token)]
		v.ExpiresAt = time.Now().Add(-time.Hour).Format(time.RFC3339)
		st.Tokens[v.Hash] = v
		return nil
	})
	status, _, _ = req(t, h, "GET", "/api/portal/"+link.Token, nil, testSession{})
	if status != 410 {
		t.Fatal("expired token accepted")
	}
	status, data, _ = req(t, h, "POST", "/api/clients/import", map[string]any{"csv": "name,email\nGood,good@example.invalid\nBad,invalid\n", "commit": true}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var result struct {
		Imported int
		Errors   []any
	}
	_ = json.Unmarshal(data, &result)
	if result.Imported != 0 || len(result.Errors) != 1 {
		t.Fatal(string(data))
	}
	_, data, _ = req(t, h, "GET", "/api/clients", nil, s)
	var clients []domain.Client
	_ = json.Unmarshal(data, &clients)
	if len(clients) != 1 {
		t.Fatal("CSV partially committed")
	}
}
func TestRecurrenceIdempotencyAndMonthBoundary(t *testing.T) {
	app, h := setup(t)
	s := registerTest(t, h, "jobs@example.invalid")
	r := createFlow(t, h, s)
	loc := app.Config.Location
	now := time.Date(2027, 1, 31, 12, 0, 0, 0, loc)
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		schedule := domain.Schedule{ID: domain.ID(), OrgID: s.Org, Name: "Mensal", TemplateID: r.TemplateID, ClientIDs: []string{r.ClientID}, DayOfMonth: 31, DueDays: 10, Active: true, CreatedAt: time.Date(2026, 12, 1, 0, 0, 0, 0, loc).UTC().Format(time.RFC3339)}
		st.Schedules[schedule.ID] = schedule
		o := st.Organizations[s.Org]
		o.RemindersEnabled = false
		st.Organizations[s.Org] = o
		return nil
	})
	app.tick(context.Background(), now)
	app.tick(context.Background(), now)
	_ = app.Store.Run(context.Background(), false, func(st *domain.State) error {
		if len(st.Runs) != 1 {
			t.Fatalf("duplicate runs: %d", len(st.Runs))
		}
		for _, run := range st.Runs {
			if run.Competence != "2026-12" {
				t.Fatal(run.Competence)
			}
		}
		return nil
	})
	app.tick(context.Background(), time.Date(2027, 2, 28, 12, 0, 0, 0, loc))
	_ = app.Store.Run(context.Background(), false, func(st *domain.State) error {
		if len(st.Runs) != 2 {
			t.Fatal("short-month run missing")
		}
		return nil
	})
}
func TestScannerFailsClosedAndJSONProductionRejected(t *testing.T) {
	dir := t.TempDir()
	if _, err := persistence.Open("json", "", filepath.Join(dir, "state.json"), "production"); err == nil {
		t.Fatal("production JSON allowed")
	}
	obj, err := storage.New(context.Background(), dir, "", "", "", "", "", "development", false)
	if err != nil {
		t.Fatal(err)
	}
	status, err := obj.Scan(context.Background(), samplePDF())
	if err == nil || status != "quarantined" {
		t.Fatal("scanner silently bypassed")
	}
}

func TestReminderIdempotencyAndNoCompletedReminder(t *testing.T) {
	app, h := setup(t)
	s := registerTest(t, h, "reminders@example.invalid")
	r := createFlow(t, h, s)
	now := time.Now()
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		v := st.Requests[r.ID]
		v.CreatedAt = now.AddDate(0, 0, -8).UTC().Format(time.RFC3339)
		st.Requests[v.ID] = v
		return nil
	})
	app.tick(context.Background(), now)
	app.tick(context.Background(), now)
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		if len(st.Deliveries) != 1 {
			t.Fatalf("reminder duplicate/burst: %d", len(st.Deliveries))
		}
		for _, d := range st.Deliveries {
			if d.Kind != "reminder_6" || d.Status != "failed" {
				t.Fatalf("unexpected delivery: %+v", d)
			}
		}
		o := st.Organizations[s.Org]
		o.ReminderDays = []int{3, 6, 10}
		st.Organizations[o.ID] = o
		v := st.Requests[r.ID]
		v.Status = "completed"
		st.Requests[v.ID] = v
		return nil
	})
	app.tick(context.Background(), now.AddDate(0, 0, 20))
	_ = app.Store.Run(context.Background(), false, func(st *domain.State) error {
		if len(st.Deliveries) != 1 {
			t.Fatal("completed request reminded")
		}
		return nil
	})
}

func TestRecurringPreservesSensitiveAndCustomRequirements(t *testing.T) {
	app, h := setup(t)
	s := registerTest(t, h, "recurring-sensitive@example.invalid")
	base := createFlow(t, h, s)
	status, data, _ := req(t, h, "POST", "/api/requests", map[string]any{"clientIds": []string{base.ClientID}, "templateId": base.TemplateID, "title": "Sensível recorrente", "competence": "2026-09", "dueDate": "2026-10-25", "sensitive": true, "recurring": true, "items": []domain.TemplateItem{{Name: "Documento específico", Instruction: "Somente a página dois.", Required: true, MaxFiles: 1}}}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var initial []domain.Request
	mustJSON(t, data, &initial)
	first := initial[0]
	var schedule domain.Schedule
	_ = app.Store.Run(context.Background(), true, func(st *domain.State) error {
		schedule = st.Schedules[first.ScheduleID]
		schedule.CreatedAt = "2020-01-01T00:00:00Z"
		st.Schedules[schedule.ID] = schedule
		t := st.Templates[base.TemplateID]
		t.Items = []domain.TemplateItem{{ID: domain.ID(), Name: "Modelo alterado", Required: true, MaxFiles: 5}}
		st.Templates[t.ID] = t
		return nil
	})
	now := time.Now().In(app.Config.Location)
	future := domain.MonthDay(now.Year(), now.Month()+1, schedule.DayOfMonth, app.Config.Location).Add(12 * time.Hour)
	app.tick(context.Background(), future)
	_ = app.Store.Run(context.Background(), false, func(st *domain.State) error {
		found := false
		for _, r := range st.Requests {
			if r.ScheduleID == first.ScheduleID && r.ID != first.ID {
				found = true
				if !r.Sensitive || len(r.Items) != 1 || r.Items[0].Name != "Documento específico" || r.Items[0].Instruction != "Somente a página dois." {
					t.Fatalf("recurrence lost sensitive/snapshot: %+v", r)
				}
			}
		}
		if !found {
			t.Fatal("recurrence not generated")
		}
		return nil
	})
}
func TestCivilDueDaysAfter21hSaoPaulo(t *testing.T) {
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	now := time.Date(2026, 10, 19, 23, 30, 0, 0, loc)
	due := time.Date(2026, 10, 25, 0, 0, 0, 0, loc)
	if got := civilDaysBetween(now, due); got != 6 {
		t.Fatalf("due days after21h: %d", got)
	}
}
func TestCorrectionPreservesOtherFiles(t *testing.T) {
	_, h := setup(t)
	s := registerTest(t, h, "files@example.invalid")
	r := createFlow(t, h, s)
	_, data, _ := req(t, h, "POST", "/api/requests/"+r.ID+"/share", nil, s)
	var link struct{ Token string }
	mustJSON(t, data, &link)
	status, data := uploadTest(t, h, link.Token, r.Items[0].ID, samplePDF())
	if status != 200 {
		t.Fatal(status, string(data))
	}
	status, data = uploadTest(t, h, link.Token, r.Items[0].ID, append(samplePDF(), []byte("\n% second synthetic file")...))
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var uploaded domain.Request
	mustJSON(t, data, &uploaded)
	firstID := uploaded.Items[0].Files[0].ID
	secondID := uploaded.Items[0].Files[1].ID
	status, data, _ = req(t, h, "POST", "/api/requests/"+r.ID+"/items/"+r.Items[0].ID+"/review", map[string]string{"action": "correct", "reason": "Primeiro arquivo ilegível.", "fileId": firstID}, s)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	status, data = uploadTest(t, h, link.Token, r.Items[0].ID, samplePDF())
	if status != 200 {
		t.Fatal(status, string(data))
	}
	mustJSON(t, data, &uploaded)
	if len(uploaded.Items[0].Files) != 3 || !uploaded.Items[0].Files[0].Superseded {
		t.Fatal("target not superseded")
	}
	for _, f := range uploaded.Items[0].Files {
		if f.ID == secondID && f.Superseded {
			t.Fatal("unaffected file was discarded")
		}
	}
}

func TestPausedScheduleCannotReadAnotherOrganizationsTemplate(t *testing.T) {
	_, h := setup(t)
	alice := registerTest(t, h, "schedule-a@example.invalid")
	bob := registerTest(t, h, "schedule-b@example.invalid")
	r := createFlow(t, h, alice)
	other := createFlow(t, h, bob)
	status, data, _ := req(t, h, "POST", "/api/schedules", map[string]any{"name": "Mensal", "templateId": r.TemplateID, "clientIds": []string{r.ClientID}, "dayOfMonth": 5, "dueDays": 10, "active": true}, alice)
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var schedule domain.Schedule
	mustJSON(t, data, &schedule)
	status, data, _ = req(t, h, "PUT", "/api/schedules/"+schedule.ID, map[string]any{"templateId": other.TemplateID, "active": false}, alice)
	if status != 422 {
		t.Fatal("paused schedule leaked cross-tenant template", status, string(data))
	}
}

func TestFileTokensAreScopedAndQuotaRollback(t *testing.T) {
	app, h := setup(t)
	alice := registerTest(t, h, "private-a@example.invalid")
	bob := registerTest(t, h, "private-b@example.invalid")
	r := createFlow(t, h, alice)
	_, data, _ := req(t, h, "POST", "/api/requests/"+r.ID+"/share", nil, alice)
	var first struct{ Token string }
	mustJSON(t, data, &first)
	status, data := uploadTest(t, h, first.Token, r.Items[0].ID, samplePDF())
	if status != 200 {
		t.Fatal(status, string(data))
	}
	var uploaded domain.Request
	mustJSON(t, data, &uploaded)
	fileID := uploaded.Items[0].Files[0].ID
	status, _, _ = req(t, h, "GET", "/api/files/"+fileID, nil, bob)
	if status != 404 {
		t.Fatal("cross-org download", status)
	}
	other := createFlow(t, h, alice)
	_, data, _ = req(t, h, "POST", "/api/requests/"+other.ID+"/share", nil, alice)
	var second struct{ Token string }
	mustJSON(t, data, &second)
	status, _, _ = req(t, h, "GET", "/api/files/"+fileID+"?token="+second.Token, nil, testSession{})
	if status != 404 {
		t.Fatal("portal token escaped request scope", status)
	}
	app.Config.MaxOrgBytes = 1
	status, data = uploadTest(t, h, second.Token, other.Items[0].ID, samplePDF())
	if status != 422 || !strings.Contains(string(data), "storage_limit") {
		t.Fatal("quota failed", status, string(data))
	}
	_, data, _ = req(t, h, "GET", "/api/requests/"+other.ID, nil, alice)
	mustJSON(t, data, &uploaded)
	if len(uploaded.Items[0].Files) != 0 {
		t.Fatal("quota failure committed file")
	}
}
