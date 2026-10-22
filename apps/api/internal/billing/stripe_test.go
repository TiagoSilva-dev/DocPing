package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"
)

const secret = "whsec_test_wQJc2nLkPd8r"

func sign(t *testing.T, payload string, at time.Time, key string) string {
	t.Helper()
	stamp := strconv.FormatInt(at.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(stamp + "." + payload))
	return "t=" + stamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

const samplePayload = `{"id":"evt_1","type":"customer.subscription.updated","created":1790000000,"data":{"object":{"id":"sub_1","customer":"cus_1","status":"active","items":{"data":[{"current_period_end":1792000000,"price":{"id":"price_escritorio"}}]}}}}`

func TestVerifyAcceptsAGenuineDelivery(t *testing.T) {
	now := time.Now()
	event, err := Verify([]byte(samplePayload), sign(t, samplePayload, now, secret), secret, now)
	if err != nil {
		t.Fatalf("genuine delivery rejected: %v", err)
	}
	if event.ID != "evt_1" || event.Type != "customer.subscription.updated" {
		t.Fatalf("envelope not parsed: %+v", event)
	}
	sub, err := event.Subscription()
	if err != nil {
		t.Fatalf("subscription: %v", err)
	}
	if sub.ID != "sub_1" || sub.CustomerID != "cus_1" || sub.Status != "active" {
		t.Fatalf("subscription not parsed: %+v", sub)
	}
	if sub.PriceID != "price_escritorio" {
		t.Fatalf("price should come from the item: %+v", sub)
	}
	if sub.PeriodEnd != 1792000000 {
		t.Fatalf("period should come from the item: %+v", sub)
	}
}

// The body is what the signature covers. A delivery whose body was edited in
// flight -- the status upgraded, the customer swapped -- must not parse.
func TestVerifyRejectsATamperedBody(t *testing.T) {
	now := time.Now()
	header := sign(t, samplePayload, now, secret)
	tampered := strings.Replace(samplePayload, `"status":"active"`, `"status":"trialing"`, 1)
	if _, err := Verify([]byte(tampered), header, secret, now); err != ErrSignature {
		t.Fatalf("tampered body accepted: %v", err)
	}
}

func TestVerifyRejectsAnotherSecret(t *testing.T) {
	now := time.Now()
	header := sign(t, samplePayload, now, "whsec_someone_elses_endpoint")
	if _, err := Verify([]byte(samplePayload), header, secret, now); err != ErrSignature {
		t.Fatalf("foreign secret accepted: %v", err)
	}
}

// Stripe signs the timestamp together with the body, so a delivery captured
// once cannot be replayed tomorrow. Both directions are refused: a stale
// delivery and one dated in the future.
func TestVerifyRejectsAReplayOutsideTolerance(t *testing.T) {
	now := time.Now()
	old := now.Add(-10 * time.Minute)
	if _, err := Verify([]byte(samplePayload), sign(t, samplePayload, old, secret), secret, now); err != ErrSignature {
		t.Fatalf("stale delivery accepted: %v", err)
	}
	ahead := now.Add(10 * time.Minute)
	if _, err := Verify([]byte(samplePayload), sign(t, samplePayload, ahead, secret), secret, now); err != ErrSignature {
		t.Fatalf("future delivery accepted: %v", err)
	}
	recent := now.Add(-4 * time.Minute)
	if _, err := Verify([]byte(samplePayload), sign(t, samplePayload, recent, secret), secret, now); err != nil {
		t.Fatalf("delivery inside tolerance rejected: %v", err)
	}
}

// An endpoint with no secret configured must refuse everything rather than
// accept anything: the empty secret is a missing configuration, not a
// permission to skip the check.
func TestVerifyRefusesWithoutASecret(t *testing.T) {
	now := time.Now()
	if _, err := Verify([]byte(samplePayload), sign(t, samplePayload, now, ""), "", now); err != ErrSignature {
		t.Fatalf("empty secret accepted: %v", err)
	}
}

func TestVerifyRejectsMalformedHeaders(t *testing.T) {
	now := time.Now()
	good := sign(t, samplePayload, now, secret)
	stamp := strconv.FormatInt(now.Unix(), 10)
	for name, header := range map[string]string{
		"empty":          "",
		"no signature":   "t=" + stamp,
		"no timestamp":   strings.SplitN(good, ",", 2)[1],
		"unparsed stamp": "t=ontem,v1=" + strings.TrimPrefix(strings.SplitN(good, ",", 2)[1], "v1="),
		"not hex":        "t=" + stamp + ",v1=zzzz",
		"scheme only":    "v0=" + strings.TrimPrefix(strings.SplitN(good, ",", 2)[1], "v1="),
		"garbage":        "assinatura valida, prometo",
	} {
		if _, err := Verify([]byte(samplePayload), header, secret, now); err != ErrSignature {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
}

// Stripe rotates an endpoint secret by signing with both for a while, sending
// several v1 signatures in one header. The valid one must be found wherever
// it sits.
func TestVerifyAcceptsOneOfSeveralSignatures(t *testing.T) {
	now := time.Now()
	valid := strings.TrimPrefix(strings.SplitN(sign(t, samplePayload, now, secret), ",", 2)[1], "v1=")
	stamp := strconv.FormatInt(now.Unix(), 10)
	header := "t=" + stamp + ",v1=" + strings.Repeat("0", 64) + ",v1=" + valid
	if _, err := Verify([]byte(samplePayload), header, secret, now); err != nil {
		t.Fatalf("rotation header rejected: %v", err)
	}
}

// A body that is signed correctly but is not an event must not produce an
// event with an empty type, which would then match nothing and be silently
// acknowledged.
func TestVerifyRejectsAnEnvelopeWithoutIdentity(t *testing.T) {
	now := time.Now()
	for _, payload := range []string{`{"type":"customer.subscription.updated"}`, `{"id":"evt_1"}`, `nao e json`} {
		if _, err := Verify([]byte(payload), sign(t, payload, now, secret), secret, now); err != ErrSignature {
			t.Fatalf("%q accepted", payload)
		}
	}
}

// Both shapes of the invoice object must resolve to the same subscription:
// the current one nests it under parent, older deliveries carry it flat.
func TestInvoiceSubscriptionFromBothShapes(t *testing.T) {
	now := time.Now()
	nested := `{"id":"evt_2","type":"invoice.paid","created":1790000000,"data":{"object":{"id":"in_1","customer":"cus_1","status":"paid","parent":{"subscription_details":{"subscription":"sub_9"}}}}}`
	event, err := Verify([]byte(nested), sign(t, nested, now, secret), secret, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	invoice, err := event.Invoice()
	if err != nil {
		t.Fatalf("invoice: %v", err)
	}
	if invoice.SubscriptionID() != "sub_9" {
		t.Fatalf("nested subscription not read: %+v", invoice)
	}
	flat := `{"id":"evt_3","type":"invoice.paid","created":1790000000,"data":{"object":{"id":"in_2","customer":"cus_1","status":"paid","subscription":"sub_8"}}}`
	event, err = Verify([]byte(flat), sign(t, flat, now, secret), secret, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	invoice, _ = event.Invoice()
	if invoice.SubscriptionID() != "sub_8" {
		t.Fatalf("flat subscription not read: %+v", invoice)
	}
}

// Without items the top-level period is the only date there is.
func TestSubscriptionFallsBackToTheTopLevelPeriod(t *testing.T) {
	now := time.Now()
	payload := `{"id":"evt_4","type":"customer.subscription.deleted","created":1790000000,"data":{"object":{"id":"sub_2","customer":"cus_2","status":"canceled","current_period_end":1791000000}}}`
	event, err := Verify([]byte(payload), sign(t, payload, now, secret), secret, now)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	sub, _ := event.Subscription()
	if sub.PeriodEnd != 1791000000 || sub.Status != "canceled" {
		t.Fatalf("fallback not read: %+v", sub)
	}
}
