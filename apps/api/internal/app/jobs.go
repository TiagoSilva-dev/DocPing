package app

import (
	"context"
	"crypto/tls"
	"docping/api/internal/domain"
	"errors"
	"fmt"
	"log"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// messageIDDomain takes the sender's domain so the Message-ID a provider sees
// belongs to the sending domain. Subjects carry accents and an em dash, so the
// header has to be RFC 2047 encoded instead of written as raw UTF-8 bytes,
// which some providers reject and others deliver as mojibake.
func messageIDDomain(from string) string {
	if n := strings.LastIndex(from, "@"); n >= 0 && n+1 < len(from) {
		return from[n+1:]
	}
	return "docping.invalid"
}

// smtpTLSConfig verifies the provider's certificate against the system roots
// and the configured hostname. A test may supply its own pool; nothing else
// ever does, and verification is never skipped.
func (a *App) smtpTLSConfig() *tls.Config {
	return &tls.Config{ServerName: a.Config.SMTPHost, MinVersion: tls.VersionTLS12, RootCAs: a.Config.smtpRoots}
}

// smtpFailure keeps the two audiences apart. The customer and the delivery
// record get the short Portuguese sentence they can act on; the server log
// gets the provider's actual complaint, which is the only thing that tells a
// blocked port apart from a rejected certificate or a refused sender. Saying
// only the friendly sentence turned a platform that silently blocks outbound
// SMTP into a mystery, so the real cause is written down now.
//
// The underlying error never carries the password: it is the transport's own
// message, or the server's reply to a command.
func (a *App) smtpFailure(stage, friendly string, err error) error {
	log.Printf("smtp %s failed (%s, mode %s): %v", stage, net.JoinHostPort(a.Config.SMTPHost, a.Config.SMTPPort), a.Config.SMTPTLSMode, err)
	return errors.New(friendly)
}

// sendEmail delivers one message by whichever transport this deployment has.
// HTTPS wins when it is configured: it is the one that works everywhere,
// including on a platform that blocks the SMTP port outright. SMTP stays for
// the local stack, where Mailpit is the whole point.
// sendEmail delivers a text-only message. Notices that are a sentence and
// nothing else still use it.
func (a *App) sendEmail(to, subject, body string) error {
	return a.sendEmailHTML(to, subject, body, "")
}

// sendEmailHTML delivers a message with both parts. The text is never
// optional: a message with no text alternative reads as suspect to a filter,
// and some people read everything as text on purpose.
func (a *App) sendEmailHTML(to, subject, body, htmlBody string) error {
	if !a.Config.CanSendEmail() {
		return errors.New("e-mail não configurado")
	}
	if !validEmail(to) || !validEmail(a.Config.Sender()) || strings.ContainsAny(subject, "\r\n") {
		return errors.New("destinatário inválido")
	}
	if a.Mail != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.Mail.Send(ctx, a.Config.Sender(), to, subject, body, htmlBody); err != nil {
			// The provider's own words go to the log; the customer and the
			// delivery record get the sentence they can act on.
			log.Printf("resend send failed (from %s): %v", a.Config.Sender(), err)
			return errors.New("provedor de e-mail recusou a mensagem")
		}
		return nil
	}
	return a.sendSMTP(to, subject, body, htmlBody)
}

func (a *App) sendSMTP(to, subject, body, htmlBody string) error {
	addr := net.JoinHostPort(a.Config.SMTPHost, a.Config.SMTPPort)
	implicit := a.Config.SMTPTLSMode == smtpTLSImplicit
	var conn net.Conn
	var err error
	if implicit {
		// The port answers in TLS, so the handshake comes before the SMTP
		// greeting. There is no plaintext phase to fall back to.
		conn, err = tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, a.smtpTLSConfig())
		if err != nil {
			return a.smtpFailure("implicit TLS dial", "falha na conexão TLS de e-mail", err)
		}
	} else if conn, err = net.DialTimeout("tcp", addr, 10*time.Second); err != nil {
		return a.smtpFailure("dial", "provedor de e-mail indisponível", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(conn, a.Config.SMTPHost)
	if err != nil {
		return a.smtpFailure("greeting", "provedor de e-mail indisponível", err)
	}
	defer client.Close()
	if !implicit {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(a.smtpTLSConfig()); err != nil {
				return a.smtpFailure("starttls", "falha na conexão TLS de e-mail", err)
			}
		} else if a.Config.SMTPRequireTLS {
			return errors.New("o provedor precisa oferecer TLS")
		}
	}
	if a.Config.SMTPUser != "" {
		if err = client.Auth(smtp.PlainAuth("", a.Config.SMTPUser, a.Config.SMTPPass, a.Config.SMTPHost)); err != nil {
			return a.smtpFailure("auth", "autenticação do provedor de e-mail falhou", err)
		}
	}
	if err = client.Mail(a.Config.Sender()); err != nil {
		return a.smtpFailure("sender", "remetente recusado pelo provedor", err)
	}
	if err = client.Rcpt(to); err != nil {
		return a.smtpFailure("recipient", "destinatário recusado pelo provedor", err)
	}
	data, err := client.Data()
	if err != nil {
		return a.smtpFailure("data", "provedor recusou a mensagem", err)
	}
	message := a.mimeMessage(to, subject, body, htmlBody)
	if _, err = data.Write([]byte(message)); err != nil {
		return a.smtpFailure("write", "falha no envio ao provedor", err)
	}
	if err = data.Close(); err != nil {
		return a.smtpFailure("commit", "provedor não confirmou o recebimento", err)
	}
	_ = client.Quit()
	return nil
}

// prepareDelivery persists an attempt before any SMTP I/O. The returned
// effect runs only after commit, then records the provider's actual result.
// Pending attempts are never blindly replayed after a process interruption.
func (a *App) prepareDelivery(s *domain.State, r domain.Request, kind, channel string) (*domain.Delivery, func() error) {
	d := &domain.Delivery{ID: domain.ID(), OrgID: r.OrgID, RequestID: r.ID, Channel: channel, Kind: kind, CreatedAt: domain.Now()}
	var effect func() error
	switch {
	case channel == "whatsapp_opened":
		d.Status = "opened"
	case s.Organizations[r.OrgID].Demo:
		d.Status = "failed"
		d.Error = "Demonstração: e-mail externo desativado."
	case s.Clients[r.ClientID].Email == "":
		d.Status = "failed"
		d.Error = "Cliente sem e-mail cadastrado."
	default:
		client, org := s.Clients[r.ClientID], s.Organizations[r.OrgID]
		link, _ := a.issueLink(s, r)
		body := fmt.Sprintf("Olá, %s.\n\n%s solicita: %s (%s).\nPrazo: %s.\n\nEnvie e acompanhe seus documentos: %s\n\nSe houver uma correção, consulte o motivo no portal.\n", client.ContactName, org.Name, r.Title, r.Competence, r.DueDate, link)
		d.Status = "pending"
		effect = func() error {
			status, reason := "accepted", ""
			allowed := true
			checkCtx, checkCancel := context.WithTimeout(context.Background(), 10*time.Second)
			checkErr := a.Store.Run(checkCtx, false, func(current *domain.State) error {
				req, ok := current.Requests[r.ID]
				if !ok || req.Status == "completed" || req.Status == "cancelled" {
					allowed = false
					reason = "Solicitação encerrada antes do envio."
				}
				if current.Organizations[r.OrgID].Demo {
					allowed = false
					reason = "Demonstração: e-mail externo desativado."
				}
				if current.Clients[r.ClientID].Email == "" {
					allowed = false
					reason = "Cliente sem e-mail cadastrado."
				}
				return nil
			})
			checkCancel()
			if checkErr != nil {
				return checkErr
			}
			if !allowed {
				status = "failed"
			} else if err := a.sendEmail(client.Email, "Documentos solicitados — "+org.Name, body); err != nil {
				status, reason = "failed", err.Error()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return a.Store.Run(ctx, true, func(current *domain.State) error {
				saved, ok := current.Deliveries[d.ID]
				if !ok {
					return missing()
				}
				saved.Status = status
				saved.Error = reason
				current.Deliveries[saved.ID] = saved
				current.Audit(saved.OrgID, "", "notification."+status, saved.ID)
				*d = saved
				return nil
			})
		}
	}
	s.Deliveries[d.ID] = *d
	s.Audit(r.OrgID, "", "notification."+d.Status, d.ID)
	return d, effect
}
func (a *App) notify(c *call) (any, error) {
	r, err := c.ownRequest(c.R.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if r.Status == "completed" || r.Status == "cancelled" {
		return nil, fail(409, "request_closed", "Esta solicitação já está encerrada.")
	}
	var in struct{ Channel string }
	if err = decode(c.R, &in); err != nil {
		return nil, err
	}
	if in.Channel != "email" && in.Channel != "whatsapp_opened" {
		return nil, invalid("Canal inválido.")
	}
	for _, d := range c.S.Deliveries {
		t, _ := time.Parse(time.RFC3339, d.CreatedAt)
		if d.RequestID == r.ID && d.Channel == in.Channel && time.Since(t) < time.Minute {
			return nil, fail(429, "notification_recent", "Aguarde um minuto antes de tentar novamente.")
		}
	}
	d, effect := a.prepareDelivery(c.S, r, "manual", in.Channel)
	if effect != nil {
		c.effects = append(c.effects, effect)
	}
	c.S.Audit(c.Org.ID, c.User.ID, "notification.requested", r.ID)
	return d, nil
}

// awaitingClient reports whether any item of the request still needs an upload
// from the client: a required item never sent, or any item sent back for
// correction.
func awaitingClient(r domain.Request) bool {
	for _, item := range r.Items {
		if item.Status == "correction_required" || item.Status == "pending" && item.Required {
			return true
		}
	}
	return false
}
func (a *App) tick(ctx context.Context, now time.Time) {
	effects := []func() error{}
	err := a.Store.Run(ctx, true, func(s *domain.State) error {
		for id, d := range s.Deliveries {
			created, e := time.Parse(time.RFC3339, d.CreatedAt)
			if e == nil && d.Status == "pending" && now.Sub(created) > 5*time.Minute {
				d.Status = "failed"
				d.Error = "Não foi possível confirmar o envio interrompido. Tente novamente manualmente."
				s.Deliveries[id] = d
			}
		}
		local := now.In(a.Config.Location)
		for id, schedule := range s.Schedules {
			if !schedule.Active || s.Organizations[schedule.OrgID].Demo {
				continue
			}
			created, err := time.Parse(time.RFC3339, schedule.CreatedAt)
			if err != nil {
				continue
			}
			execution := domain.MonthDay(local.Year(), local.Month(), schedule.DayOfMonth, a.Config.Location)
			if local.Before(execution) || created.In(a.Config.Location).After(execution) {
				continue
			}
			competence := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, a.Config.Location).AddDate(0, -1, 0).Format("2006-01")
			t, ok := s.Templates[schedule.TemplateID]
			if !ok || !t.Active {
				continue
			}
			if len(schedule.Items) > 0 {
				t.Items = schedule.Items
			}
			generated := false
			for _, clientID := range schedule.ClientIDs {
				exists := false
				for _, run := range s.Runs {
					if run.ScheduleID == id && run.ClientID == clientID && run.Competence == competence {
						exists = true
						break
					}
				}
				if exists {
					continue
				}
				client, ok := s.Clients[clientID]
				if !ok || !client.Active || client.OrgID != schedule.OrgID {
					continue
				}
				// The allowance is checked here too, and client by client:
				// a recurrence that outgrew the plan must stop adding, not
				// become the one way around the limit. The run is left
				// unrecorded, so the request appears by itself once the
				// workspace frees a slot or moves up a plan.
				if org, ok := s.Organizations[schedule.OrgID]; ok {
					if limit := org.Entitlement().ActiveRequests; limit > 0 && activeRequests(s, org.ID) >= limit {
						log.Printf("recurrence %s skipped client %s: organization %s is at its plan limit of %d active requests", id, clientID, org.ID, limit)
						continue
					}
				}
				r := newRequest(schedule.OrgID, client, t, schedule.Name, competence, execution.AddDate(0, 0, schedule.DueDays).Format("2006-01-02"))
				r.Sensitive = schedule.Sensitive
				r.ScheduleID = id
				r.Recurring = true
				generated = true
				s.Requests[r.ID] = r
				run := domain.ScheduleRun{ID: domain.ID(), OrgID: r.OrgID, ScheduleID: id, ClientID: clientID, Competence: competence, RequestID: r.ID, CreatedAt: now.UTC().Format(time.RFC3339)}
				s.Runs[run.ID] = run
				s.Audit(r.OrgID, "", "schedule.request_created", r.ID)
				_, effect := a.prepareDelivery(s, r, "initial", "email")
				if effect != nil {
					effects = append(effects, effect)
				}
			}
			if generated {
				schedule.LastRun = now.UTC().Format(time.RFC3339)
			}
			schedule.NextRun = domain.NextExecution(schedule, local).UTC().Format(time.RFC3339)
			s.Schedules[id] = schedule
		}
		for _, r := range s.Requests {
			o := s.Organizations[r.OrgID]
			if !o.RemindersEnabled || o.Demo || r.Status == "completed" || r.Status == "cancelled" {
				continue
			}
			// Only remind while the client still has something to do. A request
			// whose documents are all in review is waiting on the office, and a
			// reminder there reads as if the upload had not arrived.
			if !awaitingClient(r) {
				continue
			}
			created, err := time.Parse(time.RFC3339, r.CreatedAt)
			if err != nil {
				continue
			}
			latestDay := 0
			for _, day := range o.ReminderDays {
				if !local.Before(created.In(a.Config.Location).AddDate(0, 0, day)) && day > latestDay {
					latestDay = day
				}
			}
			if latestDay == 0 {
				continue
			}
			kind := fmt.Sprintf("reminder_%d", latestDay)
			done := false
			for _, d := range s.Deliveries {
				if d.RequestID == r.ID && d.Kind == kind {
					done = true
					break
				}
			}
			if !done {
				_, effect := a.prepareDelivery(s, r, kind, "email")
				if effect != nil {
					effects = append(effects, effect)
				}
			}

		}

		return nil
	})
	if err == nil {
		for _, effect := range effects {
			_ = effect()
		}
	}
}

// mimeMessage builds the message itself. With an html part it is
// multipart/alternative: the client picks, and a text-only reader still gets
// a readable message.
//
// The html part is quoted-printable because its lines are long and SMTP caps
// a line at a thousand characters. Breaking them by hand would break them
// inside attributes; quoted-printable inserts soft breaks the receiver undoes.
func (a *App) mimeMessage(to, subject, body, htmlBody string) string {
	headers := "From: " + a.Config.Sender() + "\r\nTo: " + to +
		"\r\nSubject: " + mime.QEncoding.Encode("utf-8", subject) +
		"\r\nDate: " + time.Now().Format(time.RFC1123Z) +
		"\r\nMessage-ID: <" + domain.ID() + "@" + messageIDDomain(a.Config.Sender()) + ">" +
		"\r\nAuto-Submitted: auto-generated\r\nMIME-Version: 1.0\r\n"
	if htmlBody == "" {
		return headers + "Content-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + body + "\r\n"
	}
	// The boundary is random so that it cannot appear inside either part.
	boundary := "docping-" + domain.ID()
	var encoded strings.Builder
	writer := quotedprintable.NewWriter(&encoded)
	_, _ = writer.Write([]byte(htmlBody))
	_ = writer.Close()
	return headers + "Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n" +
		"--" + boundary + "\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + body + "\r\n" +
		"--" + boundary + "\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n" + encoded.String() + "\r\n" +
		"--" + boundary + "--\r\n"
}
