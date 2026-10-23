package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"docping/api/internal/billing"
	"docping/api/internal/domain"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testWebhookSecret = "whsec_test_app"
	priceEssencial    = "price_essencial"
	priceEscritorio   = "price_escritorio"
)

// billingApp is the harness with money turned on. Everything else -- the JSON
// store, the development origin -- matches setup, so a difference in a test
// below is a difference billing made.
func billingApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	app, _ := setup(t)
	app.Config.StripeKey = "sk_live_not_called_in_tests"
	app.Config.StripeWebhookSecret = testWebhookSecret
	app.Config.Prices = map[string]string{
		domain.PlanEssencial:  priceEssencial,
		domain.PlanEscritorio: priceEscritorio,
		domain.PlanRede:       "price_rede",
	}
	return app, app.Handler()
}

func deliver(t *testing.T, h http.Handler, payload string) *httptest.ResponseRecorder {
	t.Helper()
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write([]byte(stamp + "." + payload))
	r := httptest.NewRequest("POST", "/api/billing/webhook", strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Stripe-Signature", "t="+stamp+",v1="+hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func subscriptionEvent(id, kind, customer, status, price string, created, periodEnd int64) string {
	return fmt.Sprintf(`{"id":%q,"type":%q,"created":%d,"data":{"object":{"id":"sub_1","customer":%q,"status":%q,"items":{"data":[{"current_period_end":%d,"price":{"id":%q}}]}}}}`,
		id, kind, created, customer, status, periodEnd, price)
}

// linkCustomer puts the organization in the state a completed Checkout leaves
// it in, without calling Stripe: the customer is known, the subscription is
// not yet active.
func linkCustomer(t *testing.T, app *App, orgID, customer string) {
	t.Helper()
	if err := app.Store.Run(t.Context(), true, func(s *domain.State) error {
		org := s.Organizations[orgID]
		org.StripeCustomerID = customer
		s.Organizations[orgID] = org
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func orgOf(t *testing.T, app *App, id string) domain.Organization {
	t.Helper()
	var out domain.Organization
	if err := app.Store.Run(t.Context(), false, func(s *domain.State) error {
		out = s.Organizations[id]
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

// A workspace that never subscribed can sign in, read its own record and
// reach checkout -- and nothing else. That is the whole gate.
func TestWithoutASubscriptionOnlyTheSubscribeScreenWorks(t *testing.T) {
	_, h := billingApp(t)
	session := registerTest(t, h, "sem-plano@exemplo.com.br")
	if status, body, _ := req(t, h, "GET", "/api/session", nil, session); status != 200 {
		t.Fatalf("session should stay readable: %d %s", status, body)
	}
	if status, body, _ := req(t, h, "GET", "/api/organization", nil, session); status != 200 {
		t.Fatalf("organization should stay readable: %d %s", status, body)
	}
	for _, route := range []struct {
		method, path string
		body         any
	}{
		{"GET", "/api/clients", nil},
		{"GET", "/api/requests", nil},
		{"POST", "/api/clients", map[string]string{"name": "Padaria Aurora", "cnpj": "11222333000181"}},
		{"PUT", "/api/organization", map[string]any{"name": "Outro nome", "retentionDays": 365}},
	} {
		status, body, _ := req(t, h, route.method, route.path, route.body, session)
		if status != 402 {
			t.Fatalf("%s %s should require a subscription: %d %s", route.method, route.path, status, body)
		}
		if !strings.Contains(string(body), "subscription_required") {
			t.Fatalf("%s %s: unexpected error %s", route.method, route.path, body)
		}
	}
}

// The same workspace, once Stripe reports an active subscription, works
// normally -- and carries the plan that was paid for.
func TestAnActiveSubscriptionOpensTheWorkspace(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "assina@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_abc")
	end := time.Now().Add(30 * 24 * time.Hour).Unix()
	if w := deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_abc", "active", priceEscritorio, time.Now().Unix(), end)); w.Code != 200 {
		t.Fatalf("webhook: %d %s", w.Code, w.Body)
	}
	org := orgOf(t, app, session.Org)
	if org.Plan != domain.PlanEscritorio {
		t.Fatalf("plan should follow the price that was paid: %q", org.Plan)
	}
	if !org.Entitled() {
		t.Fatalf("an active subscription must entitle: %+v", org)
	}
	status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": "Padaria Aurora", "cnpj": "11222333000181"}, session)
	if status != 200 {
		t.Fatalf("subscribed workspace refused: %d %s", status, body)
	}
}

// A cancellation closes the workspace again. The documents stay; what stops
// is the ability to create more work.
func TestACancelledSubscriptionClosesTheWorkspaceAgain(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "cancela@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_cancel")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_cancel", "active", priceEssencial, now, now+100))
	if status, _, _ := req(t, h, "GET", "/api/clients", nil, session); status != 200 {
		t.Fatal("active subscription should read clients")
	}
	deliver(t, h, subscriptionEvent("evt_2", "customer.subscription.deleted", "cus_cancel", "canceled", priceEssencial, now+10, now+100))
	if status, body, _ := req(t, h, "GET", "/api/clients", nil, session); status != 402 {
		t.Fatalf("cancelled subscription should close the workspace: %d %s", status, body)
	}
	if orgOf(t, app, session.Org).SubscriptionStatus != "canceled" {
		t.Fatal("status not stored")
	}
}

// A failed payment is not a cancellation: Stripe is still retrying the card,
// and the workspace keeps working while it does.
func TestPastDueKeepsTheWorkspaceOpen(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "atrasada@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_late")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.updated", "cus_late", "past_due", priceEssencial, now, now+100))
	if status, body, _ := req(t, h, "GET", "/api/clients", nil, session); status != 200 {
		t.Fatalf("past_due should keep working: %d %s", status, body)
	}
}

// Deliveries repeat and arrive out of order. An older event must never
// overwrite newer truth -- a stale cancellation after a fresh renewal would
// close a workspace that just paid.
func TestAnOlderDeliveryDoesNotOverwriteNewerTruth(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "fora-de-ordem@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_order")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_new", "customer.subscription.updated", "cus_order", "active", priceeRede(), now, now+100))
	deliver(t, h, subscriptionEvent("evt_old", "customer.subscription.deleted", "cus_order", "canceled", priceEssencial, now-600, now+100))
	org := orgOf(t, app, session.Org)
	if org.SubscriptionStatus != "active" {
		t.Fatalf("stale delivery was applied: %+v", org)
	}
	if org.Plan != domain.PlanRede {
		t.Fatalf("stale delivery changed the plan: %+v", org)
	}
	// The same delivery twice must also be harmless.
	deliver(t, h, subscriptionEvent("evt_new", "customer.subscription.updated", "cus_order", "active", priceeRede(), now, now+100))
	if orgOf(t, app, session.Org).SubscriptionStatus != "active" {
		t.Fatal("replay changed the state")
	}
}

func priceeRede() string { return "price_rede" }

// A delivery that does not verify changes nothing. This is the webhook's
// whole security boundary: without it, anyone who can reach the endpoint
// subscribes for free.
func TestAnUnsignedDeliveryChangesNothing(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "forjada@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_forge")
	now := time.Now().Unix()
	payload := subscriptionEvent("evt_forged", "customer.subscription.created", "cus_forge", "active", priceEscritorio, now, now+100)
	r := httptest.NewRequest("POST", "/api/billing/webhook", strings.NewReader(payload))
	r.Header.Set("Stripe-Signature", "t="+strconv.FormatInt(now, 10)+",v1="+strings.Repeat("a", 64))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("forged delivery accepted: %d", w.Code)
	}
	if orgOf(t, app, session.Org).Entitled() {
		t.Fatal("a forged delivery granted access")
	}
	if status, _, _ := req(t, h, "GET", "/api/clients", nil, session); status != 402 {
		t.Fatal("workspace opened on a forged delivery")
	}
}

// Money for a price this deployment does not sell must not pick a plan. The
// subscription is recorded; the allowance is not invented.
func TestAnUnknownPriceDoesNotChooseAPlan(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "preco-estranho@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_unknown")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_unknown", "active", "price_de_outra_conta", now, now+100))
	org := orgOf(t, app, session.Org)
	if org.Plan != "" {
		t.Fatalf("an unknown price chose a plan: %q", org.Plan)
	}
	if org.SubscriptionStatus != "active" {
		t.Fatalf("status should still be recorded: %+v", org)
	}
}

// The plan's allowance is the server's, not the pricing page's. Essencial
// sells fifteen active requests, so the sixteenth is refused -- and a request
// that is finished frees its slot.
func TestThePlanLimitsActiveRequests(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "limite@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_limit")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_limit", "active", priceEssencial, now, now+100))

	status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": "Padaria Aurora", "cnpj": "11222333000181"}, session)
	if status != 200 {
		t.Fatalf("client: %d %s", status, body)
	}
	var client struct{ ID string }
	json.Unmarshal(body, &client)
	status, body, _ = req(t, h, "GET", "/api/templates", nil, session)
	if status != 200 {
		t.Fatalf("templates: %d %s", status, body)
	}
	var templates []struct{ ID string }
	json.Unmarshal(body, &templates)
	if len(templates) == 0 {
		t.Fatal("the workspace should be seeded with templates")
	}
	create := func(competence string) (int, []byte) {
		status, body, _ := req(t, h, "POST", "/api/requests", map[string]any{
			"clientIds": []string{client.ID}, "templateId": templates[0].ID,
			"title": "Fechamento " + competence, "competence": competence, "dueDate": "2027-01-20",
		}, session)
		return status, body
	}
	limit := domain.PlanEntitlement(domain.PlanEssencial).ActiveRequests
	for n := 0; n < limit; n++ {
		if status, body := create(fmt.Sprintf("2026-%02d", (n%12)+1)); status != 200 {
			t.Fatalf("request %d refused early: %d %s", n+1, status, body)
		}
	}
	status, body = create("2027-01")
	if status != 422 || !strings.Contains(string(body), "plan_limit") {
		t.Fatalf("the allowance was not enforced: %d %s", status, body)
	}
	// Cancelling one frees its slot: the limit counts work in progress, not
	// work ever done.
	status, body, _ = req(t, h, "GET", "/api/requests", nil, session)
	var requests []struct{ ID string }
	json.Unmarshal(body, &requests)
	if len(requests) == 0 {
		t.Fatalf("requests: %d %s", status, body)
	}
	if status, body, _ := req(t, h, "POST", "/api/requests/"+requests[0].ID+"/cancel", map[string]string{}, session); status != 200 {
		t.Fatalf("cancel: %d %s", status, body)
	}
	if status, body := create("2027-02"); status != 200 {
		t.Fatalf("a freed slot was not reused: %d %s", status, body)
	}
}

// A batch that would cross the line is refused whole. Half a month's requests
// is worse than none: nothing would say which clients were left out.
//
// Reaching the ceiling takes several competences open at once, because a
// batch opens one request per client and the ceiling is a multiple of the
// clients the plan sells. That is the point of deriving it: no single
// closing can hit it, and an office only meets it by leaving months open.
func TestABatchOverTheLimitIsRefusedWhole(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "lote@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_batch")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_batch", "active", priceEssencial, now, now+100))
	ids := []string{}
	for n := 0; n < 20; n++ {
		status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": fmt.Sprintf("Cliente %02d", n), "cnpj": fmt.Sprintf("112223330001%02d", n)}, session)
		if status != 200 {
			t.Fatalf("client %d: %d %s", n, status, body)
		}
		var c struct{ ID string }
		json.Unmarshal(body, &c)
		ids = append(ids, c.ID)
	}
	_, body, _ := req(t, h, "GET", "/api/templates", nil, session)
	var templates []struct{ ID string }
	json.Unmarshal(body, &templates)
	batch := func(competence string) (int, []byte) {
		status, body, _ := req(t, h, "POST", "/api/requests", map[string]any{
			"clientIds": ids, "templateId": templates[0].ID,
			"title": "Fechamento mensal", "competence": competence, "dueDate": "2026-11-20",
		}, session)
		return status, body
	}
	// Six competences fill the ceiling exactly: twenty clients, six each.
	for _, competence := range []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06"} {
		if status, body := batch(competence); status != 200 {
			t.Fatalf("competence %s should fit: %d %s", competence, status, body)
		}
	}
	status, body := batch("2026-07")
	if status != 422 || !strings.Contains(string(body), "plan_limit") {
		t.Fatalf("batch over the allowance accepted: %d %s", status, body)
	}
	if status, body, _ := req(t, h, "GET", "/api/requests", nil, session); status != 200 || string(body) == "" {
		t.Fatalf("requests: %d %s", status, body)
	} else {
		var requests []struct{ ID string }
		json.Unmarshal(body, &requests)
		if len(requests) != 120 {
			t.Fatalf("a refused batch created %d requests", len(requests))
		}
	}
}

// With no Stripe key the product is what it was before money existed: a new
// workspace is a founder pilot and nothing is gated.
func TestWithoutStripeNothingIsGated(t *testing.T) {
	_, h := setup(t)
	session := registerTest(t, h, "sem-stripe@exemplo.com.br")
	if status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": "Padaria Aurora", "cnpj": "11222333000181"}, session); status != 200 {
		t.Fatalf("an unconfigured deployment must not gate: %d %s", status, body)
	}
	status, body, _ := req(t, h, "GET", "/api/session", nil, session)
	if status != 200 {
		t.Fatalf("session: %d %s", status, body)
	}
	var out struct {
		Billing struct {
			Enabled, Entitled bool
		}
		Organization struct{ Plan string }
	}
	json.Unmarshal(body, &out)
	if out.Billing.Enabled || !out.Billing.Entitled {
		t.Fatalf("billing should read as off and open: %+v", out.Billing)
	}
	if out.Organization.Plan != domain.PlanFounderPilot {
		t.Fatalf("plan should be the founder pilot: %q", out.Organization.Plan)
	}
}

// A workspace opened before billing existed keeps working after it is turned
// on. This is the promise on the pricing page, and it is the one case where
// an organization with no subscription may still write.
func TestTheFounderPilotIsNeverLockedOut(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "fundador@exemplo.com.br")
	if err := app.Store.Run(t.Context(), true, func(s *domain.State) error {
		org := s.Organizations[session.Org]
		org.Plan = domain.PlanFounderPilot
		s.Organizations[session.Org] = org
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{"name": "Padaria Aurora", "cnpj": "11222333000181"}, session); status != 200 {
		t.Fatalf("the founder pilot was locked out: %d %s", status, body)
	}
}

// A dispute is recorded for a person to look at. It does not suspend the
// workspace: documents mid-closing are not leverage in a payment argument.
func TestADisputeRecordsAnAlertWithoutClosingTheWorkspace(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "disputa@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_dispute")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_dispute", "active", priceEssencial, now, now+100))
	payload := fmt.Sprintf(`{"id":"evt_2","type":"charge.dispute.created","created":%d,"data":{"object":{"id":"dp_1","customer":"cus_dispute","charge":"ch_1","reason":"fraudulent"}}}`, now+5)
	if w := deliver(t, h, payload); w.Code != 200 {
		t.Fatalf("dispute delivery: %d %s", w.Code, w.Body)
	}
	org := orgOf(t, app, session.Org)
	if !strings.Contains(org.BillingAlert, "charge.dispute.created") {
		t.Fatalf("alert not recorded: %q", org.BillingAlert)
	}
	if !org.Entitled() {
		t.Fatal("a dispute closed the workspace")
	}
}

// An event for a customer this database never saw is acknowledged, not
// retried: it is not a failure of ours, and a 500 would make Stripe redeliver
// it for days.
func TestAnEventForAnotherSystemIsAcknowledged(t *testing.T) {
	_, h := billingApp(t)
	now := time.Now().Unix()
	w := deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_de_outro_sistema", "active", priceEssencial, now, now+100))
	if w.Code != 200 {
		t.Fatalf("unknown customer should be acknowledged: %d %s", w.Code, w.Body)
	}
}

// The event types this product does not act on are acknowledged too.
func TestAnIgnoredEventTypeIsAcknowledged(t *testing.T) {
	_, h := billingApp(t)
	payload := fmt.Sprintf(`{"id":"evt_x","type":"customer.updated","created":%d,"data":{"object":{"id":"cus_1"}}}`, time.Now().Unix())
	if w := deliver(t, h, payload); w.Code != 200 {
		t.Fatalf("ignored type should be acknowledged: %d %s", w.Code, w.Body)
	}
}

// The webhook needs no session and no CSRF token, so the rest of the request
// pipeline must not demand one -- a 403 here would be invisible until money
// started moving.
func TestTheWebhookNeedsNoSessionOrCSRF(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "sem-sessao@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_nosession")
	now := time.Now().Unix()
	w := deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_nosession", "active", priceEssencial, now, now+100))
	if w.Code != 200 {
		t.Fatalf("webhook refused without a session: %d %s", w.Code, w.Body)
	}
	if !orgOf(t, app, session.Org).Entitled() {
		t.Fatal("the delivery did not take effect")
	}
}

// Checkout refuses what it cannot sell before it ever reaches Stripe.
func TestCheckoutRefusesAnUnknownPlan(t *testing.T) {
	_, h := billingApp(t)
	session := registerTest(t, h, "plano-invalido@exemplo.com.br")
	for _, plan := range []string{"", "premium", domain.PlanDemo, domain.PlanFounderPilot} {
		status, body, _ := req(t, h, "POST", "/api/billing/checkout", map[string]string{"plan": plan}, session)
		if status != 422 {
			t.Fatalf("plan %q accepted: %d %s", plan, status, body)
		}
	}
}

// The demonstration is not a customer: it has no e-mail, no owner and expires
// by itself. Selling it a subscription would attach money to a workspace that
// is deleted on schedule.
func TestTheDemonstrationCannotSubscribe(t *testing.T) {
	_, h := billingApp(t)
	status, body, w := req(t, h, "POST", "/api/auth/demo", map[string]string{}, testSession{})
	if status != 200 {
		t.Fatalf("demo: %d %s", status, body)
	}
	var out struct{ CSRFToken string }
	json.Unmarshal(body, &out)
	demo := testSession{CSRF: out.CSRFToken}
	for _, c := range w.Result().Cookies() {
		if c.Name == "docping_session" {
			demo.Cookie = c
		}
	}
	status, body, _ = req(t, h, "POST", "/api/billing/checkout", map[string]string{"plan": domain.PlanEssencial}, demo)
	if status != 409 || !strings.Contains(string(body), "demo_organization") {
		t.Fatalf("the demonstration was offered a subscription: %d %s", status, body)
	}
}

// A demonstration keeps working with billing on: it never subscribed and
// never will, and gating it would empty the one screen a prospect sees.
func TestTheDemonstrationIsNotGated(t *testing.T) {
	_, h := billingApp(t)
	status, body, w := req(t, h, "POST", "/api/auth/demo", map[string]string{}, testSession{})
	if status != 200 {
		t.Fatalf("demo: %d %s", status, body)
	}
	var out struct{ CSRFToken string }
	json.Unmarshal(body, &out)
	demo := testSession{CSRF: out.CSRFToken}
	for _, c := range w.Result().Cookies() {
		if c.Name == "docping_session" {
			demo.Cookie = c
		}
	}
	if status, body, _ := req(t, h, "GET", "/api/requests", nil, demo); status != 200 {
		t.Fatalf("the demonstration was gated: %d %s", status, body)
	}
}

// Without a Stripe key the three routes answer that they do not exist rather
// than failing on a nil client.
func TestBillingRoutesAreInertWithoutAKey(t *testing.T) {
	_, h := setup(t)
	session := registerTest(t, h, "inerte@exemplo.com.br")
	status, _, _ := req(t, h, "POST", "/api/billing/checkout", map[string]string{"plan": domain.PlanEssencial}, session)
	if status != 503 {
		t.Fatalf("checkout should be unavailable: %d", status)
	}
	w := deliver(t, h, `{"id":"evt_1","type":"customer.subscription.created","created":1,"data":{"object":{}}}`)
	if w.Code != 404 {
		t.Fatalf("webhook should not exist: %d", w.Code)
	}
}

// The client figure the server enforces must be the one the pricing page
// sells -- it is the only number on the cards, and the only one an office
// counts before buying. apps/web/src/lib/plans.ts prints these three.
func TestTheSoldClientAllowanceIsTheEnforcedOne(t *testing.T) {
	for plan, clients := range map[string]int{
		domain.PlanEssencial:  20,
		domain.PlanEscritorio: 60,
		domain.PlanRede:       150,
	} {
		if got := domain.PlanEntitlement(plan).Clients; got != clients {
			t.Fatalf("%s: enforcing %d clients, selling %d", plan, got, clients)
		}
	}
	// An unknown slug must cost the customer nothing and grant the least.
	if domain.PlanEntitlement("inventado").Clients != 20 {
		t.Fatal("an unknown plan should fall back to the smallest paid one")
	}
}

// The two figures that are not sold are ceilings, and a ceiling has to leave
// room: an office that reaches one has met a limit nobody mentioned to it.
// This does not restate the formula -- it states the floor below which the
// formula would be a hidden boundary rather than a safety net.
func TestTheUnsoldCeilingsStayFarAboveTheSoldFigure(t *testing.T) {
	for _, plan := range []string{domain.PlanEssencial, domain.PlanEscritorio, domain.PlanRede} {
		e := domain.PlanEntitlement(plan)
		// Several competences open per client at once, never just one.
		if e.ActiveRequests < e.Clients*4 {
			t.Fatalf("%s: %d requests for %d clients is within reach of normal use", plan, e.ActiveRequests, e.Clients)
		}
		// Years of documents per client, not months.
		if e.StorageBytes < int64(e.Clients)*(1<<30) {
			t.Fatalf("%s: %d bytes for %d clients is within reach of normal use", plan, e.StorageBytes, e.Clients)
		}
	}
	// The workspaces that never bought a plan keep the deployment ceiling.
	for _, plan := range []string{domain.PlanFounderPilot, domain.PlanDemo} {
		if domain.PlanEntitlement(plan).Clients != 0 {
			t.Fatalf("%s should carry no plan figure of its own", plan)
		}
	}
}

// The configuration refuses to charge by halves.
func TestBillingConfigurationRefusesHalfSetups(t *testing.T) {
	base := func() Config {
		return Config{Env: "production", StripeKey: "sk_live_x", StripeWebhookSecret: "whsec_x", Prices: map[string]string{
			domain.PlanEssencial: "price_a", domain.PlanEscritorio: "price_b", domain.PlanRede: "price_c",
		}}
	}
	if err := base().checkBilling(); err != nil {
		t.Fatalf("a complete configuration was refused: %v", err)
	}
	cases := map[string]func(*Config){
		"no webhook secret":   func(c *Config) { c.StripeWebhookSecret = "" },
		"missing price":       func(c *Config) { delete(c.Prices, domain.PlanRede) },
		"one price two plans": func(c *Config) { c.Prices[domain.PlanRede] = "price_a" },
		"test key in production": func(c *Config) {
			c.StripeKey = "sk_test_x"
		},
		"not a live key": func(c *Config) { c.StripeKey = "pk_live_x" },
	}
	for name, mutate := range cases {
		cfg := base()
		mutate(&cfg)
		if err := cfg.checkBilling(); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	// Prices without a key is a half-finished configuration, not a silent
	// no-op.
	if err := (Config{Env: "development", Prices: map[string]string{domain.PlanEssencial: "price_a"}}).checkBilling(); err == nil {
		t.Fatal("prices without a key were accepted")
	}
	// And nothing at all is a deployment that simply does not charge.
	if err := (Config{Env: "development"}).checkBilling(); err != nil {
		t.Fatalf("an unconfigured deployment was refused: %v", err)
	}
	// A test key is how a sandbox is exercised outside production.
	sandbox := base()
	sandbox.Env, sandbox.StripeKey = "development", "sk_test_x"
	if err := sandbox.checkBilling(); err != nil {
		t.Fatalf("a sandbox key outside production was refused: %v", err)
	}
}

var _ = billing.APIVersion

// The client count is what the plan sells, so it is what the server counts.
// The twenty-first client on Essencial is refused, and the refusal names the
// plan rather than a deployment setting the office cannot see.
func TestTheClientAllowanceFollowsThePlan(t *testing.T) {
	app, h := billingApp(t)
	session := registerTest(t, h, "carteira@exemplo.com.br")
	linkCustomer(t, app, session.Org, "cus_clients")
	now := time.Now().Unix()
	deliver(t, h, subscriptionEvent("evt_1", "customer.subscription.created", "cus_clients", "active", priceEssencial, now, now+100))
	for n := 0; n < 20; n++ {
		status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{
			"name": fmt.Sprintf("Cliente %02d", n), "cnpj": fmt.Sprintf("112223330001%02d", n),
		}, session)
		if status != 200 {
			t.Fatalf("client %d should fit in Essencial: %d %s", n, status, body)
		}
	}
	status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{
		"name": "Cliente 20", "cnpj": "11222333000120",
	}, session)
	if status != 422 || !strings.Contains(string(body), "client_limit") {
		t.Fatalf("the twenty-first client was accepted: %d %s", status, body)
	}
	// Moving up the ladder makes room without touching stored data: the limit
	// is read from the plan, never copied into the organization.
	deliver(t, h, subscriptionEvent("evt_2", "customer.subscription.updated", "cus_clients", "active", priceEscritorio, now+1, now+100))
	if status, body, _ := req(t, h, "POST", "/api/clients", map[string]string{
		"name": "Cliente 20", "cnpj": "11222333000120",
	}, session); status != 200 {
		t.Fatalf("Escritório should hold more clients: %d %s", status, body)
	}
}

// A workspace that never bought a plan keeps the deployment's own limit, as
// it did before billing existed.
func TestThePilotKeepsTheDeploymentClientLimit(t *testing.T) {
	org := domain.Organization{Plan: domain.PlanFounderPilot, ClientLimit: 7}
	if got := domain.EffectiveClientLimit(org); got != 7 {
		t.Fatalf("the pilot should keep its stored limit, got %d", got)
	}
	org.Plan = domain.PlanRede
	if got := domain.EffectiveClientLimit(org); got != 150 {
		t.Fatalf("a paid plan should decide the limit, got %d", got)
	}
}
