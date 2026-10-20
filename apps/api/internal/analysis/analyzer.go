package analysis

import (
	"context"
	"docping/api/internal/domain"
	"strings"
)

// Input is one file to analyse together with the request it answers.
type Input struct {
	DocumentName string
	Guidance     string
	Competence   string
	DueDate      string
	ClientName   string
	FileName     string
	MIMEType     string
	Data         []byte
}

// Analyzer runs the two stages in order: an extractor recovers the text, Jev
// judges it, and Decide composes the verdict. It never touches persistence —
// the caller writes the result inside its own transaction.
type Analyzer struct {
	Extractor Extractor
	Jev       *Jev
	Policy    Policy
	MaxChars  int
}

// minChars is the amount of text below which a document is treated as not
// extracted at all. A fiscal document always carries more than a few words,
// so anything shorter means the engine found no text layer to read.
const minChars = 24

// Analyze returns a completed or failed analysis. It returns an error only
// for the caller to log: the returned Analysis already carries the failure,
// and a failed analysis decides nothing, leaving the item for a person.
func (a *Analyzer) Analyze(ctx context.Context, in Input) (domain.Analysis, error) {
	result := domain.Analysis{Status: "running", RequestedAt: domain.Now()}
	if a.Extractor == nil || a.Jev == nil {
		result.Status, result.Error, result.CompletedAt = "disabled", "Análise automática não está configurada.", domain.Now()
		return result, nil
	}
	extracted, err := a.Extractor.Extract(ctx, in.FileName, in.MIMEType, in.Data)
	if err != nil {
		result.Status, result.Error, result.CompletedAt = "failed", err.Error(), domain.Now()
		return result, err
	}
	result.Extractor = extracted.Engine
	text := strings.TrimSpace(extracted.Text)
	result.Characters = len([]rune(text))
	if result.Characters < minChars {
		// No readable text is itself a finding, not a service failure: the
		// client sent a blank or unreadable page and has to send another.
		result.Status, result.Verdict, result.Confidence = "completed", "rejected", 1
		result.Reason, result.CompletedAt = problemLabels["ilegivel"], domain.Now()
		return result, nil
	}
	maxChars := a.MaxChars
	if maxChars <= 0 {
		maxChars = 24000
	}
	subject := NewSubject(in.DocumentName, in.Guidance, in.Competence, in.DueDate, in.ClientName, in.FileName, in.MIMEType, text, maxChars)
	model, answers, err := a.Jev.Ask(ctx, subject, Questions())
	if err != nil {
		result.Status, result.Error, result.CompletedAt = "failed", err.Error(), domain.Now()
		return result, err
	}
	outcome := Decide(answers, a.Policy)
	result.Status, result.Model = "completed", model
	result.Verdict, result.Confidence, result.Reason, result.Signals = outcome.Verdict, outcome.Confidence, outcome.Reason, outcome.Signals
	result.CompletedAt = domain.Now()
	return result, nil
}
