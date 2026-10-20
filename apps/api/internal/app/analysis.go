package app

import (
	"context"
	"docping/api/internal/analysis"
	"docping/api/internal/domain"
	"io"
	"log"
	"net/http"
	"time"
)

// analysisJob identifies the file to analyse. It carries identifiers only:
// the worker re-reads the state when it runs, so a document reviewed,
// replaced or cancelled while it waited is never decided on stale data.
type analysisJob struct{ OrgID, RequestID, ItemID, FileID string }

// analysisWorkers bounds how many documents are extracted and judged at once.
// Extraction is the slow, memory-hungry half, and the services behind it are
// a single container in the default deployment.
const analysisWorkers = 2

// StartAnalysis builds the analyzer from configuration and starts the queue.
// It returns once the queue exists, so a request served immediately after
// startup can already enqueue. With AI_REVIEW_ENABLED unset it does nothing
// and every upload keeps going to a person, which is the safe default.
func (a *App) StartAnalysis(ctx context.Context) {
	if !a.Config.AIReview {
		return
	}
	var docling, paddle analysis.Extractor
	if a.Config.DoclingURL != "" {
		docling = &analysis.Docling{BaseURL: a.Config.DoclingURL, APIKey: a.Config.DoclingKey, Lang: a.Config.DoclingLang, HTTP: &http.Client{Timeout: 4 * time.Minute}}
	}
	if a.Config.PaddleURL != "" {
		paddle = &analysis.Paddle{BaseURL: a.Config.PaddleURL, HTTP: &http.Client{Timeout: 4 * time.Minute}}
	}
	policy := analysis.DefaultPolicy()
	policy.ApproveSignal, policy.RejectIssue = a.Config.AIApprove, a.Config.AIReject
	a.Analyzer = &analysis.Analyzer{
		// Docling reads layout and tables; PaddleOCR takes over when a page
		// carries no text layer, which is the photo-from-a-phone case the
		// client portal receives most.
		Extractor: &analysis.Chain{Primary: docling, Fallback: paddle, MinChars: 240},
		Jev:       &analysis.Jev{BaseURL: a.Config.JevURL, APIKey: a.Config.JevKey, Model: a.Config.JevModel, HTTP: &http.Client{Timeout: 2 * time.Minute}},
		Policy:    policy,
		MaxChars:  a.Config.AIMaxChars,
	}
	a.analysisJobs = make(chan analysisJob, 256)
	for n := 0; n < analysisWorkers; n++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-a.analysisJobs:
					a.runAnalysis(ctx, job)
				}
			}
		}()
	}
}

// enqueueAnalysis never blocks the request that produced the upload. A full
// queue drops the job rather than holding the portal's connection open; the
// office can always ask for the analysis again from the request screen.
func (a *App) enqueueAnalysis(job analysisJob) {
	if a.analysisJobs == nil {
		return
	}
	select {
	case a.analysisJobs <- job:
	default:
		log.Print("analysis queue full, job dropped")
	}
}

// analysisTarget is the snapshot the worker reads before doing any I/O.
type analysisTarget struct {
	mode    string
	file    domain.DocumentFile
	input   analysis.Input
	present bool
}

func (a *App) runAnalysis(ctx context.Context, job analysisJob) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	var target analysisTarget
	err := a.Store.Run(ctx, false, func(s *domain.State) error {
		target = a.analysisTarget(s, job)
		return nil
	})
	if err != nil || !target.present {
		return
	}
	data, err := a.analysisBytes(ctx, target.file)
	if err != nil {
		a.saveAnalysis(ctx, job, domain.Analysis{Status: "failed", Error: "Não foi possível ler o arquivo armazenado.", RequestedAt: domain.Now(), CompletedAt: domain.Now()})
		return
	}
	target.input.Data = data
	result, err := a.Analyzer.Analyze(ctx, target.input)
	if err != nil {
		log.Print("analysis failed: ", err)
	}
	a.saveAnalysis(ctx, job, result)
}

// analysisTarget resolves the job against current state and decides whether
// it is still worth running: the organization must want it, the item must
// still be awaiting review, and the file must have cleared the scanner.
func (a *App) analysisTarget(s *domain.State, job analysisJob) analysisTarget {
	out := analysisTarget{}
	r, ok := s.Requests[job.RequestID]
	if !ok || r.OrgID != job.OrgID || r.Status == "cancelled" || r.Status == "completed" {
		return out
	}
	org := s.Organizations[r.OrgID]
	out.mode = domain.NormalizeAIReview(org.AIReview)
	if out.mode == domain.AIReviewOff {
		return out
	}
	for _, item := range r.Items {
		if item.ID != job.ItemID || item.Status != "uploaded" {
			continue
		}
		for _, f := range item.Files {
			if f.ID != job.FileID || f.Superseded || !a.readableFile(f) {
				continue
			}
			out.file, out.present = f, true
			out.input = analysis.Input{DocumentName: item.Name, Guidance: item.Instruction, Competence: r.Competence, DueDate: r.DueDate, ClientName: r.ClientName, FileName: f.Name, MIMEType: f.MIMEType}
		}
	}
	return out
}

func (a *App) analysisBytes(ctx context.Context, f domain.DocumentFile) ([]byte, error) {
	reader, err := a.Objects.Get(ctx, f.ObjectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, a.Config.MaxUpload+1))
}

// saveAnalysis stores the verdict and, in auto mode, applies it. Everything
// is re-checked inside the transaction because minutes may have passed: a
// person may have reviewed the item already, or the client may have replaced
// the file, and the analysis must not overwrite either.
func (a *App) saveAnalysis(ctx context.Context, job analysisJob, result domain.Analysis) {
	effects := []func() error{}
	err := a.Store.Run(ctx, true, func(s *domain.State) error {
		r, ok := s.Requests[job.RequestID]
		if !ok || r.OrgID != job.OrgID {
			return nil
		}
		mode := domain.NormalizeAIReview(s.Organizations[r.OrgID].AIReview)
		for n := range r.Items {
			item := &r.Items[n]
			if item.ID != job.ItemID {
				continue
			}
			stored := false
			for j := range item.Files {
				if item.Files[j].ID == job.FileID {
					item.Files[j].Analysis = &result
					stored = true
				}
			}
			if !stored {
				return nil
			}
			// Only a confident verdict on an item still waiting for review may
			// act, and only when the organization asked for automatic decisions.
			if mode != domain.AIReviewAuto || item.Status != "uploaded" || result.Status != "completed" || r.Status == "cancelled" || r.Status == "completed" {
				break
			}
			switch result.Verdict {
			case "approved":
				item.Status, item.CorrectionReason, item.CorrectionFileID = "approved", "", ""
				item.Reviews = append(item.Reviews, domain.Review{Action: "approve", Source: "jev", CreatedAt: domain.Now()})
			case "rejected":
				item.Status, item.CorrectionReason, item.CorrectionFileID = "correction_required", result.Reason, job.FileID
				item.Reviews = append(item.Reviews, domain.Review{Action: "correct", Reason: result.Reason, Source: "jev", CreatedAt: domain.Now()})
			default:
				break
			}
			if result.Verdict == "approved" || result.Verdict == "rejected" {
				result.Applied = true
				for j := range item.Files {
					if item.Files[j].ID == job.FileID {
						item.Files[j].Analysis = &result
					}
				}
				s.Audit(r.OrgID, "", "item."+map[string]string{"approved": "approve", "rejected": "correct"}[result.Verdict], item.ID)
			}
		}
		domain.Recalculate(&r)
		r.UpdatedAt = domain.Now()
		s.Requests[r.ID] = r
		s.Audit(r.OrgID, "", "analysis."+result.Status, job.FileID)
		if result.Applied && result.Verdict == "rejected" {
			if _, effect := a.prepareDelivery(s, r, "correction", "email"); effect != nil {
				effects = append(effects, effect)
			}
		}
		return nil
	})
	if err != nil {
		log.Print("analysis result not stored: ", err)
		return
	}
	for _, effect := range effects {
		_ = effect()
	}
}

// analyze re-runs the analysis for an item on request from the office, after
// a failure or a policy change. The job is enqueued after the commit so the
// queued status is already visible when the worker picks it up.
func (a *App) analyze(c *call) (any, error) {
	if !a.Config.AIReview {
		return nil, fail(409, "analysis_disabled", "A análise automática não está habilitada neste servidor.")
	}
	r, err := c.ownRequest(c.R.PathValue("requestID"))
	if err != nil {
		return nil, err
	}
	if domain.NormalizeAIReview(c.Org.AIReview) == domain.AIReviewOff {
		return nil, fail(409, "analysis_disabled", "A análise automática está desativada para esta organização.")
	}
	target := ""
	for n := range r.Items {
		item := &r.Items[n]
		if item.ID != c.R.PathValue("itemID") {
			continue
		}
		if item.Status != "uploaded" {
			return nil, fail(409, "invalid_transition", "Este documento não está aguardando revisão.")
		}
		for j := range item.Files {
			if !item.Files[j].Superseded && a.readableFile(item.Files[j]) {
				target = item.Files[j].ID
				item.Files[j].Analysis = &domain.Analysis{Status: "queued", RequestedAt: domain.Now()}
			}
		}
	}
	if target == "" {
		return nil, fail(409, "file_quarantined", "Não há arquivo disponível para analisar.")
	}
	r.UpdatedAt = domain.Now()
	c.S.Requests[r.ID] = r
	c.S.Audit(c.Org.ID, c.User.ID, "analysis.requested", target)
	job := analysisJob{OrgID: r.OrgID, RequestID: r.ID, ItemID: c.R.PathValue("itemID"), FileID: target}
	c.effects = append(c.effects, func() error { a.enqueueAnalysis(job); return nil })
	return domain.PublicRequest(r), nil
}
