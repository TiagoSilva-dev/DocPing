package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"docping/api/internal/domain"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

func passwordHash(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic(err)
	}
	dk := argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32)
	return "argon2id$v=19$m=65536,t=3,p=1$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(dk)
}
func passwordOK(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[2] != "m=65536,t=3,p=1" {
		argon2.IDKey([]byte(password), []byte("invalid-account-salt"), 3, 64*1024, 1, 32)
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[3])
	want, e2 := base64.RawStdEncoding.DecodeString(parts[4])
	if e1 != nil || e2 != nil || len(salt) != 16 || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 3, 64*1024, 1, 32)
	return subtle.ConstantTimeCompare(want, got) == 1
}
func validEmail(v string) bool {
	if v == "" || len(v) > 254 {
		return false
	}
	a, err := mail.ParseAddress(v)
	return err == nil && a.Address == v && !strings.ContainsAny(v, "\r\n")
}
func (c *call) sessionResponse() any {
	return map[string]any{"environment": c.A.Config.Env, "user": map[string]string{"id": c.User.ID, "name": c.User.Name, "email": c.User.Email}, "organization": c.Org, "csrfToken": c.Session.CSRF, "billing": c.billingState()}
}

// billingState is what the workspace needs to decide which screen to show:
// whether this installation charges at all, whether this organization may
// work, and the allowance it is working inside. The figures come from the
// same table the server enforces, so the screen can never promise more than
// the API will accept.
func (c *call) billingState() map[string]any {
	e := c.Org.Entitlement()
	return map[string]any{
		"enabled":           c.A.Config.Billing(),
		"entitled":          !c.A.Config.Billing() || c.Org.Entitled(),
		"plan":              c.Org.Plan,
		"status":            c.Org.SubscriptionStatus,
		"periodEnd":         c.Org.PeriodEnd,
		"cancelAtPeriodEnd": c.Org.CancelAtPeriodEnd,
		"manageable":        c.Org.StripeCustomerID != "",
		"activeRequests":    activeRequests(c.S, c.Org.ID),
		"limits": map[string]any{
			"activeRequests": e.ActiveRequests,
			"storageBytes":   e.StorageBytes,
			"clients":        c.Org.ClientLimit,
		},
	}
}
func (a *App) session(c *call) (any, error) { return c.sessionResponse(), nil }
func (c *call) newSession(u domain.User) any {
	raw := token()
	s := domain.Session{Hash: hash(raw), UserID: u.ID, CSRF: token(), ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)}
	c.S.Sessions[s.Hash] = s
	c.User = u
	c.Org = c.S.Organizations[u.OrgID]
	c.Session = s
	http.SetCookie(c.W, &http.Cookie{Name: "docping_session", Value: raw, Path: "/", HttpOnly: true, Secure: c.A.Config.Env == "production", SameSite: http.SameSiteLaxMode, MaxAge: 86400})
	return c.sessionResponse()
}
func (a *App) register(c *call) (any, error) {
	var input struct {
		OrganizationName      string `json:"organizationName"`
		Name, Email, Password string
	}
	if err := decode(c.R, &input); err != nil {
		return nil, err
	}
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if !title(input.OrganizationName, 180) || !title(input.Name, 120) || !validEmail(input.Email) || len(input.Password) < 12 || len(input.Password) > 128 {
		return nil, invalid("Informe nome, escritório, e-mail válido e senha com 12 a 128 caracteres.")
	}
	for _, u := range c.S.Users {
		if u.Email == input.Email {
			return nil, fail(409, "email_exists", "Este e-mail já possui uma conta.")
		}
	}
	// A workspace opened where billing is on starts with no plan: it is the
	// subscription that opens it, and the gate in authenticate holds it at
	// the subscribe screen until Stripe says one exists. Where billing is off
	// -- a development machine, the test suite -- it starts as the founder
	// pilot, exactly as every workspace did before money existed.
	plan := domain.PlanFounderPilot
	if a.Config.Billing() {
		plan = ""
	}
	o := domain.Organization{ID: domain.ID(), Name: strings.TrimSpace(input.OrganizationName), ClientLimit: a.Config.ClientLimit, Plan: plan, AIReview: domain.AIReviewSuggest, RetentionDays: 365, ReminderDays: []int{3, 6}, RemindersEnabled: true, CreatedAt: domain.Now()}
	u := domain.User{ID: domain.ID(), OrgID: o.ID, Name: strings.TrimSpace(input.Name), Email: input.Email, StoredHash: passwordHash(input.Password), EmailPending: a.Config.VerifyEmail, CreatedAt: domain.Now()}
	if !u.EmailPending {
		u.EmailVerifiedAt = domain.Now()
	}
	c.S.Organizations[o.ID] = o
	c.S.Users[u.ID] = u
	starterTemplates(c.S, o.ID)
	c.S.Audit(o.ID, u.ID, "organization.registered", o.ID)
	// Without SMTP there is no way to confirm an address, which only happens
	// on a development machine: the workspace opens immediately, exactly as it
	// did before confirmation existed.
	if !u.EmailPending {
		return c.newSession(u), nil
	}
	a.issueVerification(c, u)
	return map[string]any{"status": "verification_sent", "email": u.Email}, nil
}

// issueVerification replaces any outstanding link for the user and sends a new
// one after the transaction commits. A send that fails is recorded and left
// for the person to retry from the sign-in screen: the account already exists,
// so failing the request here would report a registration that did happen as
// if it had not.
func (a *App) issueVerification(c *call, u domain.User) {
	for h, v := range c.S.Verifications {
		if v.UserID == u.ID && !v.Used {
			v.Used = true
			c.S.Verifications[h] = v
		}
	}
	raw := token()
	v := domain.EmailVerification{Hash: hash(raw), UserID: u.ID, Email: u.Email, ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339), SentAt: domain.Now()}
	c.S.Verifications[v.Hash] = v
	to, org := u.Email, u.OrgID
	c.effects = append(c.effects, func() error {
		body := "Confirme seu e-mail para ativar o acesso do seu escritório no DocPing.\n\nUse este link em até 24 horas: " + a.Config.Origin + "/confirmar-email?token=" + raw + "\n\nSe não foi você que criou a conta, ignore esta mensagem: sem a confirmação, o espaço é apagado automaticamente.\n"
		if err := a.sendEmail(to, "Confirme seu e-mail — DocPing", body); err != nil {
			log.Printf("verification e-mail not sent: %v", err)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = a.Store.Run(ctx, true, func(s *domain.State) error {
				s.Audit(org, u.ID, "auth.verification_failed", u.ID)
				return nil
			})
		}
		return nil
	})
}

// verifyEmail confirms the address and opens the session, so the link the
// person just clicked lands them inside the workspace.
func (a *App) verifyEmail(c *call) (any, error) {
	var input struct{ Token string }
	if err := decode(c.R, &input); err != nil {
		return nil, err
	}
	expiredLink := fail(410, "verification_expired", "Este link de confirmação expirou. Peça um novo na tela de entrada.")
	v, ok := c.S.Verifications[hash(input.Token)]
	if !ok || v.Used || expired(v.ExpiresAt) {
		return nil, expiredLink
	}
	u, ok := c.S.Users[v.UserID]
	// The address is part of the token: a link issued for an e-mail the
	// account no longer uses confirms nothing.
	if !ok || u.Email != v.Email {
		return nil, expiredLink
	}
	v.Used = true
	c.S.Verifications[v.Hash] = v
	if u.EmailPending {
		u.EmailPending = false
		u.EmailVerifiedAt = domain.Now()
		c.S.Users[u.ID] = u
		c.S.Audit(u.OrgID, u.ID, "auth.email_verified", u.ID)
	}
	return c.newSession(u), nil
}

// resendVerification answers the same way whether or not the account exists,
// so the endpoint cannot be used to find out who has an account.
func (a *App) resendVerification(c *call) (any, error) {
	var input struct{ Email string }
	if err := decode(c.R, &input); err != nil {
		return nil, err
	}
	neutral := map[string]string{"message": "Se houver uma conta aguardando confirmação, enviaremos um novo link."}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if !validEmail(email) {
		return neutral, nil
	}
	for _, u := range c.S.Users {
		if u.Email != email || !u.EmailPending {
			continue
		}
		for _, v := range c.S.Verifications {
			sent, err := time.Parse(time.RFC3339, v.SentAt)
			// Two minutes between links: a resend button held down must not
			// turn into a mail flood aimed at the address.
			if v.UserID == u.ID && err == nil && time.Since(sent) < 2*time.Minute {
				return neutral, nil
			}
		}
		a.issueVerification(c, u)
		break
	}
	return neutral, nil
}
func (a *App) login(c *call) (any, error) {
	var input struct{ Email, Password string }
	if err := decode(c.R, &input); err != nil {
		return nil, err
	}
	if len(input.Password) > 128 || len(input.Email) > 254 {
		return nil, invalid("E-mail ou senha inválidos.")
	}
	email := strings.ToLower(strings.TrimSpace(input.Email))
	var user domain.User
	for _, u := range c.S.Users {
		if u.Email == email {
			user = u
			break
		}
	}
	if !passwordOK(input.Password, user.StoredHash) {
		return nil, fail(401, "invalid_credentials", "E-mail ou senha incorretos.")
	}
	// Checked only after the password matched, so the response never reveals
	// the state of an account to someone who does not hold its credentials.
	if user.EmailPending {
		return nil, fail(403, "email_unverified", "Confirme seu e-mail para entrar. Podemos enviar um novo link de confirmação.")
	}
	c.S.Audit(user.OrgID, user.ID, "auth.login", user.ID)
	return c.newSession(user), nil
}
func (a *App) logout(c *call) (any, error) {
	s := c.Session
	s.Revoked = true
	c.S.Sessions[s.Hash] = s
	http.SetCookie(c.W, &http.Cookie{Name: "docping_session", Path: "/", Value: "", HttpOnly: true, Secure: a.Config.Env == "production", SameSite: http.SameSiteLaxMode, MaxAge: -1})
	return nil, nil
}
func (a *App) demo(c *call) (any, error) {
	// Each demonstration is a real organization with sample clients and
	// requests, so an unbounded number of them is an unbounded amount of
	// state. The per-IP rate limit slows one source down; this ceiling is what
	// holds when the requests come from many.
	live := 0
	for _, org := range c.S.Organizations {
		if org.Demo {
			live++
		}
	}
	if live >= a.Config.DemoMaxActive {
		return nil, fail(503, "demo_unavailable", "A demonstração está com todas as vagas ocupadas neste momento. Tente novamente mais tarde.")
	}
	o := domain.Organization{ID: domain.ID(), Name: "Aurora Contabilidade", Demo: true, ClientLimit: 30, Plan: "demo", AIReview: domain.AIReviewAuto, RetentionDays: 365, ReminderDays: []int{3, 6}, RemindersEnabled: false, CreatedAt: domain.Now()}
	u := domain.User{ID: domain.ID(), OrgID: o.ID, Name: "Mariana", Email: "demo-" + o.ID + "@example.invalid", CreatedAt: domain.Now()}
	c.S.Organizations[o.ID] = o
	c.S.Users[u.ID] = u
	templates := starterTemplates(c.S, o.ID)
	now := time.Now().In(a.Config.Location)
	names := []string{"Horizonte Arquitetura", "Café Amora", "Studio Nativa", "Atlas Engenharia", "Vila Verde Floricultura", "Solaris Tecnologia", "Pão & Prosa", "Casa Oliva", "Lima Consultoria", "Raiz Comércio", "Monteiro Saúde", "Oficina Norte"}
	for n, name := range names {
		client := domain.Client{ID: domain.ID(), OrgID: o.ID, Name: name, ContactName: []string{"Ana Martins", "Pedro Lima", "Luiza Reis", "Rafael Costa"}[n%4], Email: fmt.Sprintf("cliente%d@example.invalid", n+1), Phone: "5511999990000", Active: true, CreatedAt: now.AddDate(0, -2, -n).UTC().Format(time.RFC3339)}
		c.S.Clients[client.ID] = client
		r := newRequest(o.ID, client, templates[0], "Fechamento mensal", now.AddDate(0, -1, 0).Format("2006-01"), now.AddDate(0, 0, n-3).Format("2006-01-02"))
		r.CreatedAt = now.AddDate(0, 0, -8).UTC().Format(time.RFC3339)
		r.UpdatedAt = now.Add(-time.Duration(n) * time.Hour).UTC().Format(time.RFC3339)
		if n < 8 {
			for j := range r.Items {
				if j >= n%4+1 && n != 7 {
					break
				}
				f := domain.DocumentFile{ID: domain.ID(), Name: "exemplo-" + fmt.Sprint(j+1) + ".pdf", MIMEType: "application/pdf", CreatedAt: r.UpdatedAt, ScanStatus: "sample", Sample: true}
				f.ObjectKey = o.ID + "/" + f.ID
				sample := samplePDF()
				f.Size = int64(len(sample))

				r.Items[j].Files = append(r.Items[j].Files, f)
				r.Items[j].Status = "uploaded"
				if n >= 5 {
					r.Items[j].Status = "approved"
				}
			}
			if n == 3 {
				r.Items[0].Status = "correction_required"
				r.Items[0].CorrectionReason = "A imagem está cortada. Envie o documento inteiro."
			}
			domain.Recalculate(&r)
		}
		c.S.Requests[r.ID] = r
		if n < 3 {
			schedule := domain.Schedule{ID: domain.ID(), OrgID: o.ID, Name: "Fechamento de " + name, TemplateID: templates[0].ID, ClientIDs: []string{client.ID}, DayOfMonth: 5, DueDays: 10, Active: true, CreatedAt: domain.Now()}
			schedule.NextRun = domain.NextExecution(schedule, now).UTC().Format(time.RFC3339)
			c.S.Schedules[schedule.ID] = schedule
		}
	}
	c.S.Audit(o.ID, u.ID, "demo.created", o.ID)
	return c.newSession(u), nil
}
func starterTemplates(s *domain.State, org string) []domain.Template {
	definitions := []struct {
		name, description string
		items             []string
	}{{"Fechamento mensal", "Tudo o que precisamos para fechar o mês.", []string{"Extrato bancário", "Notas fiscais", "Comprovantes de despesas", "Outros comprovantes"}}, {"Abertura de empresa", "Documentos para começar com tranquilidade.", []string{"Documento dos sócios", "Comprovante de endereço", "Contrato social"}}, {"Admissão de funcionário", "Organize a entrada de uma nova pessoa.", []string{"Documento de identificação", "Comprovante de residência", "Documentação de admissão"}}}
	out := []domain.Template{}
	for _, d := range definitions {
		t := domain.Template{ID: domain.ID(), OrgID: org, Name: d.name, Description: d.description, Active: true, CreatedAt: domain.Now(), Items: []domain.TemplateItem{}}
		for n, name := range d.items {
			t.Items = append(t.Items, domain.TemplateItem{ID: domain.ID(), Name: name, Instruction: "Envie um PDF ou uma foto legível, com todas as páginas.", Required: n < 3, MaxFiles: 5})
		}
		s.Templates[t.ID] = t
		out = append(out, t)
	}
	return out
}
func samplePDF() []byte {
	content := "BT /F1 16 Tf 48 740 Td (DOCPING - DOCUMENTO DE EXEMPLO) Tj 0 -30 Td /F1 11 Tf (Arquivo sintetico para demonstracao. Sem dados pessoais.) Tj 0 -24 Td (Nao usar para fins contabilisticos ou fiscais.) Tj ET"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)}
	out := "%PDF-1.4\n"
	offsets := []int{0}
	for n, obj := range objects {
		offsets = append(offsets, len(out))
		out += fmt.Sprintf("%d 0 obj\n%s\nendobj\n", n+1, obj)
	}
	xref := len(out)
	out += "xref\n0 6\n0000000000 65535 f \n"
	for _, offset := range offsets[1:] {
		out += fmt.Sprintf("%010d 00000 n \n", offset)
	}
	out += fmt.Sprintf("trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return []byte(out)
}
func (a *App) forgotPassword(c *call) (any, error) {
	var input struct{ Email string }
	if err := decode(c.R, &input); err != nil {
		return nil, err
	}
	for _, u := range c.S.Users {
		if u.Email == strings.ToLower(strings.TrimSpace(input.Email)) && !c.S.Organizations[u.OrgID].Demo {
			raw := token()
			reset := domain.PasswordReset{Hash: hash(raw), UserID: u.ID, ExpiresAt: time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)}
			c.S.Resets[reset.Hash] = reset
			to := u.Email
			c.effects = append(c.effects, func() error {
				if err := a.sendEmail(to, "Redefina sua senha do DocPing", "Use este link em até 30 minutos: "+a.Config.Origin+"/redefinir-senha?token="+raw); err != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = a.Store.Run(ctx, true, func(s *domain.State) error {
						r := s.Resets[reset.Hash]
						r.Used = true
						s.Resets[r.Hash] = r
						return nil
					})
				}
				return nil
			})
			break
		}
	}
	return map[string]string{"message": "Se o e-mail estiver cadastrado, enviaremos as instruções."}, nil
}
func (a *App) resetPassword(c *call) (any, error) {
	var input struct{ Token, Password string }
	if err := decode(c.R, &input); err != nil {
		return nil, err
	}
	if len(input.Password) < 12 || len(input.Password) > 128 {
		return nil, invalid("Use uma senha com 12 a 128 caracteres.")
	}
	r, ok := c.S.Resets[hash(input.Token)]
	if !ok || r.Used || expired(r.ExpiresAt) {
		return nil, fail(410, "reset_expired", "O link de recuperação expirou.")
	}
	u, ok := c.S.Users[r.UserID]
	if !ok {
		return nil, fail(410, "reset_expired", "O link de recuperação expirou.")
	}
	u.StoredHash = passwordHash(input.Password)
	// Receiving the reset link proves control of the address just as the
	// confirmation link does, so it also lifts the pending state instead of
	// leaving the account with a new password it still cannot sign in with.
	if u.EmailPending {
		u.EmailPending = false
		u.EmailVerifiedAt = domain.Now()
		c.S.Audit(u.OrgID, u.ID, "auth.email_verified", u.ID)
	}
	c.S.Users[u.ID] = u
	r.Used = true
	c.S.Resets[r.Hash] = r
	for k, s := range c.S.Sessions {
		if s.UserID == u.ID {
			s.Revoked = true
			c.S.Sessions[k] = s
		}
	}
	c.S.Audit(u.OrgID, u.ID, "auth.password_reset", u.ID)
	return nil, nil
}
