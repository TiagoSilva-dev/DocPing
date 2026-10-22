// Package mailer sends transactional e-mail over HTTPS. It is a leaf, like
// internal/billing: no persistence, no domain knowledge, one external call.
//
// It exists because SMTP is not always reachable. A platform that blocks
// outbound SMTP -- Railway does so on every plan below Pro -- makes the port
// time out silently, and an application whose only transport is SMTP then has
// no way to tell a customer anything at all. Over HTTPS the message leaves
// through the same channel as every other API call this binary makes.
package mailer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const endpoint = "https://api.resend.com/emails"

// Resend is a configured client. The zero value is not usable; use New.
type Resend struct {
	key  string
	url  string
	http *http.Client
}

func New(key string) *Resend { return NewAt(key, endpoint) }

// NewAt is New pointed somewhere else. Only a test uses it, and it is
// exported for that reason alone: a test that reaches the real provider is
// not a test.
func NewAt(key, url string) *Resend {
	return &Resend{key: key, url: url, http: &http.Client{Timeout: 20 * time.Second}}
}

// Error is a refusal the provider described. Message is for the log, never
// for the customer: it is English and written for developers.
type Error struct {
	Status  int
	Name    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("resend %d %s: %s", e.Status, e.Name, e.Message)
}

// Send delivers one plain-text message. The body is text on purpose: every
// message this product sends is a sentence and a link, and an HTML part would
// only add a second copy to keep in agreement with the first.
func (r *Resend) Send(ctx context.Context, from, to, subject, body string) error {
	payload, err := json.Marshal(map[string]any{
		"from":    from,
		"to":      []string{to},
		"subject": subject,
		"text":    body,
	})
	if err != nil {
		return err
	}
	// One retry, and only for the provider's own rate limit. A burst of
	// reminders can cross it, and the second attempt a second later costs
	// nothing; anything else that failed is reported rather than repeated,
	// because a repeated send is a duplicate message to a customer.
	for attempt := 0; ; attempt++ {
		err = r.post(ctx, payload)
		var refusal *Error
		if attempt == 0 && errors.As(err, &refusal) && refusal.Status == http.StatusTooManyRequests {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
			continue
		}
		return err
	}
}

func (r *Resend) post(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.key)
	req.Header.Set("Content-Type", "application/json")
	res, err := r.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		var wrapper struct {
			Name, Message string
		}
		_ = json.Unmarshal(data, &wrapper)
		return &Error{Status: res.StatusCode, Name: wrapper.Name, Message: wrapper.Message}
	}
	// An accepted message carries an id. Its absence means the call did not
	// do what it appeared to, and reporting success there would record a
	// delivery that never happened.
	var accepted struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &accepted); err != nil || accepted.ID == "" {
		return errors.New("resend accepted the request without returning a message id")
	}
	return nil
}
