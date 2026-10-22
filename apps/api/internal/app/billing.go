package app

import (
	"context"
	"docping/api/internal/billing"
	"docping/api/internal/domain"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Billing lives outside the single-transaction pipeline for the same reason
// SMTP does: every route here talks to Stripe, and an HTTP call to another
// company has no business holding a database transaction open. Checkout and
// the portal follow the shape of uploadHTTP -- read transaction, external
// call, write transaction -- and the webhook adds one rule of its own: it
// verifies a signature before it believes a word of what it was sent.

// checkoutHTTP opens a Stripe Checkout for one plan and answers with its URL.
func (a *App) checkoutHTTP(w http.ResponseWriter, r *http.Request) {
	if !a.Config.Billing() {
		writeError(w, fail(503, "billing_disabled", "A assinatura não está disponível nesta instalação."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
	if err != nil {
		writeError(w, invalid("Não foi possível ler a requisição."))
		return
	}
	plan := planFromBody(body)
	price := a.Config.Prices[plan]
	if price == "" {
		writeError(w, invalid("Escolha um dos planos disponíveis."))
		return
	}
	var org domain.Organization
	var email string
	err = a.Store.Run(ctx, false, func(s *domain.State) error {
		c := &call{A: a, W: w, R: r, S: s}
		// write=true: this is a POST from the workspace, so it carries the
		// CSRF token like every other write, even though it stores nothing.
		if e := c.authenticate(true); e != nil {
			return e
		}
		org, email = c.Org, c.User.Email
		if org.Demo {
			return fail(409, "demo_organization", "A demonstração não assina planos. Crie o espaço do seu escritório para assinar.")
		}
		if org.Entitled() && org.SubscriptionID != "" {
			return fail(409, "already_subscribed", "Seu escritório já tem uma assinatura ativa. Use a área de cobrança para trocar de plano.")
		}
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	url, err := a.Stripe.Checkout(ctx, billing.CheckoutInput{
		PriceID:    price,
		CustomerID: org.StripeCustomerID,
		Email:      email,
		OrgID:      org.ID,
		SuccessURL: a.Config.Origin + "/app?assinatura=confirmada",
		CancelURL:  a.Config.Origin + "/assinar?assinatura=cancelada",
	})
	if err != nil {
		log.Printf("stripe checkout failed for organization %s: %v", org.ID, err)
		writeError(w, fail(502, "billing_unavailable", "Não foi possível abrir o pagamento agora. Tente novamente em instantes."))
		return
	}
	writeJSON(w, 200, map[string]string{"url": url})
}

// portalHTTP opens Stripe's customer portal, which is where a card is changed,
// a plan is switched and a subscription is cancelled. None of those screens
// are rebuilt here.
func (a *App) portalHTTP(w http.ResponseWriter, r *http.Request) {
	if !a.Config.Billing() {
		writeError(w, fail(503, "billing_disabled", "A assinatura não está disponível nesta instalação."))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var customer, orgID string
	err := a.Store.Run(ctx, false, func(s *domain.State) error {
		c := &call{A: a, W: w, R: r, S: s}
		if e := c.authenticate(true); e != nil {
			return e
		}
		customer, orgID = c.Org.StripeCustomerID, c.Org.ID
		if customer == "" {
			return fail(409, "no_subscription", "Seu escritório ainda não tem assinatura.")
		}
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	url, err := a.Stripe.Portal(ctx, customer, a.Config.Origin+"/app/configuracoes")
	if err != nil {
		log.Printf("stripe portal failed for organization %s: %v", orgID, err)
		writeError(w, fail(502, "billing_unavailable", "Não foi possível abrir a área de cobrança agora. Tente novamente em instantes."))
		return
	}
	writeJSON(w, 200, map[string]string{"url": url})
}

// webhookHTTP is the only route that takes Stripe's word for anything, and it
// does so only after the signature checks out. It deliberately does not use
// a.handle: there is no session and no CSRF token here, the body must be read
// raw to be verified, and the Stripe calls some events require cannot happen
// inside a transaction.
func (a *App) webhookHTTP(w http.ResponseWriter, r *http.Request) {
	if !a.Config.Billing() {
		w.WriteHeader(404)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		w.WriteHeader(400)
		return
	}
	event, err := billing.Verify(body, r.Header.Get("Stripe-Signature"), a.Config.StripeWebhookSecret, time.Now())
	if err != nil {
		// Said without detail on purpose: whoever is probing this endpoint
		// learns nothing from the answer.
		log.Print("stripe webhook rejected: signature did not verify")
		w.WriteHeader(400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	if err := a.applyEvent(ctx, event); err != nil {
		// A 500 asks Stripe to deliver again, which is what should happen
		// when the failure was ours: the database was busy, or Stripe itself
		// did not answer a follow-up read.
		log.Printf("stripe webhook %s (%s) failed: %v", event.ID, event.Type, err)
		w.WriteHeader(500)
		return
	}
	writeJSON(w, 200, map[string]string{"received": "ok"})
}

// applyEvent routes one verified event. Every unknown type is acknowledged
// and ignored: Stripe retries what it cannot deliver, and answering 500 to an
// event this product does not act on would retry it forever.
func (a *App) applyEvent(ctx context.Context, event billing.Event) error {
	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		session, err := event.CheckoutSession()
		if err != nil {
			return err
		}
		if session.Mode != "subscription" || session.Subscription == "" {
			return nil
		}
		// paid or no_payment_required; an unpaid session that completed is a
		// payment still being processed, and the async event will follow.
		if session.PaymentStatus == "unpaid" {
			return nil
		}
		// The session names the subscription but does not carry it. Reading
		// it back is what makes the stored plan and period the subscription's
		// own truth rather than an inference from the checkout.
		sub, err := a.Stripe.Subscription(ctx, session.Subscription)
		if err != nil {
			return err
		}
		if sub.CustomerID == "" {
			sub.CustomerID = session.Customer
		}
		return a.storeSubscription(ctx, event, sub, session.ClientReferenceID)
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		sub, err := event.Subscription()
		if err != nil {
			return err
		}
		if event.Type == "customer.subscription.deleted" {
			// Stripe reports a deleted subscription with whatever status it
			// ended on; what matters is that it is over.
			sub.Status = "canceled"
		}
		return a.storeSubscription(ctx, event, sub, "")
	case "invoice.paid", "invoice.payment_failed":
		invoice, err := event.Invoice()
		if err != nil {
			return err
		}
		id := invoice.SubscriptionID()
		if id == "" {
			return nil
		}
		// The invoice says a payment happened or failed; the subscription says
		// what that did to the customer's access. Only the second is stored.
		sub, err := a.Stripe.Subscription(ctx, id)
		if err != nil {
			return err
		}
		return a.storeSubscription(ctx, event, sub, "")
	case "charge.dispute.created", "charge.refunded", "radar.early_fraud_warning.created":
		return a.storeRiskAlert(ctx, event)
	}
	return nil
}

// storeSubscription writes what Stripe reported onto the organization it
// belongs to. The organization is found by Stripe customer id -- the resource
// that represents the ownership boundary -- and only falls back to the id the
// checkout carried, because metadata is something a request can influence and
// a customer id is not.
func (a *App) storeSubscription(ctx context.Context, event billing.Event, sub billing.Subscription, fallbackOrg string) error {
	plan := a.Config.PlanForPrice(sub.PriceID)
	if sub.PriceID != "" && plan == "" {
		// Money arrived for something this deployment does not sell. Guessing
		// a plan would hand out an allowance nobody bought, so the status is
		// recorded and a person is told.
		log.Printf("stripe subscription %s is on unknown price %s: plan left unchanged", sub.ID, sub.PriceID)
	}
	at := eventTime(event)
	return a.Store.Run(ctx, true, func(s *domain.State) error {
		org, ok := findOrganization(s, sub.CustomerID, fallbackOrg)
		if !ok {
			// Nothing to update and nothing to retry: a subscription for a
			// workspace this database never had is not a failure of ours.
			log.Printf("stripe subscription %s has no organization here (customer %s)", sub.ID, sub.CustomerID)
			return nil
		}
		if stale(org.BillingUpdatedAt, at) {
			return nil
		}
		previous := org.SubscriptionStatus
		org.StripeCustomerID = sub.CustomerID
		org.SubscriptionID = sub.ID
		org.SubscriptionStatus = sub.Status
		org.CancelAtPeriodEnd = sub.CancelAtPeriodEnd
		org.BillingUpdatedAt = at.UTC().Format(time.RFC3339)
		if sub.PeriodEnd > 0 {
			org.PeriodEnd = time.Unix(sub.PeriodEnd, 0).UTC().Format(time.RFC3339)
		}
		if plan != "" {
			org.Plan = plan
		}
		s.Organizations[org.ID] = org
		if previous != org.SubscriptionStatus {
			s.Audit(org.ID, "", "billing."+org.SubscriptionStatus, org.SubscriptionID)
		}
		return nil
	})
}

// storeRiskAlert records a dispute, a refund or a fraud warning against the
// workspace it concerns. It never suspends anyone: money being disputed is a
// conversation to have, not a reason to lock an accountant out of a client's
// documents in the middle of a closing.
func (a *App) storeRiskAlert(ctx context.Context, event billing.Event) error {
	object, err := event.RiskObject()
	if err != nil {
		return err
	}
	customer := object.Customer
	if customer == "" && object.Charge != "" {
		// A dispute and a fraud warning name a charge, not a customer.
		charge, err := a.Stripe.Charge(ctx, object.Charge)
		if err != nil {
			return err
		}
		customer = charge.Customer
	}
	if customer == "" {
		log.Printf("stripe %s could not be resolved to a customer", event.Type)
		return nil
	}
	at := eventTime(event)
	return a.Store.Run(ctx, true, func(s *domain.State) error {
		org, ok := findOrganization(s, customer, "")
		if !ok {
			return nil
		}
		org.BillingAlert = event.Type + " em " + at.UTC().Format(time.RFC3339)
		s.Organizations[org.ID] = org
		s.Audit(org.ID, "", "billing.alert", event.Type)
		log.Printf("billing alert for organization %s: %s", org.ID, event.Type)
		return nil
	})
}

// findOrganization resolves a Stripe customer to a workspace. The scan is over
// state already loaded for this transaction, so it adds no query; it is also
// the reason this feature stores no new column.
func findOrganization(s *domain.State, customer, fallback string) (domain.Organization, bool) {
	if customer != "" {
		for _, org := range s.Organizations {
			if org.StripeCustomerID == customer {
				return org, true
			}
		}
	}
	if fallback != "" {
		if org, ok := s.Organizations[fallback]; ok {
			return org, true
		}
	}
	return domain.Organization{}, false
}

// stale answers whether an event is older than what is already stored.
// Webhook deliveries arrive out of order and repeat, and applying an older
// one would overwrite newer truth: a cancellation delivered twice around a
// resubscription would otherwise close an account that just paid.
func stale(stored string, at time.Time) bool {
	if stored == "" {
		return false
	}
	previous, err := time.Parse(time.RFC3339, stored)
	if err != nil {
		return false
	}
	return at.Before(previous)
}

func eventTime(event billing.Event) time.Time {
	if event.Created > 0 {
		return time.Unix(event.Created, 0)
	}
	return time.Now()
}

// planFromBody reads the requested plan without binding the whole body: the
// only field this route accepts is a slug.
func planFromBody(body []byte) string {
	var in struct {
		Plan string `json:"plan"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return ""
	}
	return strings.TrimSpace(in.Plan)
}
