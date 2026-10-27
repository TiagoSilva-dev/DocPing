// Command accounts is an operator tool for the accounts in a deployment. It
// lists them, and it retires the address of one that was only ever a test.
//
// Retiring an address is not deleting the workspace. It frees a real e-mail
// that a test signup took -- addresses are unique, so the real person cannot
// register again while a test holds theirs -- and it makes sure nothing this
// product ever sends can reach a stranger: the replacement lives under
// .invalid, a domain RFC 2606 reserves so that it can never resolve.
//
// Why a command and not SQL: the address is stored twice, in the unique
// column and inside the JSON aggregate, and those two are written together by
// one upsert. Editing the column alone leaves the aggregate carrying the old
// address, and the next transaction writes it straight back. Going through
// Store.Run is what keeps the two in agreement.
//
// It reads only MYSQL_DSN, MYSQL_CA_CERT and, when the database is reached
// through a TCP proxy, MYSQL_TLS_SERVER_NAME. It never touches the bucket,
// the scanner or SMTP, so it runs from a laptop against a database reached
// over a TCP proxy without any of the other production services in reach.
//
//	go run ./cmd/accounts                                   # list
//	go run ./cmd/accounts -retire a@b.com,c@d.com -confirm  # retire addresses
package main

import (
	"context"
	"crypto/rand"
	"docping/api/internal/domain"
	"docping/api/internal/persistence"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	retire := flag.String("retire", "", "comma-separated e-mail addresses (or user ids) to retire")
	confirm := flag.Bool("confirm", false, "actually write; without it the change is only printed")
	flag.Parse()

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN is required")
	}
	if ca := os.Getenv("MYSQL_CA_CERT"); ca != "" {
		// MYSQL_TLS_SERVER_NAME is for reaching the database through a TCP
		// proxy: the certificate names the private host, the dialled address
		// is the proxy's. Naming it keeps both the chain and the hostname
		// checked; leaving it empty keeps the driver's own behaviour.
		if err := persistence.RegisterCAWithName(ca, os.Getenv("MYSQL_TLS_SERVER_NAME")); err != nil {
			log.Fatal(err)
		}
	} else if strings.Contains(dsn, "tls="+persistence.CATLSConfig) {
		log.Fatalf("this DSN asks for tls=%s, which needs MYSQL_CA_CERT", persistence.CATLSConfig)
	}
	store, err := persistence.Open("mysql", dsn, "", "production")
	if err != nil {
		log.Fatal("database unavailable: ", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if *retire == "" {
		if err := list(ctx, store); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := retireAddresses(ctx, store, strings.Split(*retire, ","), *confirm); err != nil {
		log.Fatal(err)
	}
}

type account struct {
	org   domain.Organization
	users []domain.User
	// Counts are what tells a forgotten test apart from a workspace someone
	// is using: an account with real clients and requests is not a leftover.
	clients, requests, files int
}

func collect(s *domain.State) []account {
	byOrg := map[string]*account{}
	for id, o := range s.Organizations {
		byOrg[id] = &account{org: o}
	}
	for _, u := range s.Users {
		if a := byOrg[u.OrgID]; a != nil {
			a.users = append(a.users, u)
		}
	}
	for _, c := range s.Clients {
		if a := byOrg[c.OrgID]; a != nil {
			a.clients++
		}
	}
	for _, r := range s.Requests {
		a := byOrg[r.OrgID]
		if a == nil {
			continue
		}
		a.requests++
		for _, item := range r.Items {
			a.files += len(item.Files)
		}
	}
	out := make([]account, 0, len(byOrg))
	for _, a := range byOrg {
		sort.Slice(a.users, func(i, j int) bool { return a.users[i].Email < a.users[j].Email })
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].org.CreatedAt < out[j].org.CreatedAt })
	return out
}

func list(ctx context.Context, store *persistence.Store) error {
	var accounts []account
	if err := store.Run(ctx, false, func(s *domain.State) error {
		accounts = collect(s)
		return nil
	}); err != nil {
		return err
	}
	fmt.Printf("%d organização(ões)\n\n", len(accounts))
	for _, a := range accounts {
		kind := "escritório"
		if a.org.Demo {
			kind = "demonstração"
		}
		plan := a.org.Plan
		if plan == "" {
			plan = "sem plano"
		}
		status := a.org.SubscriptionStatus
		if status == "" {
			status = "sem assinatura"
		}
		fmt.Printf("%s  %s\n", a.org.ID, a.org.Name)
		fmt.Printf("  %s, criada em %s, plano %s, %s\n", kind, a.org.CreatedAt, plan, status)
		fmt.Printf("  %d cliente(s), %d solicitação(ões), %d arquivo(s)\n", a.clients, a.requests, a.files)
		for _, u := range a.users {
			state := "confirmado"
			if u.EmailPending {
				state = "NÃO confirmado"
			}
			fmt.Printf("  - %s  <%s>  %s  %s\n", u.ID, u.Email, u.Name, state)
		}
		fmt.Println()
	}
	return nil
}

// testAddress is the replacement. The .invalid domain is reserved by RFC 2606
// and never resolves, so a confirmation or a password reset aimed at it can
// never arrive anywhere. The random half keeps two retirements from colliding
// on the unique column.
func testAddress() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "teste-" + hex.EncodeToString(buf) + "@docping-teste.invalid", nil
}

func retireAddresses(ctx context.Context, store *persistence.Store, wanted []string, confirm bool) error {
	targets := map[string]bool{}
	for _, raw := range wanted {
		if v := strings.ToLower(strings.TrimSpace(raw)); v != "" {
			targets[v] = true
		}
	}
	if len(targets) == 0 {
		return fmt.Errorf("nothing to retire")
	}
	// The plan is computed in a read transaction and printed before anything
	// is written, so -confirm is given to a change already seen.
	type change struct{ userID, name, from, to string }
	var planned []change
	if err := store.Run(ctx, false, func(s *domain.State) error {
		for _, u := range s.Users {
			if !targets[strings.ToLower(u.Email)] && !targets[strings.ToLower(u.ID)] {
				continue
			}
			next, err := testAddress()
			if err != nil {
				return err
			}
			planned = append(planned, change{userID: u.ID, name: u.Name, from: u.Email, to: next})
		}
		return nil
	}); err != nil {
		return err
	}
	if len(planned) != len(targets) {
		found := map[string]bool{}
		for _, c := range planned {
			found[strings.ToLower(c.from)] = true
			found[strings.ToLower(c.userID)] = true
		}
		for t := range targets {
			if !found[t] {
				// Refused whole: a typo in one address must not leave half
				// the list retired and the other half untouched.
				return fmt.Errorf("no account found for %q -- nothing was changed", t)
			}
		}
	}
	for _, c := range planned {
		fmt.Printf("%s (%s): %s -> %s\n", c.userID, c.name, c.from, c.to)
	}
	if !confirm {
		fmt.Println("\nnada foi gravado. Repita com -confirm para aplicar.")
		return nil
	}
	return store.Run(ctx, true, func(s *domain.State) error {
		for _, c := range planned {
			u, ok := s.Users[c.userID]
			if !ok {
				return fmt.Errorf("user %s disappeared between reading and writing", c.userID)
			}
			previous := u.Email
			u.Email = c.to
			// A pending confirmation for the old address is now pointing at an
			// e-mail nobody will ever read; clearing it keeps the account
			// usable with the password it already has.
			u.EmailPending = false
			s.Users[u.ID] = u
			// The organization's notification address is a separate field.
			// When it was the same person's, it has to move too, or the
			// workspace keeps mailing an address that is no longer anyone's.
			if org, ok := s.Organizations[u.OrgID]; ok && strings.EqualFold(org.Email, previous) {
				org.Email = c.to
				s.Organizations[org.ID] = org
			}
			s.Audit(u.OrgID, u.ID, "account.address_retired", u.ID)
		}
		return nil
	})
}
