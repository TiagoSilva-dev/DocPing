package domain

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"time"
)

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Now() string { return time.Now().UTC().Format(time.RFC3339) }

type Organization struct {
	Email         string `json:"email"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Demo          bool   `json:"demo"`
	ClientLimit   int    `json:"clientLimit"`
	Plan          string `json:"plan"`
	RetentionDays int    `json:"retentionDays"`
	// AIReview is off, suggest or auto. Auto lets a confident Jev verdict move
	// the item itself; suggest only records the verdict for a person to act on.
	AIReview         string `json:"aiReview"`
	ReminderDays     []int  `json:"reminderDays"`
	RemindersEnabled bool   `json:"remindersEnabled"`
	CreatedAt        string `json:"createdAt"`
}
type User struct {
	ID           string `json:"id"`
	OrgID        string `json:"organizationId"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	StoredHash   string `json:"passwordHash,omitempty"`
	// EmailPending gates login until the address is confirmed. The zero value
	// means "not pending", so every account stored before this field existed
	// keeps working without a data migration; only new registrations set it.
	EmailPending    bool   `json:"emailPending,omitempty"`
	EmailVerifiedAt string `json:"emailVerifiedAt,omitempty"`
	CreatedAt       string `json:"createdAt"`
}
type Session struct {
	Hash      string `json:"hash"`
	UserID    string `json:"userId"`
	CSRF      string `json:"csrf"`
	ExpiresAt string `json:"expiresAt"`
	Revoked   bool   `json:"revoked"`
}
type Client struct {
	ID          string `json:"id"`
	OrgID       string `json:"organizationId"`
	Name        string `json:"name"`
	CNPJ        string `json:"cnpj"`
	ContactName string `json:"contactName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Active      bool   `json:"active"`
	CreatedAt   string `json:"createdAt"`
}
type TemplateItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Instruction string `json:"instruction"`
	Required    bool   `json:"required"`
	MaxFiles    int    `json:"maxFiles"`
}
type Template struct {
	ID          string         `json:"id"`
	OrgID       string         `json:"organizationId"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Items       []TemplateItem `json:"items"`
	Active      bool           `json:"active"`
	CreatedAt   string         `json:"createdAt"`
}
type DocumentFile struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	MIMEType   string    `json:"mimeType"`
	Size       int64     `json:"size"`
	CreatedAt  string    `json:"createdAt"`
	ScanStatus string    `json:"scanStatus"`
	ObjectKey  string    `json:"objectKey,omitempty"`
	SHA256     string    `json:"sha256,omitempty"`
	Superseded bool      `json:"superseded"`
	Sample     bool      `json:"sample,omitempty"`
	Analysis   *Analysis `json:"analysis,omitempty"`
}

// AnalysisSignal keeps one Jev judgement verbatim. Storing the raw answers,
// not only the verdict, lets the office see why a document was approved or
// sent back, and lets a changed policy be re-applied without paying for
// inference again.
type AnalysisSignal struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Kind       string  `json:"kind"`
	Answer     string  `json:"answer,omitempty"`
	Value      float64 `json:"value"`
	Confidence float64 `json:"confidence"`
}

// Analysis is the automated conformity check of a single uploaded file:
// extraction (Docling, PaddleOCR as the OCR fallback) followed by Jev.
// Status is queued, running, completed, failed or disabled; Verdict is
// approved, review or rejected. A failed analysis never decides anything —
// the item stays waiting for a person.
type Analysis struct {
	Status      string           `json:"status"`
	Verdict     string           `json:"verdict,omitempty"`
	Confidence  float64          `json:"confidence"`
	Reason      string           `json:"reason,omitempty"`
	Signals     []AnalysisSignal `json:"signals,omitempty"`
	Extractor   string           `json:"extractor,omitempty"`
	Model       string           `json:"model,omitempty"`
	Characters  int              `json:"characters,omitempty"`
	Applied     bool             `json:"applied"`
	Error       string           `json:"error,omitempty"`
	RequestedAt string           `json:"requestedAt,omitempty"`
	CompletedAt string           `json:"completedAt,omitempty"`
}
type Review struct {
	Action string `json:"action"`
	// Source is empty for a person and "jev" when the automated analysis
	// applied the decision, so the history never attributes it to a user.
	Source    string `json:"source,omitempty"`
	Reason    string `json:"reason,omitempty"`
	UserID    string `json:"userId"`
	CreatedAt string `json:"createdAt"`
}
type RequestItem struct {
	CorrectionFileID string `json:"correctionFileId,omitempty"`
	TemplateItem
	Status           string         `json:"status"`
	CorrectionReason string         `json:"correctionReason,omitempty"`
	Files            []DocumentFile `json:"files"`
	Reviews          []Review       `json:"reviews"`
}
type Request struct {
	ID         string        `json:"id"`
	OrgID      string        `json:"organizationId"`
	ClientID   string        `json:"clientId"`
	ClientName string        `json:"clientName"`
	TemplateID string        `json:"templateId"`
	Title      string        `json:"title"`
	Competence string        `json:"competence"`
	DueDate    string        `json:"dueDate"`
	Status     string        `json:"status"`
	Items      []RequestItem `json:"items"`
	CreatedAt  string        `json:"createdAt"`
	UpdatedAt  string        `json:"updatedAt"`
	Recurring  bool          `json:"recurring"`
	ScheduleID string        `json:"scheduleId,omitempty"`
	Sensitive  bool          `json:"sensitive"`
}
type PublicToken struct {
	Hash             string            `json:"hash"`
	OrgID            string            `json:"organizationId"`
	RequestID        string            `json:"requestId"`
	ExpiresAt        string            `json:"expiresAt"`
	Revoked          bool              `json:"revoked"`
	VerifiedSessions map[string]string `json:"verifiedSessions,omitempty"`
	CodeHash         string            `json:"codeHash,omitempty"`
	CodeExpiresAt    string            `json:"codeExpiresAt,omitempty"`
	CodeSentAt       string            `json:"codeSentAt,omitempty"`
	CodeSends        int               `json:"codeSends,omitempty"`
	CodeAttempts     int               `json:"codeAttempts,omitempty"`
}
type Schedule struct {
	Items      []TemplateItem `json:"items,omitempty"`
	Sensitive  bool           `json:"sensitive"`
	ID         string         `json:"id"`
	OrgID      string         `json:"organizationId"`
	Name       string         `json:"name"`
	TemplateID string         `json:"templateId"`
	ClientIDs  []string       `json:"clientIds"`
	DayOfMonth int            `json:"dayOfMonth"`
	DueDays    int            `json:"dueDays"`
	Active     bool           `json:"active"`
	LastRun    string         `json:"lastRun,omitempty"`
	NextRun    string         `json:"nextRun,omitempty"`
	CreatedAt  string         `json:"createdAt"`
}
type ScheduleRun struct {
	ID         string `json:"id"`
	OrgID      string `json:"organizationId"`
	ScheduleID string `json:"scheduleId"`
	ClientID   string `json:"clientId"`
	Competence string `json:"competence"`
	RequestID  string `json:"requestId"`
	CreatedAt  string `json:"createdAt"`
}
type Delivery struct {
	ID        string `json:"id"`
	OrgID     string `json:"organizationId"`
	RequestID string `json:"requestId"`
	Channel   string `json:"channel"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"createdAt"`
}
type Audit struct {
	ID        string `json:"id"`
	OrgID     string `json:"organizationId"`
	UserID    string `json:"userId,omitempty"`
	Action    string `json:"action"`
	EntityID  string `json:"entityId"`
	CreatedAt string `json:"createdAt"`
}

// EmailVerification proves the registrant controls the address. It carries
// the address it was issued for: an account whose e-mail changed before the
// link is used must not be confirmed by the stale token.
type EmailVerification struct {
	Hash      string `json:"hash"`
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expiresAt"`
	Used      bool   `json:"used"`
	SentAt    string `json:"sentAt"`
}
type PasswordReset struct {
	Hash      string `json:"hash"`
	UserID    string `json:"userId"`
	ExpiresAt string `json:"expiresAt"`
	Used      bool   `json:"used"`
}
type State struct {
	Organizations map[string]Organization      `json:"organizations"`
	Users         map[string]User              `json:"users"`
	Sessions      map[string]Session           `json:"sessions"`
	Clients       map[string]Client            `json:"clients"`
	Templates     map[string]Template          `json:"templates"`
	Requests      map[string]Request           `json:"requests"`
	Tokens        map[string]PublicToken       `json:"tokens"`
	Schedules     map[string]Schedule          `json:"schedules"`
	Runs          map[string]ScheduleRun       `json:"runs"`
	Deliveries    map[string]Delivery          `json:"deliveries"`
	Audits        map[string]Audit             `json:"audits"`
	Resets        map[string]PasswordReset     `json:"resets"`
	Verifications map[string]EmailVerification `json:"verifications"`
}

func NewState() *State {
	return &State{Organizations: map[string]Organization{}, Users: map[string]User{}, Sessions: map[string]Session{}, Clients: map[string]Client{}, Templates: map[string]Template{}, Requests: map[string]Request{}, Tokens: map[string]PublicToken{}, Schedules: map[string]Schedule{}, Runs: map[string]ScheduleRun{}, Deliveries: map[string]Delivery{}, Audits: map[string]Audit{}, Resets: map[string]PasswordReset{}, Verifications: map[string]EmailVerification{}}
}

// AIReviewModes are the automated-review settings an organization may choose.
// An organization stored before the feature existed reads as suggest: an
// existing workspace never starts deciding on its own without being asked.
const (
	AIReviewOff     = "off"
	AIReviewSuggest = "suggest"
	AIReviewAuto    = "auto"
)

// NormalizeAIReview resolves the stored value, including the empty one.
func NormalizeAIReview(mode string) string {
	switch mode {
	case AIReviewOff, AIReviewSuggest, AIReviewAuto:
		return mode
	}
	return AIReviewSuggest
}
func (s *State) Audit(org, user, action, id string) {
	a := Audit{ID: ID(), OrgID: org, UserID: user, Action: action, EntityID: id, CreatedAt: Now()}
	s.Audits[a.ID] = a
}
func approvedAll(items []RequestItem) bool {
	for _, i := range items {
		if i.Status != "approved" {
			return false
		}
	}
	return true
}
func Recalculate(r *Request) {
	if r.Status == "cancelled" {
		return
	}
	required, approved, uploaded, correction, any := 0, 0, false, false, false
	for _, i := range r.Items {
		if i.Required {
			required++
			if i.Status == "approved" {
				approved++
			}
		}
		if i.Status == "uploaded" {
			uploaded = true
		}
		if i.Status == "correction_required" {
			correction = true
		}
		if i.Status != "pending" {
			any = true
		}
	}
	switch {
	case required > 0 && approved == required && !uploaded && !correction:
		r.Status = "completed"
	// A snapshot can hold only optional items, so completion also has to be
	// reachable when every item was approved and none is required.
	case required == 0 && len(r.Items) > 0 && approvedAll(r.Items):
		r.Status = "completed"
	case correction:
		r.Status = "correction_required"
	case uploaded:
		r.Status = "under_review"
	case any:
		r.Status = "in_progress"
	default:
		r.Status = "pending"
	}
}

// PortalRequest is what the client outside the office may see. On top of the
// object identity PublicRequest removes, it hides the automated analysis: the
// client reads the correction reason, never the internal signals or scores.
func PortalRequest(r Request) Request {
	r = PublicRequest(r)
	for n := range r.Items {
		for j := range r.Items[n].Files {
			r.Items[n].Files[j].Analysis = nil
		}
		for j := range r.Items[n].Reviews {
			r.Items[n].Reviews[j].UserID = ""
		}
	}
	return r
}
func PublicRequest(r Request) Request {
	r.Items = append([]RequestItem{}, r.Items...)
	for n := range r.Items {
		r.Items[n].Files = append([]DocumentFile{}, r.Items[n].Files...)
		for j := range r.Items[n].Files {
			r.Items[n].Files[j].ObjectKey = ""
			r.Items[n].Files[j].SHA256 = ""
		}
	}
	return r
}
func SortRequests(rs []Request) {
	sort.Slice(rs, func(i, j int) bool { return rs[i].UpdatedAt > rs[j].UpdatedAt })
}
func MonthDay(year int, month time.Month, day int, loc *time.Location) time.Time {
	last := time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
	if day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}
func NextExecution(s Schedule, now time.Time) time.Time {
	candidate := MonthDay(now.Year(), now.Month(), s.DayOfMonth, now.Location())
	if !candidate.After(now) {
		candidate = MonthDay(now.Year(), now.Month()+1, s.DayOfMonth, now.Location())
	}
	return candidate
}
