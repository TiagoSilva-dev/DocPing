package app

import (
	"context"
	"crypto/rand"
	"docping/api/internal/domain"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func newRequest(org string, client domain.Client, t domain.Template, title, competence, due string) domain.Request {
	r := domain.Request{ID: domain.ID(), OrgID: org, ClientID: client.ID, ClientName: client.Name, TemplateID: t.ID, Title: title, Competence: competence, DueDate: due, Status: "pending", Items: []domain.RequestItem{}, CreatedAt: domain.Now(), UpdatedAt: domain.Now()}
	for _, item := range t.Items {
		item.ID = domain.ID()
		r.Items = append(r.Items, domain.RequestItem{TemplateItem: item, Status: "pending", Files: []domain.DocumentFile{}, Reviews: []domain.Review{}})
	}
	return r
}
func (a *App) requests(c *call) (any, error) {
	out := []domain.Request{}
	for _, v := range c.S.Requests {
		if v.OrgID == c.Org.ID {
			out = append(out, domain.PublicRequest(v))
		}
	}
	domain.SortRequests(out)
	return out, nil
}
func (a *App) request(c *call) (any, error) {
	r, err := c.ownRequest(c.R.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return domain.PublicRequest(r), nil
}
func (a *App) createRequests(c *call) (any, error) {
	var in struct {
		ClientIDs                  []string `json:"clientIds"`
		TemplateID                 string   `json:"templateId"`
		Title, Competence, DueDate string
		Items                      []domain.TemplateItem
		Recurring, Sensitive       bool
	}
	if err := decode(c.R, &in); err != nil {
		return nil, err
	}
	if len(in.ClientIDs) < 1 || len(in.ClientIDs) > 100 || !title(in.Title, 180) {
		return nil, invalid("Selecione clientes e informe um título.")
	}
	if _, err := time.Parse("2006-01", in.Competence); err != nil {
		return nil, invalid("Competência inválida. Use AAAA-MM.")
	}
	if _, err := time.Parse("2006-01-02", in.DueDate); err != nil {
		return nil, invalid("Prazo inválido. Use AAAA-MM-DD.")
	}
	t, ok := c.S.Templates[in.TemplateID]
	if !ok || t.OrgID != c.Org.ID || !t.Active {
		return nil, invalid("Escolha um modelo ativo do seu escritório.")
	}
	if in.Items != nil {
		if err := validateItems(in.Items); err != nil {
			return nil, err
		}
		t.Items = in.Items
	}
	out := []domain.Request{}
	seen := map[string]bool{}
	if err := c.roomForRequests(countDistinct(in.ClientIDs)); err != nil {
		return nil, err
	}
	for _, id := range in.ClientIDs {
		client, ok := c.S.Clients[id]
		if !ok || client.OrgID != c.Org.ID || !client.Active {
			return nil, invalid("Um cliente selecionado não está ativo neste escritório.")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		if in.Sensitive && client.Email == "" {
			return nil, invalid("Solicitações sensíveis exigem e-mail do responsável.")
		}
		r := newRequest(c.Org.ID, client, t, in.Title, in.Competence, in.DueDate)
		r.Sensitive = in.Sensitive
		c.S.Requests[r.ID] = r
		c.S.Audit(c.Org.ID, c.User.ID, "request.created", r.ID)
		out = append(out, domain.PublicRequest(r))
	}
	if in.Recurring {
		now := time.Now().In(a.Config.Location)
		due, _ := time.ParseInLocation("2006-01-02", in.DueDate, a.Config.Location)
		dueDays := civilDaysBetween(now, due)
		dueDays = max(1, min(90, dueDays))
		s := domain.Schedule{Items: append([]domain.TemplateItem{}, t.Items...), Sensitive: in.Sensitive, ID: domain.ID(), OrgID: c.Org.ID, Name: in.Title, TemplateID: t.ID, ClientIDs: in.ClientIDs, DayOfMonth: now.Day(), DueDays: dueDays, Active: true, CreatedAt: domain.Now()}
		s.NextRun = domain.NextExecution(s, now).UTC().Format(time.RFC3339)
		c.S.Schedules[s.ID] = s
		for n := range out {
			r := c.S.Requests[out[n].ID]
			r.Recurring = true
			r.ScheduleID = s.ID
			c.S.Requests[r.ID] = r
			run := domain.ScheduleRun{ID: domain.ID(), OrgID: r.OrgID, ScheduleID: s.ID, ClientID: r.ClientID, Competence: r.Competence, RequestID: r.ID, CreatedAt: domain.Now()}
			c.S.Runs[run.ID] = run
			out[n] = domain.PublicRequest(r)
		}
	}
	return out, nil
}
func (a *App) issueLink(s *domain.State, r domain.Request) (string, string) {
	raw := token()
	t := domain.PublicToken{Hash: hash(raw), OrgID: r.OrgID, RequestID: r.ID, ExpiresAt: time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339)}
	s.Tokens[t.Hash] = t
	return a.Config.Origin + "/p/" + url.PathEscape(raw), raw
}
func (a *App) share(c *call) (any, error) {
	r, err := c.ownRequest(c.R.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if r.Status == "cancelled" {
		return nil, fail(409, "request_cancelled", "Esta solicitação foi cancelada.")
	}
	link, raw := a.issueLink(c.S, r)
	c.S.Audit(c.Org.ID, c.User.ID, "request.link_created", r.ID)
	return map[string]string{"url": link, "token": raw, "expiresAt": c.S.Tokens[hash(raw)].ExpiresAt}, nil
}
func (a *App) revoke(c *call) (any, error) {
	r, err := c.ownRequest(c.R.PathValue("id"))
	if err != nil {
		return nil, err
	}
	for k, t := range c.S.Tokens {
		if t.RequestID == r.ID {
			t.Revoked = true
			c.S.Tokens[k] = t
		}
	}
	c.S.Audit(c.Org.ID, c.User.ID, "request.links_revoked", r.ID)
	return nil, nil
}
func (a *App) cancel(c *call) (any, error) {
	r, err := c.ownRequest(c.R.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if r.Status == "completed" {
		return nil, fail(409, "already_completed", "Uma solicitação concluída não pode ser cancelada.")
	}
	r.Status = "cancelled"
	r.UpdatedAt = domain.Now()
	c.S.Requests[r.ID] = r
	for k, t := range c.S.Tokens {
		if t.RequestID == r.ID {
			t.Revoked = true
			c.S.Tokens[k] = t
		}
	}
	c.S.Audit(c.Org.ID, c.User.ID, "request.cancelled", r.ID)
	return domain.PublicRequest(r), nil
}
func (a *App) review(c *call) (any, error) {
	r, err := c.ownRequest(c.R.PathValue("requestID"))
	if err != nil {
		return nil, err
	}
	if r.Status == "cancelled" || r.Status == "completed" {
		return nil, fail(409, "request_closed", "Esta solicitação já está encerrada.")
	}
	var in struct {
		Action, Reason string
		FileID         string `json:"fileId"`
	}
	if err = decode(c.R, &in); err != nil {
		return nil, err
	}
	if in.Action != "approve" && in.Action != "correct" {
		return nil, invalid("Escolha aprovar ou solicitar correção.")
	}
	if in.Action == "correct" && (!title(in.Reason, 1000)) {
		return nil, invalid("Explique o que precisa ser corrigido.")
	}
	found := false
	for n := range r.Items {
		item := &r.Items[n]
		if item.ID != c.R.PathValue("itemID") {
			continue
		}
		found = true
		if item.Status != "uploaded" {
			return nil, fail(409, "invalid_transition", "Este documento não está aguardando revisão.")
		}
		available := false
		for _, f := range item.Files {
			if !f.Superseded && (f.ScanStatus == "clean" || f.Sample || a.Config.Env == "development" && f.ScanStatus == "unscanned_development") {
				available = true
			}
		}
		if !available {
			return nil, fail(409, "file_quarantined", "Aguarde a verificação do arquivo.")
		}
		correctionFileID := ""
		if in.Action == "correct" {
			activeFiles := []domain.DocumentFile{}
			for _, f := range item.Files {
				if !f.Superseded {
					activeFiles = append(activeFiles, f)
				}
			}
			if len(activeFiles) == 1 && in.FileID == "" {
				correctionFileID = activeFiles[0].ID
			} else {
				for _, f := range activeFiles {
					if f.ID == in.FileID {
						correctionFileID = f.ID
					}
				}
			}
			if correctionFileID == "" {
				return nil, invalid("Selecione o arquivo que precisa de correção.")
			}
		}
		item.Reviews = append(item.Reviews, domain.Review{Action: in.Action, Reason: in.Reason, UserID: c.User.ID, CreatedAt: domain.Now()})
		// A person reviewing the item settles it: drop the pending analysis
		// verdict so the panel never shows a contradicting suggestion.
		for j := range item.Files {
			if item.Files[j].Analysis != nil && item.Files[j].Analysis.Status == "queued" {
				item.Files[j].Analysis = nil
			}
		}
		if in.Action == "approve" {
			item.Status = "approved"
			item.CorrectionReason = ""
			item.CorrectionFileID = ""
		} else {
			item.Status = "correction_required"
			item.CorrectionReason = strings.TrimSpace(in.Reason)
			item.CorrectionFileID = correctionFileID
		}
	}
	if !found {
		return nil, missing()
	}
	domain.Recalculate(&r)
	r.UpdatedAt = domain.Now()
	c.S.Requests[r.ID] = r
	c.S.Audit(c.Org.ID, c.User.ID, "item."+in.Action, c.R.PathValue("itemID"))
	if in.Action == "correct" {
		_, effect := a.prepareDelivery(c.S, r, "correction", "email")
		if effect != nil {
			c.effects = append(c.effects, effect)
		}
	}
	return domain.PublicRequest(r), nil
}
func (c *call) publicToken(checkIdentity bool) (domain.PublicToken, domain.Request, error) {
	raw := c.R.PathValue("token")
	if raw == "" {
		raw = c.R.URL.Query().Get("token")
	}
	if len(raw) != 64 {
		return domain.PublicToken{}, domain.Request{}, fail(410, "link_expired", "Este link é inválido ou expirou. Peça um novo link ao escritório.")
	}
	t, ok := c.S.Tokens[hash(raw)]
	if !ok || t.Revoked || expired(t.ExpiresAt) {
		return t, domain.Request{}, fail(410, "link_expired", "Este link é inválido ou expirou. Peça um novo link ao escritório.")
	}
	r, ok := c.S.Requests[t.RequestID]
	if !ok || r.OrgID != t.OrgID || r.Status == "cancelled" {
		return t, r, fail(410, "link_revoked", "Esta solicitação não está disponível.")
	}
	verified := false
	if cookie, e := c.R.Cookie("docping_portal_" + t.Hash[:12]); e == nil && t.VerifiedSessions != nil {
		expiry, ok := t.VerifiedSessions[hash(cookie.Value)]
		verified = ok && !expired(expiry)
	}
	if checkIdentity && r.Sensitive && !verified {
		return t, r, fail(403, "identity_required", "Confirme o código enviado para o e-mail cadastrado.")
	}
	return t, r, nil
}
func (a *App) portal(c *call) (any, error) {
	_, r, err := c.publicToken(true)
	if err != nil {
		return nil, err
	}
	o := c.S.Organizations[r.OrgID]
	client := c.S.Clients[r.ClientID]
	return map[string]any{"environment": a.Config.Env, "request": domain.PortalRequest(r), "maxUploadBytes": a.Config.MaxUpload, "organization": map[string]any{"name": o.Name, "demo": o.Demo}, "client": map[string]string{"name": client.Name}}, nil
}
func (a *App) portalCode(c *call) (any, error) {
	t, r, err := c.publicToken(false)
	if err != nil {
		return nil, err
	}
	if c.S.Organizations[r.OrgID].Demo {
		return nil, fail(403, "demo_email_disabled", "Demonstração: o envio de códigos por e-mail está desativado.")
	}
	if !r.Sensitive {
		return nil, invalid("Esta solicitação não exige código.")
	}
	// A client who lost the message must be able to ask for another one without
	// waiting out the full ten minutes of validity. The first few retries are a
	// minute apart; after that the wait grows so the endpoint cannot be used to
	// flood the client's inbox.
	cooldown, message := time.Minute, "Acabamos de enviar um código. Aguarde um minuto para pedir outro."
	if t.CodeSends >= 3 {
		cooldown, message = 10*time.Minute, "Já enviamos vários códigos. Aguarde dez minutos ou fale com o escritório."
	}
	if sent, e := time.Parse(time.RFC3339, t.CodeSentAt); e == nil && time.Since(sent) < cooldown {
		return nil, fail(429, "code_recent", message)
	}
	random := make([]byte, 4)
	if _, err = rand.Read(random); err != nil {
		return nil, err
	}
	n := int(random[0])<<16 | int(random[1])<<8 | int(random[2])
	code := fmt.Sprintf("%06d", n%1000000)
	t.CodeHash = hash(code)
	t.CodeExpiresAt = time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
	t.CodeSentAt = domain.Now()
	t.CodeSends++
	t.CodeAttempts = 0
	c.S.Tokens[t.Hash] = t
	to := c.S.Clients[r.ClientID].Email
	c.effects = append(c.effects, func() error {
		if err := a.sendEmail(to, "Código de acesso DocPing", "Seu código de acesso é "+code+". Válido por 10 minutos."); err != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = a.Store.Run(ctx, true, func(s *domain.State) error {
				current := s.Tokens[t.Hash]
				if current.CodeHash == hash(code) {
					current.CodeHash = ""
					current.CodeExpiresAt = ""
					current.CodeSentAt = ""
					if current.CodeSends > 0 {
						current.CodeSends--
					}
					s.Tokens[t.Hash] = current
				}
				return nil
			})
			return fail(503, "email_unavailable", "Não foi possível enviar o código. Contate o escritório.")
		}
		return nil
	})
	return map[string]string{"message": "Código enviado ao e-mail cadastrado."}, nil
}
func (a *App) portalVerify(c *call) (any, error) {
	t, r, err := c.publicToken(false)
	if err != nil {
		return nil, err
	}
	var in struct{ Code string }
	if err = decode(c.R, &in); err != nil {
		return nil, err
	}
	if !r.Sensitive {
		return nil, nil
	}
	if expired(t.CodeExpiresAt) || t.CodeAttempts >= 5 || !rEqual(hash(in.Code), t.CodeHash) {
		t.CodeAttempts++
		c.S.Tokens[t.Hash] = t
		return map[string]any{"verified": false, "message": "Código inválido ou expirado."}, nil
	}
	if t.VerifiedSessions == nil {
		t.VerifiedSessions = map[string]string{}
	}
	for key, expiry := range t.VerifiedSessions {
		if expired(expiry) {
			delete(t.VerifiedSessions, key)
		}
	}
	raw := token()
	t.VerifiedSessions[hash(raw)] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	http.SetCookie(c.W, &http.Cookie{Name: "docping_portal_" + t.Hash[:12], Value: raw, Path: "/api", HttpOnly: true, Secure: a.Config.Env == "production", SameSite: http.SameSiteLaxMode, MaxAge: 3600})
	t.CodeHash = ""
	t.CodeSends = 0
	t.CodeSentAt = ""
	c.S.Tokens[t.Hash] = t
	return map[string]bool{"verified": true}, nil
}
func (a *App) schedules(c *call) (any, error) {
	out := []domain.Schedule{}
	now := time.Now().In(a.Config.Location)
	for _, v := range c.S.Schedules {
		if v.OrgID == c.Org.ID {
			v.NextRun = domain.NextExecution(v, now).UTC().Format(time.RFC3339)
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (c *call) validateSchedule(v domain.Schedule) error {
	if !title(v.Name, 180) || v.DayOfMonth < 1 || v.DayOfMonth > 31 || v.DueDays < 1 || v.DueDays > 90 || len(v.ClientIDs) < 1 || len(v.ClientIDs) > 100 {
		return invalid("Informe nome, clientes, dia de 1 a 31 e prazo de 1 a 90 dias.")
	}
	t, ok := c.S.Templates[v.TemplateID]
	if !ok || t.OrgID != c.Org.ID || v.Active && !t.Active {
		return invalid("Escolha um modelo ativo.")
	}
	seen := map[string]bool{}
	for _, id := range v.ClientIDs {
		client, ok := c.S.Clients[id]
		if !ok || client.OrgID != c.Org.ID || v.Active && !client.Active || seen[id] {
			return invalid("Escolha clientes ativos sem repetir.")
		}
		seen[id] = true
		if v.Active && v.Sensitive && client.Email == "" {
			return invalid("Solicitações sensíveis exigem e-mail de todos os clientes.")
		}
	}
	return nil
}
func (a *App) createSchedule(c *call) (any, error) {
	var v domain.Schedule
	if err := decode(c.R, &v); err != nil {
		return nil, err
	}
	if err := c.validateSchedule(v); err != nil {
		return nil, err
	}
	v.ID = domain.ID()
	v.OrgID = c.Org.ID
	v.CreatedAt = domain.Now()
	v.Items = append([]domain.TemplateItem{}, c.S.Templates[v.TemplateID].Items...)
	v.NextRun = domain.NextExecution(v, time.Now().In(a.Config.Location)).UTC().Format(time.RFC3339)
	c.S.Schedules[v.ID] = v
	c.S.Audit(c.Org.ID, c.User.ID, "schedule.created", v.ID)
	return v, nil
}
func (a *App) updateSchedule(c *call) (any, error) {
	old, ok := c.S.Schedules[c.R.PathValue("id")]
	if !ok || old.OrgID != c.Org.ID {
		return nil, missing()
	}
	var in struct {
		Name       string
		TemplateID string   `json:"templateId"`
		ClientIDs  []string `json:"clientIds"`
		DayOfMonth int
		DueDays    int
		Active     *bool
		Sensitive  *bool
	}
	if err := decode(c.R, &in); err != nil {
		return nil, err
	}
	if in.Name != "" {
		old.Name = in.Name
	}
	if in.TemplateID != "" && in.TemplateID != old.TemplateID {
		old.TemplateID = in.TemplateID
		old.Items = append([]domain.TemplateItem{}, c.S.Templates[in.TemplateID].Items...)
	}
	if in.ClientIDs != nil {
		old.ClientIDs = in.ClientIDs
	}
	if in.DayOfMonth != 0 {
		old.DayOfMonth = in.DayOfMonth
	}
	if in.DueDays != 0 {
		old.DueDays = in.DueDays
	}
	if in.Active != nil {
		old.Active = *in.Active
	}
	if in.Sensitive != nil {
		old.Sensitive = *in.Sensitive
	}
	if err := c.validateSchedule(old); err != nil {
		return nil, err
	}
	old.NextRun = domain.NextExecution(old, time.Now().In(a.Config.Location)).UTC().Format(time.RFC3339)
	c.S.Schedules[old.ID] = old
	c.S.Audit(c.Org.ID, c.User.ID, "schedule.updated", old.ID)
	return old, nil
}
func (a *App) scheduleRuns(c *call) (any, error) {
	schedule, ok := c.S.Schedules[c.R.PathValue("id")]
	if !ok || schedule.OrgID != c.Org.ID {
		return nil, missing()
	}
	out := []domain.ScheduleRun{}
	for _, v := range c.S.Runs {
		if v.ScheduleID == schedule.ID {
			out = append(out, v)
		}
	}
	return out, nil
}

func civilDaysBetween(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return int(time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC).Sub(time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)).Hours() / 24)
}

// activeRequests counts what the plan actually meters: a request that is still
// being collected. A completed or cancelled one keeps its documents and its
// history but no longer occupies the allowance -- otherwise the limit would
// be a lifetime total and every workspace would fill up and stop.
func activeRequests(s *domain.State, orgID string) int {
	n := 0
	for _, r := range s.Requests {
		if r.OrgID == orgID && r.Status != "completed" && r.Status != "cancelled" {
			n++
		}
	}
	return n
}

// roomForRequests refuses the whole batch when it would cross the plan's
// allowance. It refuses before creating any of them: half a batch of monthly
// requests is worse than none, because nothing says which clients were left
// out.
func (c *call) roomForRequests(wanted int) error {
	limit := c.Org.Entitlement().ActiveRequests
	if limit == 0 {
		return nil
	}
	if activeRequests(c.S, c.Org.ID)+wanted > limit {
		return fail(422, "plan_limit", "Seu plano permite "+strconv.Itoa(limit)+" solicitações ativas ao mesmo tempo. Conclua ou cancele solicitações, ou troque de plano.")
	}
	return nil
}

func countDistinct(ids []string) int {
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	return len(seen)
}
