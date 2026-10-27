package app

import (
	"context"
	"docping/api/internal/domain"
	"testing"
)

// seedAnalysisItem puts one request with one uploaded file into the store and
// returns the job that identifies it.
func seedAnalysisItem(t *testing.T, a *App, mode string) analysisJob {
	t.Helper()
	org := domain.Organization{ID: domain.ID(), Name: "Escritório", AIReview: mode, ClientLimit: 30, RetentionDays: 365, CreatedAt: domain.Now()}
	file := domain.DocumentFile{ID: domain.ID(), Name: "das.pdf", MIMEType: "application/pdf", Size: 10, ScanStatus: "clean", CreatedAt: domain.Now()}
	item := domain.RequestItem{TemplateItem: domain.TemplateItem{ID: domain.ID(), Name: "DAS", Required: true, MaxFiles: 2}, Status: "uploaded", Files: []domain.DocumentFile{file}}
	r := domain.Request{ID: domain.ID(), OrgID: org.ID, ClientID: domain.ID(), ClientName: "Acme ME", Competence: "2026-09", DueDate: "2026-10-20", Status: "under_review", Items: []domain.RequestItem{item}, CreatedAt: domain.Now(), UpdatedAt: domain.Now()}
	err := a.Store.Run(context.Background(), true, func(s *domain.State) error {
		s.Organizations[org.ID] = org
		s.Requests[r.ID] = r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return analysisJob{OrgID: org.ID, RequestID: r.ID, ItemID: item.ID, FileID: file.ID}
}

func loadItem(t *testing.T, a *App, job analysisJob) domain.RequestItem {
	t.Helper()
	var out domain.RequestItem
	err := a.Store.Run(context.Background(), false, func(s *domain.State) error {
		out = s.Requests[job.RequestID].Items[0]
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func completed(verdict, reason string) domain.Analysis {
	return domain.Analysis{Status: "completed", Verdict: verdict, Confidence: 0.94, Reason: reason, CompletedAt: domain.Now()}
}

func TestAnalysisAutoApprovesAndCompletesTheRequest(t *testing.T) {
	a, _ := setup(t)
	job := seedAnalysisItem(t, a, domain.AIReviewAuto)
	a.saveAnalysis(context.Background(), job, completed("approved", "Tudo confere."))
	item := loadItem(t, a, job)
	if item.Status != "approved" {
		t.Fatalf("status = %q, want approved", item.Status)
	}
	if len(item.Reviews) != 1 || item.Reviews[0].Source != "jev" || item.Reviews[0].UserID != "" {
		t.Fatalf("review = %+v, must be attributed to the analysis, not a person", item.Reviews)
	}
	if item.Files[0].Analysis == nil || !item.Files[0].Analysis.Applied {
		t.Fatal("the stored analysis must record that it was applied")
	}
	var status string
	_ = a.Store.Run(context.Background(), false, func(s *domain.State) error {
		status = s.Requests[job.RequestID].Status
		return nil
	})
	if status != "completed" {
		t.Fatalf("request status = %q, want the derived completed", status)
	}
}

func TestAnalysisAutoRejectionCarriesTheReasonToTheClient(t *testing.T) {
	a, _ := setup(t)
	job := seedAnalysisItem(t, a, domain.AIReviewAuto)
	a.saveAnalysis(context.Background(), job, completed("rejected", "A competência do documento não é a solicitada."))
	item := loadItem(t, a, job)
	if item.Status != "correction_required" {
		t.Fatalf("status = %q", item.Status)
	}
	if item.CorrectionReason == "" || item.CorrectionFileID != job.FileID {
		t.Fatalf("correction = %q on file %q", item.CorrectionReason, item.CorrectionFileID)
	}
}

// Suggest mode is the whole point of having a mode: the verdict is recorded
// and nothing moves until a person acts on it.
func TestAnalysisInSuggestModeOnlyRecords(t *testing.T) {
	a, _ := setup(t)
	job := seedAnalysisItem(t, a, domain.AIReviewSuggest)
	a.saveAnalysis(context.Background(), job, completed("rejected", "Documento diferente."))
	item := loadItem(t, a, job)
	if item.Status != "uploaded" {
		t.Fatalf("status = %q, want the item untouched", item.Status)
	}
	if item.Files[0].Analysis == nil || item.Files[0].Analysis.Applied {
		t.Fatalf("analysis = %+v, want recorded but not applied", item.Files[0].Analysis)
	}
}

// An organization stored before the feature existed must not start deciding.
func TestAnalysisWithAnUnsetModeDoesNotApply(t *testing.T) {
	a, _ := setup(t)
	job := seedAnalysisItem(t, a, "")
	a.saveAnalysis(context.Background(), job, completed("approved", "Tudo confere."))
	if item := loadItem(t, a, job); item.Status != "uploaded" {
		t.Fatalf("status = %q, want the item untouched", item.Status)
	}
}

// Extraction or the model being unavailable can never approve a document.
func TestFailedAnalysisDecidesNothing(t *testing.T) {
	a, _ := setup(t)
	job := seedAnalysisItem(t, a, domain.AIReviewAuto)
	a.saveAnalysis(context.Background(), job, domain.Analysis{Status: "failed", Verdict: "approved", Error: "serviço indisponível", CompletedAt: domain.Now()})
	item := loadItem(t, a, job)
	if item.Status != "uploaded" {
		t.Fatalf("status = %q, want the item untouched", item.Status)
	}
	if item.Files[0].Analysis.Error == "" {
		t.Fatal("the failure must be visible to the office")
	}
}

// A verdict of review is the escalation path and must leave the item alone.
func TestReviewVerdictLeavesTheItemForAPerson(t *testing.T) {
	a, _ := setup(t)
	job := seedAnalysisItem(t, a, domain.AIReviewAuto)
	a.saveAnalysis(context.Background(), job, completed("review", "Revisão humana recomendada."))
	if item := loadItem(t, a, job); item.Status != "uploaded" {
		t.Fatalf("status = %q", item.Status)
	}
}

// The worker runs minutes after the upload; a person who reviewed the item in
// the meantime wins, and the late verdict is only recorded.
func TestAnalysisDoesNotOverrideAPersonWhoAlreadyReviewed(t *testing.T) {
	a, _ := setup(t)
	job := seedAnalysisItem(t, a, domain.AIReviewAuto)
	err := a.Store.Run(context.Background(), true, func(s *domain.State) error {
		r := s.Requests[job.RequestID]
		r.Items[0].Status = "approved"
		s.Requests[r.ID] = r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	a.saveAnalysis(context.Background(), job, completed("rejected", "Documento diferente."))
	item := loadItem(t, a, job)
	if item.Status != "approved" {
		t.Fatalf("status = %q, the analysis overrode a human decision", item.Status)
	}
	if item.Files[0].Analysis.Applied {
		t.Fatal("a late verdict must not claim to have been applied")
	}
}

// PortalRequest is what leaves the office: the client reads the correction
// reason, never the internal signals, scores or reviewer identity.
func TestPortalResponseHidesTheAnalysis(t *testing.T) {
	r := domain.Request{Items: []domain.RequestItem{{
		Files:   []domain.DocumentFile{{ID: "f", ObjectKey: "org/req/f", SHA256: "abc", Analysis: &domain.Analysis{Status: "completed", Verdict: "rejected", Confidence: 0.9}}},
		Reviews: []domain.Review{{Action: "correct", Reason: "Envie o documento correto.", UserID: "user-1"}},
	}}}
	out := domain.PortalRequest(r)
	file := out.Items[0].Files[0]
	if file.Analysis != nil || file.ObjectKey != "" || file.SHA256 != "" {
		t.Fatalf("portal file leaked internals: %+v", file)
	}
	if out.Items[0].Reviews[0].UserID != "" || out.Items[0].Reviews[0].Reason == "" {
		t.Fatalf("portal review = %+v, want the reason without the reviewer", out.Items[0].Reviews[0])
	}
	if r.Items[0].Files[0].Analysis == nil {
		t.Fatal("PortalRequest must not mutate the stored request")
	}
}
