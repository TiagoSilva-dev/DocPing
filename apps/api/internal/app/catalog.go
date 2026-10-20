package app

import (
	"docping/api/internal/domain"
	"encoding/csv"
	"io"
	"sort"
	"strings"
)

func (a *App) clients(c *call) (any, error) {
	out := []domain.Client{}
	for _, v := range c.S.Clients {
		if v.OrgID == c.Org.ID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (a *App) client(c *call) (any, error) {
	v, ok := c.S.Clients[c.R.PathValue("id")]
	if !ok || v.OrgID != c.Org.ID {
		return nil, missing()
	}
	return v, nil
}
func validateClient(v *domain.Client) error {
	v.Name = strings.TrimSpace(v.Name)
	v.Email = strings.TrimSpace(v.Email)
	v.CNPJ = strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(v.CNPJ))
	if !title(v.Name, 180) || len(v.ContactName) > 120 || len(v.Phone) > 30 {
		return invalid("Informe o nome da empresa e contatos válidos.")
	}
	if v.Email != "" && !validEmail(v.Email) {
		return invalid("Informe um e-mail válido.")
	}
	if v.CNPJ != "" {
		if len(v.CNPJ) != 14 {
			return invalid("O CNPJ deve ter 14 caracteres.")
		}
		for n, r := range v.CNPJ {
			if !(r >= '0' && r <= '9') && !(n < 12 && r >= 'A' && r <= 'Z') {
				return invalid("CNPJ inválido.")
			}
		}
	}
	return nil
}
func (c *call) addClient(v domain.Client) (domain.Client, error) {
	if err := validateClient(&v); err != nil {
		return v, err
	}
	active := 0
	for _, ex := range c.S.Clients {
		if ex.OrgID == c.Org.ID {
			if ex.Active {
				active++
			}
			if v.CNPJ != "" && v.CNPJ == ex.CNPJ {
				return v, fail(409, "duplicate_client", "Já existe um cliente com este CNPJ.")
			}
		}
	}
	if active >= c.Org.ClientLimit {
		return v, fail(422, "client_limit", "O limite de clientes ativos do plano foi atingido.")
	}
	v.ID = domain.ID()
	v.OrgID = c.Org.ID
	v.Active = true
	v.CreatedAt = domain.Now()
	c.S.Clients[v.ID] = v
	c.S.Audit(c.Org.ID, c.User.ID, "client.created", v.ID)
	return v, nil
}
func (a *App) createClient(c *call) (any, error) {
	var v domain.Client
	if err := decode(c.R, &v); err != nil {
		return nil, err
	}
	return c.addClient(v)
}
func (a *App) updateClient(c *call) (any, error) {
	old, ok := c.S.Clients[c.R.PathValue("id")]
	if !ok || old.OrgID != c.Org.ID {
		return nil, missing()
	}
	var in struct {
		domain.Client
		Active *bool `json:"active"`
	}
	in.Client = old
	if err := decode(c.R, &in); err != nil {
		return nil, err
	}
	// Archiving is a deliberate action, never the side effect of a payload that
	// simply left "active" out.
	v := in.Client
	v.Active = old.Active
	if in.Active != nil {
		v.Active = *in.Active
	}
	if err := validateClient(&v); err != nil {
		return nil, err
	}
	for _, other := range c.S.Clients {
		if other.ID != old.ID && other.OrgID == c.Org.ID && v.CNPJ != "" && other.CNPJ == v.CNPJ {
			return nil, fail(409, "duplicate_client", "Já existe um cliente com este CNPJ.")
		}
	}
	if v.Active && !old.Active {
		count := 0
		for _, other := range c.S.Clients {
			if other.OrgID == c.Org.ID && other.Active {
				count++
			}
		}
		if count >= c.Org.ClientLimit {
			return nil, invalid("O limite de clientes ativos do plano foi atingido.")
		}
	}
	v.ID = old.ID
	v.OrgID = old.OrgID
	v.CreatedAt = old.CreatedAt
	c.S.Clients[v.ID] = v
	c.S.Audit(c.Org.ID, c.User.ID, "client.updated", v.ID)
	return v, nil
}
func (a *App) importClients(c *call) (any, error) {
	var in struct {
		CSV    string `json:"csv"`
		Commit bool   `json:"commit"`
	}
	if err := decode(c.R, &in); err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(in.CSV, "\ufeff")))
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, invalid("CSV vazio ou inválido.")
	}
	if strings.Contains(strings.Join(header, ""), ";") {
		reader = csv.NewReader(strings.NewReader(strings.TrimPrefix(in.CSV, "\ufeff")))
		reader.Comma = ';'
		reader.TrimLeadingSpace = true
		header, err = reader.Read()
		if err != nil {
			return nil, invalid("CSV inválido.")
		}
	}
	fields := map[string]int{}
	for n, h := range header {
		fields[strings.ToLower(strings.TrimSpace(h))] = n
	}
	get := func(row []string, keys ...string) string {
		for _, k := range keys {
			if n, ok := fields[k]; ok && n < len(row) {
				return strings.TrimSpace(row[n])
			}
		}
		return ""
	}
	out := []domain.Client{}
	errs := []map[string]any{}
	seen := map[string]bool{}
	for n := 2; ; n++ {
		row, e := reader.Read()
		if e == io.EOF {
			break
		}
		if n > 1001 {
			return nil, invalid("Importe até 1.000 clientes por arquivo.")
		}
		if e != nil {
			errs = append(errs, map[string]any{"row": n, "message": "Quantidade de colunas inválida."})
			continue
		}
		v := domain.Client{Name: get(row, "name", "nome", "empresa"), CNPJ: get(row, "cnpj"), ContactName: get(row, "contactname", "responsavel", "responsável"), Email: get(row, "email", "e-mail"), Phone: get(row, "phone", "telefone", "whatsapp"), Active: true}
		e = validateClient(&v)
		if e == nil && v.CNPJ != "" {
			if seen[v.CNPJ] {
				e = invalid("CNPJ repetido no arquivo.")
			}
			for _, ex := range c.S.Clients {
				if ex.OrgID == c.Org.ID && ex.CNPJ == v.CNPJ {
					e = invalid("CNPJ já cadastrado.")
				}
			}
			seen[v.CNPJ] = true
		}
		if e != nil {
			errs = append(errs, map[string]any{"row": n, "message": e.Error()})
			continue
		}
		out = append(out, v)
	}
	active := 0
	for _, v := range c.S.Clients {
		if v.OrgID == c.Org.ID && v.Active {
			active++
		}
	}
	if active+len(out) > c.Org.ClientLimit {
		errs = append(errs, map[string]any{"row": 0, "message": "O lote excede o limite de clientes ativos do plano."})
	}
	imported := 0
	if in.Commit && len(errs) == 0 {
		for n, v := range out {
			saved, e := c.addClient(v)
			if e != nil {
				return nil, e
			}
			out[n] = saved
			imported++
		}
	}
	return map[string]any{"clients": out, "errors": errs, "imported": imported}, nil
}

// templates returns archived models too, each carrying its `active` flag.
// Filtering them out here left an archived model unreachable: it disappeared
// from the only screen that lists models, so it could never be reactivated.
func (a *App) templates(c *call) (any, error) {
	out := []domain.Template{}
	for _, v := range c.S.Templates {
		if v.OrgID == c.Org.ID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
func (a *App) template(c *call) (any, error) {
	v, ok := c.S.Templates[c.R.PathValue("id")]
	if !ok || v.OrgID != c.Org.ID {
		return nil, missing()
	}
	return v, nil
}
func validateItems(items []domain.TemplateItem) error {
	if len(items) < 1 || len(items) > 30 {
		return invalid("Adicione entre 1 e 30 documentos.")
	}
	required := false
	for _, item := range items {
		if !title(item.Name, 180) || len(item.Instruction) > 1000 || item.MaxFiles < 1 || item.MaxFiles > 20 {
			return invalid("Revise os documentos: nome e limite de 1 a 20 arquivos são obrigatórios.")
		}
		required = required || item.Required
	}
	if !required {
		return invalid("Inclua ao menos um documento obrigatório.")
	}
	return nil
}
func (a *App) createTemplate(c *call) (any, error) {
	var v domain.Template
	if err := decode(c.R, &v); err != nil {
		return nil, err
	}
	if !title(v.Name, 180) || len(v.Description) > 1000 {
		return nil, invalid("Informe um nome para o modelo.")
	}
	if err := validateItems(v.Items); err != nil {
		return nil, err
	}
	v.ID = domain.ID()
	v.OrgID = c.Org.ID
	v.Active = true
	v.CreatedAt = domain.Now()
	for n := range v.Items {
		v.Items[n].ID = domain.ID()
	}
	c.S.Templates[v.ID] = v
	c.S.Audit(c.Org.ID, c.User.ID, "template.created", v.ID)
	return v, nil
}
func (a *App) updateTemplate(c *call) (any, error) {
	old, ok := c.S.Templates[c.R.PathValue("id")]
	if !ok || old.OrgID != c.Org.ID {
		return nil, missing()
	}
	var in struct {
		Name        string
		Description *string
		Items       []domain.TemplateItem
		Active      *bool
	}
	if err := decode(c.R, &in); err != nil {
		return nil, err
	}
	if in.Name != "" {
		old.Name = in.Name
	}
	if in.Items != nil {
		if err := validateItems(in.Items); err != nil {
			return nil, err
		}
		old.Items = in.Items
		for n := range old.Items {
			old.Items[n].ID = domain.ID()
		}
	}
	if in.Description != nil {
		old.Description = *in.Description
	}
	if in.Active != nil {
		old.Active = *in.Active
	}
	if !title(old.Name, 180) || len(old.Description) > 1000 {
		return nil, invalid("Modelo inválido.")
	}
	c.S.Templates[old.ID] = old
	if !old.Active {
		for id, schedule := range c.S.Schedules {
			if schedule.OrgID == c.Org.ID && schedule.TemplateID == old.ID && schedule.Active {
				schedule.Active = false
				c.S.Schedules[id] = schedule
				c.S.Audit(c.Org.ID, c.User.ID, "schedule.paused_template_archived", id)
			}
		}
	}
	c.S.Audit(c.Org.ID, c.User.ID, "template.updated", old.ID)
	return old, nil
}
func (a *App) organization(c *call) (any, error) {
	o := c.Org
	o.AIReview = domain.NormalizeAIReview(o.AIReview)
	return o, nil
}
func (a *App) updateOrganization(c *call) (any, error) {
	var in struct {
		Name, Email      string
		RemindersEnabled bool
		ReminderDays     []int
		RetentionDays    int
		AIReview         string `json:"aiReview"`
	}
	if err := decode(c.R, &in); err != nil {
		return nil, err
	}
	if !title(in.Name, 180) || in.Email != "" && !validEmail(in.Email) || in.RetentionDays < 30 || in.RetentionDays > 3650 || len(in.ReminderDays) > 5 {
		return nil, invalid("Revise nome, e-mail e retenção de 30 a 3.650 dias.")
	}
	if in.AIReview == "" {
		in.AIReview = "suggest"
	}
	if in.AIReview != "off" && in.AIReview != "suggest" && in.AIReview != "auto" {
		return nil, invalid("A análise automática deve ser desativada, sugerida ou automática.")
	}
	sort.Ints(in.ReminderDays)
	seen := map[int]bool{}
	for _, d := range in.ReminderDays {
		if d < 1 || d > 60 || seen[d] {
			return nil, invalid("Lembretes devem ter dias únicos entre 1 e 60.")
		}
		seen[d] = true
	}
	o := c.Org
	o.Name = in.Name
	o.Email = in.Email
	o.RemindersEnabled = in.RemindersEnabled
	o.ReminderDays = in.ReminderDays
	o.RetentionDays = in.RetentionDays
	o.AIReview = in.AIReview
	c.S.Organizations[o.ID] = o
	c.S.Audit(o.ID, c.User.ID, "organization.updated", o.ID)
	return o, nil
}
func (a *App) notifications(c *call) (any, error) {
	out := []domain.Delivery{}
	for _, v := range c.S.Deliveries {
		if v.OrgID == c.Org.ID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}
func (a *App) audits(c *call) (any, error) {
	out := []domain.Audit{}
	for _, v := range c.S.Audits {
		if v.OrgID == c.Org.ID {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}
