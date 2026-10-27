// Package analysis turns an uploaded document into a conformity verdict:
// an extractor recovers the text, Jev judges it against what the office asked
// for, and code alone composes the verdict. Nothing here mutates state.
package analysis

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// Extraction is the plain text recovered from a file plus the engine that
// produced it, which the office sees next to the verdict.
type Extraction struct {
	Text   string
	Engine string
}

// Extractor recovers text from the document bytes. Two engines implement it:
// Docling, which understands PDF layout and tables, and PaddleOCR, which reads
// photographed or scanned pages where there is no text layer at all.
type Extractor interface {
	Extract(ctx context.Context, name, mimeType string, data []byte) (Extraction, error)
}

func decodeJSON(res *http.Response, out any) error {
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return errors.New("resposta do serviço de extração não pôde ser lida")
	}
	if res.StatusCode != 200 {
		detail := strings.TrimSpace(string(body))
		if len(detail) > 300 {
			detail = detail[:300]
		}
		return fmt.Errorf("serviço de extração respondeu %d: %s", res.StatusCode, detail)
	}
	if err = json.Unmarshal(body, out); err != nil {
		return errors.New("resposta do serviço de extração é inválida")
	}
	return nil
}

// Docling converts PDFs, images and office files to Markdown through a
// docling-serve instance. It is the primary engine because it preserves the
// reading order and tables that a fiscal document depends on.
type Docling struct {
	BaseURL string
	APIKey  string
	Lang    string
	HTTP    *http.Client
}

type doclingResponse struct {
	Document struct {
		MDContent   string `json:"md_content"`
		TextContent string `json:"text_content"`
	} `json:"document"`
	Status string `json:"status"`
	Errors []any  `json:"errors"`
}

func (d *Docling) Extract(ctx context.Context, name, mimeType string, data []byte) (Extraction, error) {
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	part, err := form.CreateFormFile("files", name)
	if err != nil {
		return Extraction{}, err
	}
	if _, err = part.Write(data); err != nil {
		return Extraction{}, err
	}
	// Markdown keeps the table structure a DAS or a folha de pagamento relies
	// on; OCR stays on so a scanned PDF without a text layer still resolves.
	fields := [][2]string{{"to_formats", "md"}, {"do_ocr", "true"}, {"image_export_mode", "placeholder"}, {"abort_on_error", "false"}}
	if d.Lang != "" {
		fields = append(fields, [2]string{"ocr_lang", d.Lang})
	}
	for _, f := range fields {
		if err = form.WriteField(f[0], f[1]); err != nil {
			return Extraction{}, err
		}
	}
	if err = form.Close(); err != nil {
		return Extraction{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(d.BaseURL, "/")+"/v1/convert/file", body)
	if err != nil {
		return Extraction{}, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	if d.APIKey != "" {
		req.Header.Set("X-Api-Key", d.APIKey)
	}
	res, err := client(d.HTTP).Do(req)
	if err != nil {
		return Extraction{}, errors.New("serviço de extração indisponível")
	}
	var parsed doclingResponse
	if err = decodeJSON(res, &parsed); err != nil {
		return Extraction{}, err
	}
	if parsed.Status == "failure" {
		return Extraction{}, errors.New("o serviço de extração não conseguiu ler o arquivo")
	}
	text := parsed.Document.MDContent
	if strings.TrimSpace(text) == "" {
		text = parsed.Document.TextContent
	}
	return Extraction{Text: text, Engine: "docling"}, nil
}

// Paddle reads a PaddleOCR (PaddleX) serving instance. It is the fallback for
// the photo-of-a-document case the portal receives from phones, where Docling
// returns a page with no recoverable text.
type Paddle struct {
	BaseURL string
	HTTP    *http.Client
}

type paddleResponse struct {
	ErrorCode int    `json:"errorCode"`
	ErrorMsg  string `json:"errorMsg"`
	Result    struct {
		OCRResults []struct {
			PrunedResult struct {
				RecTexts  []string  `json:"rec_texts"`
				RecScores []float64 `json:"rec_scores"`
			} `json:"prunedResult"`
		} `json:"ocrResults"`
	} `json:"result"`
}

func (p *Paddle) Extract(ctx context.Context, name, mimeType string, data []byte) (Extraction, error) {
	// fileType 0 is a PDF and 1 is an image; PaddleOCR cannot infer it from a
	// base64 payload the way it does from a URL.
	fileType := 1
	if mimeType == "application/pdf" {
		fileType = 0
	}
	payload, err := json.Marshal(map[string]any{"file": base64.StdEncoding.EncodeToString(data), "fileType": fileType})
	if err != nil {
		return Extraction{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(p.BaseURL, "/")+"/ocr", bytes.NewReader(payload))
	if err != nil {
		return Extraction{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := client(p.HTTP).Do(req)
	if err != nil {
		return Extraction{}, errors.New("serviço de OCR indisponível")
	}
	var parsed paddleResponse
	if err = decodeJSON(res, &parsed); err != nil {
		return Extraction{}, err
	}
	if parsed.ErrorCode != 0 {
		return Extraction{}, errors.New("o serviço de OCR não conseguiu ler o arquivo")
	}
	lines := []string{}
	for _, page := range parsed.Result.OCRResults {
		lines = append(lines, page.PrunedResult.RecTexts...)
	}
	return Extraction{Text: strings.Join(lines, "\n"), Engine: "paddleocr"}, nil
}

// Chain runs Primary and falls back to Fallback when the primary fails or
// recovers less than MinChars of text. A scanned page that yields three
// characters is indistinguishable from a failure as far as judging it goes.
type Chain struct {
	Primary  Extractor
	Fallback Extractor
	MinChars int
}

func (c *Chain) Extract(ctx context.Context, name, mimeType string, data []byte) (Extraction, error) {
	out, err := Extraction{}, error(nil)
	if c.Primary != nil {
		out, err = c.Primary.Extract(ctx, name, mimeType, data)
		if err == nil && len([]rune(strings.TrimSpace(out.Text))) >= c.MinChars {
			return out, nil
		}
	}
	if c.Fallback == nil {
		if err != nil {
			return Extraction{}, err
		}
		return out, nil
	}
	second, secondErr := c.Fallback.Extract(ctx, name, mimeType, data)
	if secondErr != nil {
		if err != nil {
			return Extraction{}, err
		}
		return out, nil
	}
	if len([]rune(strings.TrimSpace(second.Text))) <= len([]rune(strings.TrimSpace(out.Text))) && err == nil {
		return out, nil
	}
	return second, nil
}

func client(h *http.Client) *http.Client {
	if h != nil {
		return h
	}
	return http.DefaultClient
}
