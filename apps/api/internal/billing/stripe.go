// Package billing talks to Stripe. It is a leaf: it knows nothing about the
// domain, holds no state and performs no persistence, like internal/analysis.
//
// The client is written here rather than taken from stripe-go. What this
// product needs from Stripe is three POSTs, one GET and one HMAC, and the
// official SDK is a large surface to carry for that in a repository that
// builds MinIO from source over supply-chain concern. The cost of the choice
// is that the API version is pinned here by hand and that signature
// verification is ours to get right -- which is why it is the most tested
// function in the package.
package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// APIVersion is sent on every request so a dashboard-side upgrade never
// changes the shape of what this code parses. Moving it is a deliberate act
// with a re-read of the fields below.
const APIVersion = "2026-09-30.endive"

// integrationIdentifier labels the Checkout sessions this product creates, so
// they can be told apart in the Stripe dashboard from any other flow. The
// random suffix is fixed: it identifies the flow, not the session.
const integrationIdentifier = "docping-assinatura-qwbrmfxd"

const endpoint = "https://api.stripe.com"

// Client is a configured Stripe caller. The zero value is not usable; use New.
type Client struct {
	key  string
	http *http.Client
}

func New(key string) *Client {
	return &Client{key: key, http: &http.Client{Timeout: 20 * time.Second}}
}

// Error is a refusal Stripe itself described. Code is Stripe's machine code
// when it sent one, which is what callers switch on; Message is for the log,
// never for the customer, because it is English and written for developers.
type Error struct {
	Status  int
	Code    string
	Type    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("stripe %d %s %s: %s", e.Status, e.Type, e.Code, e.Message)
}

func (c *Client) do(ctx context.Context, method, path string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Stripe-Version", APIVersion)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		var wrapper struct {
			Error struct {
				Type, Code, Message string
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &wrapper)
		return &Error{Status: res.StatusCode, Code: wrapper.Error.Code, Type: wrapper.Error.Type, Message: wrapper.Error.Message}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// CheckoutInput is one workspace buying one plan.
type CheckoutInput struct {
	PriceID string
	// CustomerID is empty the first time an organization subscribes; Stripe
	// then creates the customer and reports it back on the session. Passing
	// the stored one afterwards is what keeps a workspace from accumulating a
	// new Stripe customer per attempt.
	CustomerID string
	// Email seeds the Checkout form when there is no customer yet. Stripe
	// ignores it when CustomerID is set, and sending both is an error.
	Email                 string
	OrgID                 string
	SuccessURL, CancelURL string
}

// Checkout opens a hosted subscription Checkout and returns its URL.
//
// payment_method_types is deliberately absent: leaving it out is what lets
// Stripe offer every method enabled in the dashboard, and pinning it to card
// would quietly turn Pix and boleto off for a Brazilian customer.
func (c *Client) Checkout(ctx context.Context, in CheckoutInput) (string, error) {
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("line_items[0][price]", in.PriceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("success_url", in.SuccessURL)
	form.Set("cancel_url", in.CancelURL)
	form.Set("client_reference_id", in.OrgID)
	form.Set("metadata[organization_id]", in.OrgID)
	form.Set("subscription_data[metadata][organization_id]", in.OrgID)
	form.Set("integration_identifier", integrationIdentifier)
	form.Set("locale", "pt-BR")
	// A Brazilian company buys with a CNPJ on the invoice; collecting it at
	// Checkout is the only moment the customer is there to type it.
	form.Set("tax_id_collection[enabled]", "true")
	form.Set("allow_promotion_codes", "true")
	if in.CustomerID != "" {
		form.Set("customer", in.CustomerID)
		form.Set("customer_update[name]", "auto")
		form.Set("customer_update[address]", "auto")
	} else if in.Email != "" {
		form.Set("customer_email", in.Email)
	}
	var out struct {
		ID, URL string
	}
	if err := c.do(ctx, http.MethodPost, "/v1/checkout/sessions", form, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", errors.New("stripe returned a checkout session without a url")
	}
	return out.URL, nil
}

// Portal opens Stripe's own customer portal, where the subscriber changes the
// card, switches plan or cancels. Building those screens here would mean
// holding card data flows this product has no reason to touch.
func (c *Client) Portal(ctx context.Context, customerID, returnURL string) (string, error) {
	form := url.Values{}
	form.Set("customer", customerID)
	form.Set("return_url", returnURL)
	form.Set("locale", "pt-BR")
	var out struct{ URL string }
	if err := c.do(ctx, http.MethodPost, "/v1/billing_portal/sessions", form, &out); err != nil {
		return "", err
	}
	if out.URL == "" {
		return "", errors.New("stripe returned a portal session without a url")
	}
	return out.URL, nil
}

// Charge is the little of a charge this product reads: who it belongs to. A
// dispute and a fraud warning name a charge, not a customer, so resolving
// either to a workspace takes this one extra read.
type Charge struct {
	ID       string `json:"id"`
	Customer string `json:"customer"`
}

func (c *Client) Charge(ctx context.Context, id string) (Charge, error) {
	var out Charge
	err := c.do(ctx, http.MethodGet, "/v1/charges/"+url.PathEscape(id), nil, &out)
	return out, err
}

// Subscription is the subscription state this product stores.
type Subscription struct {
	ID                string
	CustomerID        string
	Status            string
	PriceID           string
	CancelAtPeriodEnd bool
	// PeriodEnd is Unix seconds, zero when Stripe sent none.
	PeriodEnd int64
}

// subscriptionPayload parses both shapes of the subscription object. Since
// 2025-03-31.basil the period moved from the subscription to each of its
// items, and this product sells a single item per subscription; the top-level
// field is read as a fallback so an older API version still produces a date.
type subscriptionPayload struct {
	ID                string `json:"id"`
	Customer          string `json:"customer"`
	Status            string `json:"status"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	CurrentPeriodEnd  int64  `json:"current_period_end"`
	Items             struct {
		Data []struct {
			CurrentPeriodEnd int64 `json:"current_period_end"`
			Price            struct {
				ID string `json:"id"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

func (p subscriptionPayload) subscription() Subscription {
	s := Subscription{ID: p.ID, CustomerID: p.Customer, Status: p.Status, CancelAtPeriodEnd: p.CancelAtPeriodEnd, PeriodEnd: p.CurrentPeriodEnd}
	if len(p.Items.Data) > 0 {
		s.PriceID = p.Items.Data[0].Price.ID
		if end := p.Items.Data[0].CurrentPeriodEnd; end > 0 {
			s.PeriodEnd = end
		}
	}
	return s
}

// Subscription reads one subscription. The webhook carries the object already,
// but a Checkout that completes reports only its id, and trusting the event's
// copy of a subscription that may have changed since is how stale state gets
// stored.
func (c *Client) Subscription(ctx context.Context, id string) (Subscription, error) {
	var out subscriptionPayload
	if err := c.do(ctx, http.MethodGet, "/v1/subscriptions/"+url.PathEscape(id), nil, &out); err != nil {
		return Subscription{}, err
	}
	return out.subscription(), nil
}

// Event is a webhook delivery, already verified.
type Event struct {
	ID      string
	Type    string
	Created int64
	data    json.RawMessage
}

// Subscription reads the event's object as a subscription.
func (e Event) Subscription() (Subscription, error) {
	var p subscriptionPayload
	if err := json.Unmarshal(e.data, &p); err != nil {
		return Subscription{}, err
	}
	return p.subscription(), nil
}

// CheckoutSession is the part of a completed Checkout this product acts on.
type CheckoutSession struct {
	ID string `json:"id"`
	// ClientReferenceID is the organization that started the Checkout. It is
	// used only to find the workspace; what grants access is the subscription
	// Stripe reports, never a value the browser could have influenced.
	ClientReferenceID string `json:"client_reference_id"`
	Customer          string `json:"customer"`
	Subscription      string `json:"subscription"`
	Mode              string `json:"mode"`
	PaymentStatus     string `json:"payment_status"`
	Status            string `json:"status"`
}

func (e Event) CheckoutSession() (CheckoutSession, error) {
	var s CheckoutSession
	err := json.Unmarshal(e.data, &s)
	return s, err
}

// Invoice is a paid or failed renewal. Since 2025-03-31.basil the owning
// subscription moved under parent.subscription_details; the flat field is
// read as a fallback for an older delivery.
type Invoice struct {
	ID       string `json:"id"`
	Customer string `json:"customer"`
	Status   string `json:"status"`
	Flat     string `json:"subscription"`
	Parent   struct {
		SubscriptionDetails struct {
			Subscription string `json:"subscription"`
		} `json:"subscription_details"`
	} `json:"parent"`
}

// SubscriptionID is the subscription this invoice belongs to, from whichever
// shape the delivery used. It is empty for a one-off invoice.
func (i Invoice) SubscriptionID() string {
	if id := i.Parent.SubscriptionDetails.Subscription; id != "" {
		return id
	}
	return i.Flat
}

func (e Event) Invoice() (Invoice, error) {
	var i Invoice
	err := json.Unmarshal(e.data, &i)
	return i, err
}

// RiskObject covers the three events that put money at risk: a dispute, a
// refunded charge and an early fraud warning. Each names a charge; only the
// charge itself names the customer.
type RiskObject struct {
	ID       string `json:"id"`
	Customer string `json:"customer"`
	Charge   string `json:"charge"`
	Reason   string `json:"reason"`
}

func (e Event) RiskObject() (RiskObject, error) {
	var r RiskObject
	err := json.Unmarshal(e.data, &r)
	return r, err
}

// tolerance is how far a delivery's timestamp may sit from now. Stripe signs
// the timestamp together with the body, so this is what stops a captured
// delivery from being replayed later.
const tolerance = 5 * time.Minute

// ErrSignature is every refusal of a webhook delivery. It is deliberately one
// error with no detail: the endpoint answers 400 and says nothing, because an
// attacker probing it learns nothing from a specific complaint.
var ErrSignature = errors.New("invalid stripe signature")

// Verify checks the Stripe-Signature header against the payload and returns
// the parsed event. Everything about this function is the security boundary of
// the webhook: an event that fails here never reaches the database.
func Verify(payload []byte, header, secret string, now time.Time) (Event, error) {
	if secret == "" {
		return Event{}, ErrSignature
	}
	var timestamp string
	signatures := []string{}
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return Event{}, ErrSignature
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return Event{}, ErrSignature
	}
	if drift := now.Sub(time.Unix(seconds, 0)); drift > tolerance || drift < -tolerance {
		return Event{}, ErrSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := []byte(hex.EncodeToString(mac.Sum(nil)))
	ok := false
	for _, signature := range signatures {
		// Every candidate is compared, and compared in constant time: an
		// early return on the first match would leak which one matched, and
		// bytes.Equal would leak where the comparison stopped.
		if hmac.Equal([]byte(signature), expected) {
			ok = true
		}
	}
	if !ok {
		return Event{}, ErrSignature
	}
	var envelope struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Created int64  `json:"created"`
		Data    struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return Event{}, ErrSignature
	}
	if envelope.ID == "" || envelope.Type == "" {
		return Event{}, ErrSignature
	}
	return Event{ID: envelope.ID, Type: envelope.Type, Created: envelope.Created, data: envelope.Data.Object}, nil
}
