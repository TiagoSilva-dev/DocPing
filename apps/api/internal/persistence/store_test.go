package persistence

import (
	"context"
	"docping/api/internal/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONAtomicRollbackAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open("json", "", path, "development")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	org := domain.Organization{ID: domain.ID(), Name: "Teste", ClientLimit: 30}
	if err = store.Run(ctx, true, func(s *domain.State) error { s.Organizations[org.ID] = org; return nil }); err != nil {
		t.Fatal(err)
	}
	if err = store.Run(ctx, true, func(s *domain.State) error {
		v := s.Organizations[org.ID]
		v.Name = "Rollback"
		s.Organizations[v.ID] = v
		return context.Canceled
	}); err == nil {
		t.Fatal("mutation should fail")
	}
	reopened, err := Open("json", "", path, "development")
	if err != nil {
		t.Fatal(err)
	}
	_ = reopened.Run(ctx, false, func(s *domain.State) error {
		if s.Organizations[org.ID].Name != "Teste" {
			t.Fatal("rollback or persistence failed")
		}
		return nil
	})
}

// Opt-in integration uses an explicitly disposable MySQL database. It never
// creates the database or deletes existing data; each fixture has random IDs.
func TestMySQLRoundTrip(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to run actual MySQL8 integration")
	}
	store, err := Open("mysql", dsn, "", "development")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	o := domain.Organization{ID: domain.ID(), Name: "Integration fixture", ClientLimit: 30}
	client := domain.Client{ID: domain.ID(), OrgID: o.ID, Name: "Synthetic company", Active: true}
	template := domain.Template{ID: domain.ID(), OrgID: o.ID, Name: "Integration template", Active: true, Items: []domain.TemplateItem{{ID: domain.ID(), Name: "Synthetic PDF", Required: true, MaxFiles: 1}}}
	r := domain.Request{ID: domain.ID(), OrgID: o.ID, ClientID: client.ID, ClientName: client.Name, TemplateID: template.ID, Title: "Synthetic request", Competence: "2026-09", DueDate: "2026-10-25", Status: "pending", Items: []domain.RequestItem{{TemplateItem: domain.TemplateItem{ID: domain.ID(), Name: "Synthetic PDF", Required: true, MaxFiles: 1}, Status: "pending", Files: []domain.DocumentFile{}}}}
	if err = store.Run(ctx, true, func(s *domain.State) error {
		s.Organizations[o.ID] = o
		s.Clients[client.ID] = client
		s.Templates[template.ID] = template
		s.Requests[r.ID] = r
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err = store.Run(ctx, false, func(s *domain.State) error {
		got, ok := s.Requests[r.ID]
		if !ok || got.OrgID != o.ID || len(got.Items) != 1 {
			t.Fatal("MySQL aggregate did not persist")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// fixture builds one organization owning a row in every table that references
// it, which is what makes it useful for exercising deletion.
func fixture(org string) func(*domain.State) error {
	o := domain.Organization{ID: org, Name: "Purge fixture", ClientLimit: 30, CreatedAt: domain.Now()}
	user := domain.User{ID: domain.ID(), OrgID: org, Name: "Mariana", Email: "purge-" + org + "@example.invalid", CreatedAt: domain.Now()}
	client := domain.Client{ID: domain.ID(), OrgID: org, Name: "Synthetic company", Active: true}
	template := domain.Template{ID: domain.ID(), OrgID: org, Name: "Purge template", Active: true, Items: []domain.TemplateItem{{ID: domain.ID(), Name: "Synthetic PDF", Required: true, MaxFiles: 1}}}
	item := domain.RequestItem{TemplateItem: domain.TemplateItem{ID: domain.ID(), Name: "Synthetic PDF", Required: true, MaxFiles: 1}, Status: "uploaded", Files: []domain.DocumentFile{{ID: domain.ID(), Name: "exemplo.pdf", MIMEType: "application/pdf", ObjectKey: org + "/" + domain.ID(), ScanStatus: "clean", CreatedAt: domain.Now()}}}
	r := domain.Request{ID: domain.ID(), OrgID: org, ClientID: client.ID, ClientName: client.Name, TemplateID: template.ID, Title: "Purge request", Competence: "2026-09", DueDate: "2026-10-25", Status: "under_review", Items: []domain.RequestItem{item}}
	schedule := domain.Schedule{ID: domain.ID(), OrgID: org, Name: "Purge schedule", TemplateID: template.ID, ClientIDs: []string{client.ID}, DayOfMonth: 5, DueDays: 10, Active: true, CreatedAt: domain.Now()}
	return func(s *domain.State) error {
		s.Organizations[o.ID] = o
		s.Users[user.ID] = user
		s.Sessions["session-"+org] = domain.Session{Hash: "session-" + org, UserID: user.ID, CSRF: "csrf", ExpiresAt: domain.Now()}
		s.Resets["reset-"+org] = domain.PasswordReset{Hash: "reset-" + org, UserID: user.ID, ExpiresAt: domain.Now()}
		s.Verifications["verify-"+org] = domain.EmailVerification{Hash: "verify-" + org, UserID: user.ID, Email: user.Email, ExpiresAt: domain.Now(), SentAt: domain.Now()}
		s.Clients[client.ID] = client
		s.Templates[template.ID] = template
		s.Requests[r.ID] = r
		s.Schedules[schedule.ID] = schedule
		s.Runs[domain.ID()] = domain.ScheduleRun{ID: domain.ID(), OrgID: org, ScheduleID: schedule.ID, ClientID: client.ID, Competence: "2026-09", RequestID: r.ID, CreatedAt: domain.Now()}
		s.Tokens["token-"+org] = domain.PublicToken{Hash: "token-" + org, OrgID: org, RequestID: r.ID, ExpiresAt: domain.Now()}
		s.Deliveries[domain.ID()] = domain.Delivery{ID: domain.ID(), OrgID: org, RequestID: r.ID, Channel: "email", Kind: "initial", Status: "accepted", CreatedAt: domain.Now()}
		s.Audit(org, user.ID, "organization.registered", org)
		return nil
	}
}

// remaining counts the rows still owned by the organization across every
// collection, so the assertion cannot pass by deleting only the top-level row.
func remaining(s *domain.State, org string) int {
	n := 0
	users := map[string]bool{}
	for _, u := range s.Users {
		if u.OrgID == org {
			users[u.ID] = true
			n++
		}
	}
	for _, v := range s.Sessions {
		if users[v.UserID] {
			n++
		}
	}
	for _, v := range s.Resets {
		if users[v.UserID] {
			n++
		}
	}
	for _, v := range s.Verifications {
		if users[v.UserID] {
			n++
		}
	}
	for _, v := range s.Clients {
		if v.OrgID == org {
			n++
		}
	}
	for _, v := range s.Templates {
		if v.OrgID == org {
			n++
		}
	}
	for _, v := range s.Requests {
		if v.OrgID == org {
			n++
		}
	}
	for _, v := range s.Schedules {
		if v.OrgID == org {
			n++
		}
	}
	for _, v := range s.Runs {
		if v.OrgID == org {
			n++
		}
	}
	for _, v := range s.Tokens {
		if v.OrgID == org {
			n++
		}
	}
	for _, v := range s.Deliveries {
		if v.OrgID == org {
			n++
		}
	}
	for _, v := range s.Audits {
		if v.OrgID == org {
			n++
		}
	}
	if _, ok := s.Organizations[org]; ok {
		n++
	}
	return n
}
func TestJSONPurgeRemovesOnlyTheNamedOrganization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open("json", "", path, "development")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	doomed, kept := domain.ID(), domain.ID()
	for _, org := range []string{doomed, kept} {
		if err = store.Run(ctx, true, fixture(org)); err != nil {
			t.Fatal(err)
		}
	}
	before := 0
	_ = store.Run(ctx, false, func(s *domain.State) error { before = remaining(s, doomed); return nil })
	if before < 13 {
		t.Fatalf("fixture did not populate every collection: %d rows", before)
	}
	if err = store.PurgeOrganizations(ctx, []string{doomed}); err != nil {
		t.Fatal(err)
	}
	_ = store.Run(ctx, false, func(s *domain.State) error {
		if n := remaining(s, doomed); n != 0 {
			t.Fatalf("%d rows survived the purge", n)
		}
		if n := remaining(s, kept); n != before {
			t.Fatalf("purge touched another organization: %d of %d rows left", n, before)
		}
		return nil
	})
	// The deletion has to survive a reopen, not only the cached state.
	reopened, err := Open("json", "", path, "development")
	if err != nil {
		t.Fatal(err)
	}
	_ = reopened.Run(ctx, false, func(s *domain.State) error {
		if n := remaining(s, doomed); n != 0 {
			t.Fatalf("%d rows came back after reopen", n)
		}
		return nil
	})
}

// The order of the DELETE statements is only load-bearing against real
// foreign keys, so this is the test that can prove it. Opt-in, like the round
// trip above, and it only ever touches the organization it created.
func TestMySQLPurgeRespectsForeignKeys(t *testing.T) {
	dsn := os.Getenv("MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_TEST_DSN to run actual MySQL8 integration")
	}
	store, err := Open("mysql", dsn, "", "development")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	org := domain.ID()
	if err = store.Run(ctx, true, fixture(org)); err != nil {
		t.Fatal(err)
	}
	if err = store.PurgeOrganizations(ctx, []string{org}); err != nil {
		t.Fatal(err)
	}
	if err = store.Run(ctx, false, func(s *domain.State) error {
		if n := remaining(s, org); n != 0 {
			t.Fatalf("%d rows survived the purge", n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
