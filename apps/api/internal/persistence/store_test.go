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
