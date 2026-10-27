package analysis

import (
	"context"
	"encoding/json"
	"io"
	stdmime "mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func conf(v float64) *float64 { return &v }

// clean is the set of answers for a document that matches in every respect.
func clean() map[string]Answer {
	return map[string]Answer{
		"tipo_correto": {Type: "noul", Noul: 0.97},
		"integridade":  {Type: "noul", Noul: 0.95},
		"legibilidade": {Type: "score", Score: 2.8, Confidence: conf(0.9)},
		"competencia":  {Type: "choice", Choice: "correta", Confidence: conf(0.9), Probabilities: map[string]float64{"correta": 0.93}},
		"titular":      {Type: "choice", Choice: "confere", Confidence: conf(0.88), Probabilities: map[string]float64{"confere": 0.9}},
		"problema":     {Type: "choice", Choice: "nenhum", Confidence: conf(0.91), Probabilities: map[string]float64{"nenhum": 0.94}},
	}
}

func TestDecideApprovesWhenEverySignalAgrees(t *testing.T) {
	out := Decide(clean(), DefaultPolicy())
	if out.Verdict != "approved" {
		t.Fatalf("verdict = %q, reason %q", out.Verdict, out.Reason)
	}
	if out.Confidence < 0.8 {
		t.Fatalf("confidence = %v, want the weakest signal", out.Confidence)
	}
	if len(out.Signals) != 6 {
		t.Fatalf("signals = %d, want 6", len(out.Signals))
	}
	if out.Signals[0].Key != "tipo_correto" || out.Signals[5].Key != "problema" {
		t.Fatalf("signal order = %q..%q", out.Signals[0].Key, out.Signals[5].Key)
	}
}

func TestDecideRejectsOnAConfidentProblem(t *testing.T) {
	answers := clean()
	answers["problema"] = Answer{Type: "choice", Choice: "competencia_errada", Confidence: conf(0.88), Probabilities: map[string]float64{"competencia_errada": 0.9}}
	out := Decide(answers, DefaultPolicy())
	if out.Verdict != "rejected" {
		t.Fatalf("verdict = %q", out.Verdict)
	}
	if !strings.Contains(out.Reason, "competência") {
		t.Fatalf("reason = %q, want the client-facing sentence", out.Reason)
	}
}

// A problem the model is unsure about must reach a person, never the client.
func TestDecideSendsAnUnsureProblemToReview(t *testing.T) {
	answers := clean()
	answers["problema"] = Answer{Type: "choice", Choice: "incompleto", Confidence: conf(0.41), Probabilities: map[string]float64{"incompleto": 0.5}}
	out := Decide(answers, DefaultPolicy())
	if out.Verdict != "review" {
		t.Fatalf("verdict = %q, want review", out.Verdict)
	}
}

func TestDecideRejectsAnotherDocument(t *testing.T) {
	answers := clean()
	answers["tipo_correto"] = Answer{Type: "noul", Noul: 0.04}
	if out := Decide(answers, DefaultPolicy()); out.Verdict != "rejected" {
		t.Fatalf("verdict = %q", out.Verdict)
	}
}

// A noul sitting on the fence is the model saying it does not know, which is
// the one case that must never become an automatic decision.
func TestDecideRoutesUndecidedSignalsToReview(t *testing.T) {
	answers := clean()
	answers["tipo_correto"] = Answer{Type: "noul", Noul: 0.52}
	out := Decide(answers, DefaultPolicy())
	if out.Verdict != "review" {
		t.Fatalf("verdict = %q, want review", out.Verdict)
	}
	if !strings.Contains(out.Reason, "tipo do documento") {
		t.Fatalf("reason = %q", out.Reason)
	}
}

func TestDecideWithoutEveryAnswerStaysOnReview(t *testing.T) {
	answers := clean()
	delete(answers, "titular")
	if out := Decide(answers, DefaultPolicy()); out.Verdict != "review" {
		t.Fatalf("verdict = %q, want review", out.Verdict)
	}
}

// Missing competência or titular is normal for many valid documents and must
// not block an otherwise clean approval.
func TestDecideApprovesWhenCompetenceIsAbsent(t *testing.T) {
	answers := clean()
	answers["competencia"] = Answer{Type: "choice", Choice: "ausente", Confidence: conf(0.84), Probabilities: map[string]float64{"ausente": 0.87}}
	if out := Decide(answers, DefaultPolicy()); out.Verdict != "approved" {
		t.Fatalf("verdict = %q, reason %q", out.Verdict, out.Reason)
	}
}

func TestCertaintyNormalisesThePrimitives(t *testing.T) {
	if got := (Answer{Type: "noul", Noul: 0.5}).Certainty(); got != 0 {
		t.Fatalf("noul 0.5 certainty = %v, want 0", got)
	}
	if got := (Answer{Type: "noul", Noul: 0.05}).Certainty(); got < 0.89 {
		t.Fatalf("confident no certainty = %v", got)
	}
	if got := (Answer{Type: "choice", Confidence: nil}).Certainty(); got != 0 {
		t.Fatalf("missing confidence = %v, want 0", got)
	}
}

func TestTruncateKeepsBothEndsAndMarksTheCut(t *testing.T) {
	text := strings.Repeat("a", 500) + strings.Repeat("z", 500)
	out := Truncate(text, 100)
	if !strings.HasPrefix(out, "aaa") || !strings.HasSuffix(out, "zzz") {
		t.Fatalf("truncation dropped an end: %q", out[:20])
	}
	if !strings.Contains(out, "omitido") {
		t.Fatal("truncation is not marked, the model would read a cut document as complete")
	}
	if Truncate("curto", 100) != "curto" {
		t.Fatal("short text must pass through untouched")
	}
}

func TestDoclingExtractsMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/convert/file" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, params, err := mime(r)
		if err != nil {
			t.Fatal(err)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		form, err := reader.ReadForm(1 << 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(form.File["files"]) != 1 {
			t.Errorf("files part missing: %v", form.File)
		}
		if form.Value["to_formats"][0] != "md" || form.Value["do_ocr"][0] != "true" {
			t.Errorf("options = %v", form.Value)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "document": map[string]string{"md_content": "# DAS\n\nCompetência 09/2026"}})
	}))
	defer server.Close()
	out, err := (&Docling{BaseURL: server.URL, Lang: "pt"}).Extract(context.Background(), "das.pdf", "application/pdf", []byte("%PDF-1.4"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Engine != "docling" || !strings.Contains(out.Text, "Competência") {
		t.Fatalf("extraction = %+v", out)
	}
}

func TestPaddleJoinsRecognisedLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			File     string `json:"file"`
			FileType int    `json:"fileType"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.FileType != 1 {
			t.Errorf("fileType = %d, want 1 for an image", body.FileType)
		}
		if body.File == "" {
			t.Error("file must be base64 encoded")
		}
		_, _ = io.WriteString(w, `{"errorCode":0,"result":{"ocrResults":[{"prunedResult":{"rec_texts":["NOTA FISCAL","Valor 1.200,00"]}}]}}`)
	}))
	defer server.Close()
	out, err := (&Paddle{BaseURL: server.URL}).Extract(context.Background(), "foto.jpg", "image/jpeg", []byte{0xff, 0xd8})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "NOTA FISCAL\nValor 1.200,00" || out.Engine != "paddleocr" {
		t.Fatalf("extraction = %+v", out)
	}
}

// The chain exists for the photographed page: Docling returns a handful of
// characters and OCR has to take over.
func TestChainFallsBackWhenTheTextLayerIsEmpty(t *testing.T) {
	chain := &Chain{Primary: stub{text: "   ", engine: "docling"}, Fallback: stub{text: "conteúdo recuperado por OCR", engine: "paddleocr"}, MinChars: 240}
	out, err := chain.Extract(context.Background(), "foto.jpg", "image/jpeg", nil)
	if err != nil || out.Engine != "paddleocr" {
		t.Fatalf("extraction = %+v, err %v", out, err)
	}
}

func TestChainKeepsThePrimaryWhenItReadsEnough(t *testing.T) {
	chain := &Chain{Primary: stub{text: strings.Repeat("texto ", 100), engine: "docling"}, Fallback: stub{text: "ocr", engine: "paddleocr"}, MinChars: 240}
	out, _ := chain.Extract(context.Background(), "das.pdf", "application/pdf", nil)
	if out.Engine != "docling" {
		t.Fatalf("engine = %q", out.Engine)
	}
}

func TestJevSendsOneRequestForEveryQuestion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body struct {
			Model     string              `json:"model"`
			Questions map[string]Question `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Questions) != 6 || body.Model != "jev-latest" {
			t.Errorf("payload = %d questions, model %q", len(body.Questions), body.Model)
		}
		_, _ = io.WriteString(w, `{"model":"jev-1.13","answers":{"tipo_correto":{"type":"noul","noul":0.9}}}`)
	}))
	defer server.Close()
	model, answers, err := (&Jev{BaseURL: server.URL, APIKey: "k"}).Ask(context.Background(), Subject{}, Questions())
	if err != nil {
		t.Fatal(err)
	}
	if model != "jev-1.13" || answers["tipo_correto"].Noul != 0.9 {
		t.Fatalf("model %q answers %+v", model, answers)
	}
}

func TestJevWithoutAKeyFailsBeforeAnyRequest(t *testing.T) {
	if _, _, err := (&Jev{}).Ask(context.Background(), nil, nil); err == nil {
		t.Fatal("a missing key must fail closed")
	}
}

type stub struct{ text, engine string }

func (s stub) Extract(context.Context, string, string, []byte) (Extraction, error) {
	return Extraction{Text: s.text, Engine: s.engine}, nil
}

func mime(r *http.Request) (string, map[string]string, error) {
	return stdmime.ParseMediaType(r.Header.Get("Content-Type"))
}
