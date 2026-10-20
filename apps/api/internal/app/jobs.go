package app

import (
	"context"
	"crypto/tls"
	"docping/api/internal/domain"
	"errors"
	"fmt"
	"mime"
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
func (a *App) sendEmail(to, subject, body string) error {
	if a.Config.SMTPHost == "" || a.Config.SMTPFrom == "" {
		return errors.New("e-mail não configurado")
	}
	if !validEmail(to) || !validEmail(a.Config.SMTPFrom) || strings.ContainsAny(subject, "\r\n") {
		return errors.New("destinatário inválido")
	}
	addr := net.JoinHostPort(a.Config.SMTPHost, a.Config.SMTPPort)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return errors.New("provedor de e-mail indisponível")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	client, err := smtp.NewClient(conn, a.Config.SMTPHost)
	if err != nil {
		return errors.New("provedor de e-mail indisponível")
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: a.Config.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return errors.New("falha na conexão TLS de e-mail")
		}
	} else if a.Config.SMTPRequireTLS {
		return errors.New("o provedor precisa oferecer TLS")
	}
	if a.Config.SMTPUser != "" {
		if err = client.Auth(smtp.PlainAuth("", a.Config.SMTPUser, a.Config.SMTPPass, a.Config.SMTPHost)); err != nil {
			return errors.New("autenticação do provedor de e-mail falhou")
		}
	}
	if err = client.Mail(a.Config.SMTPFrom); err != nil {
		return errors.New("remetente recusado pelo provedor")
	}
	if err = client.Rcpt(to); err != nil {
		return errors.New("destinatário recusado pelo provedor")
	}
	data, err := client.Data()
	if err != nil {
		return errors.New("provedor recusou a mensagem")
	}
	message := "From: " + a.Config.SMTPFrom + "\r\nTo: " + to + "\r\nSubject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\nMessage-ID: <" + domain.ID() + "@" + messageIDDomain(a.Config.SMTPFrom) + ">\r\nAuto-Submitted: auto-generated\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n" + body + "\r\n"
	if _, err = data.Write([]byte(message)); err != nil {
		return errors.New("falha no envio ao provedor")
	}
	if err = data.Close(); err != nil {
		return errors.New("provedor não confirmou o recebimento")
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
