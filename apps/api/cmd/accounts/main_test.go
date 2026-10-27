package main

import (
	"context"
	"docping/api/internal/domain"
	"docping/api/internal/persistence"
	"path/filepath"
	"strings"
	"testing"
)

// The JSON store takes the same writes the MySQL one does, so the behaviour
// below is the behaviour against production -- without pointing a test at it.
func seeded(t *testing.T) *persistence.Store {
	t.Helper()
	store, err := persistence.Open("json", "", filepath.Join(t.TempDir(), "state.json"), "development")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Run(t.Context(), true, func(s *domain.State) error {
		s.Organizations["org-1"] = domain.Organization{ID: "org-1", Name: "Escritório de teste", Email: "tiago@docping.com.br", Plan: domain.PlanFounderPilot, CreatedAt: "2026-10-20T02:00:00Z"}
		s.Organizations["org-2"] = domain.Organization{ID: "org-2", Name: "Outro escritório", Email: "contato@outro.com.br", Plan: domain.PlanEssencial, CreatedAt: "2026-10-21T02:00:00Z"}
		s.Users["user-1"] = domain.User{ID: "user-1", OrgID: "org-1", Name: "Tiago", Email: "tiago@docping.com.br", EmailPending: true}
		s.Users["user-2"] = domain.User{ID: "user-2", OrgID: "org-2", Name: "Mariana", Email: "mariana@outro.com.br"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return store
}

func userOf(t *testing.T, store *persistence.Store, id string) domain.User {
	t.Helper()
	var out domain.User
	if err := store.Run(context.Background(), false, func(s *domain.State) error {
		out = s.Users[id]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func orgOf(t *testing.T, store *persistence.Store, id string) domain.Organization {
	t.Helper()
	var out domain.Organization
	if err := store.Run(context.Background(), false, func(s *domain.State) error {
		out = s.Organizations[id]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// Without -confirm the command is a rehearsal: it prints what it would do and
// writes nothing.
func TestWithoutConfirmNothingIsWritten(t *testing.T) {
	store := seeded(t)
	if err := retireAddresses(context.Background(), store, []string{"tiago@docping.com.br"}, false); err != nil {
		t.Fatal(err)
	}
	if got := userOf(t, store, "user-1").Email; got != "tiago@docping.com.br" {
		t.Fatalf("a rehearsal wrote: %q", got)
	}
}

func TestRetiringFreesTheAddress(t *testing.T) {
	store := seeded(t)
	if err := retireAddresses(context.Background(), store, []string{"tiago@docping.com.br"}, true); err != nil {
		t.Fatal(err)
	}
	u := userOf(t, store, "user-1")
	if u.Email == "tiago@docping.com.br" {
		t.Fatal("the address was not freed")
	}
	if !strings.HasSuffix(u.Email, "@docping-teste.invalid") {
		t.Fatalf("the replacement must be undeliverable: %q", u.Email)
	}
	// A confirmation pending on an address nobody will read would leave the
	// account unable to sign in with the password it already has.
	if u.EmailPending {
		t.Fatal("the pending confirmation was left behind")
	}
	// The workspace's own notification address was the same person's, so it
	// had to move too -- otherwise the product keeps mailing a stranger.
	if org := orgOf(t, store, "org-1"); org.Email != u.Email {
		t.Fatalf("organization still mails %q", org.Email)
	}
	// Nobody else was touched.
	if other := userOf(t, store, "user-2"); other.Email != "mariana@outro.com.br" {
		t.Fatalf("another account changed: %q", other.Email)
	}
	if org := orgOf(t, store, "org-2"); org.Email != "contato@outro.com.br" {
		t.Fatalf("another organization changed: %q", org.Email)
	}
}

// An organization whose notification address belongs to someone else keeps it:
// retiring one person's login is not a reason to silence the workspace.
func TestADifferentOrganizationAddressIsLeftAlone(t *testing.T) {
	store := seeded(t)
	if err := retireAddresses(context.Background(), store, []string{"mariana@outro.com.br"}, true); err != nil {
		t.Fatal(err)
	}
	if org := orgOf(t, store, "org-2"); org.Email != "contato@outro.com.br" {
		t.Fatalf("organization address changed: %q", org.Email)
	}
}

// A typo must not retire half the list. Nothing is written unless every
// address named was found.
func TestAnUnknownAddressChangesNothing(t *testing.T) {
	store := seeded(t)
	err := retireAddresses(context.Background(), store, []string{"tiago@docping.com.br", "ninguem@exemplo.com"}, true)
	if err == nil {
		t.Fatal("an unknown address was accepted")
	}
	if got := userOf(t, store, "user-1").Email; got != "tiago@docping.com.br" {
		t.Fatalf("a refused run still wrote: %q", got)
	}
}

func TestAnAccountCanBeNamedByID(t *testing.T) {
	store := seeded(t)
	if err := retireAddresses(context.Background(), store, []string{"user-2"}, true); err != nil {
		t.Fatal(err)
	}
	if got := userOf(t, store, "user-2").Email; !strings.HasSuffix(got, "@docping-teste.invalid") {
		t.Fatalf("id was not resolved: %q", got)
	}
}

// Two retirements must never produce the same address: it is a unique column,
// and a collision would fail the write.
func TestReplacementsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for n := 0; n < 500; n++ {
		address, err := testAddress()
		if err != nil {
			t.Fatal(err)
		}
		if seen[address] {
			t.Fatalf("collision on %q", address)
		}
		seen[address] = true
	}
}

// The listing has to tell a forgotten test apart from a workspace in use, so
// it counts what is inside each one.
func TestTheListingCountsWhatIsInside(t *testing.T) {
	store := seeded(t)
	if err := store.Run(context.Background(), true, func(s *domain.State) error {
		s.Clients["c1"] = domain.Client{ID: "c1", OrgID: "org-2", Name: "Padaria", Active: true}
		s.Requests["r1"] = domain.Request{ID: "r1", OrgID: "org-2", Items: []domain.RequestItem{{TemplateItem: domain.TemplateItem{ID: "i1"}, Files: []domain.DocumentFile{{ID: "f1"}, {ID: "f2"}}}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var accounts []account
	if err := store.Run(context.Background(), false, func(s *domain.State) error {
		accounts = collect(s)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected both organizations: %d", len(accounts))
	}
	// Sorted by creation, so org-1 comes first and org-2 second.
	if accounts[0].org.ID != "org-1" || accounts[1].org.ID != "org-2" {
		t.Fatalf("unexpected order: %s, %s", accounts[0].org.ID, accounts[1].org.ID)
	}
	if accounts[1].clients != 1 || accounts[1].requests != 1 || accounts[1].files != 2 {
		t.Fatalf("counts are wrong: %+v", accounts[1])
	}
	if accounts[0].clients != 0 || accounts[0].requests != 0 {
		t.Fatalf("counts leaked between organizations: %+v", accounts[0])
	}
}
