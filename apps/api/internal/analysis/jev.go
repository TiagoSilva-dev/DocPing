package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Question is one judgement sent to the System One endpoint. Type is noul,
// choice or score; Criteria is the object or ordered array that type expects.
type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// Answer is the typed judgement that comes back. Noul carries the probability
// of yes and has no confidence of its own; choice and score report both the
// answer and how concentrated the distribution behind it is.
type Answer struct {
	Type          string             `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// Certainty normalises the three primitives onto one 0..1 scale so a single
// threshold can gate them. A noul of 0.5 means genuinely unsure, which maps
// to 0, the same reading a flat choice distribution gets.
func (a Answer) Certainty() float64 {
	if a.Type == "noul" {
		c := 2*a.Noul - 1
		if c < 0 {
			c = -c
		}
		return c
	}
	if a.Confidence == nil {
		return 0
	}
	return *a.Confidence
}

// Jev talks to the TypeSafe System One endpoint. One request carries every
// question about the same document: they are independent, so they run in
// parallel and cost a single round trip.
type Jev struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

type jevResponse struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Detail []struct {
		Msg string `json:"msg"`
	} `json:"detail"`
}

// Ask sends the state and every question in one call and returns the answers
// keyed by question name. Question names are for this code only; the model
// never sees them, so each question carries its full meaning.
func (j *Jev) Ask(ctx context.Context, state any, questions map[string]Question) (string, map[string]Answer, error) {
	if j.APIKey == "" {
		return "", nil, errors.New("TYPESAFE_API_KEY não configurada")
	}
	model := j.Model
	if model == "" {
		model = "jev-latest"
	}
	payload, err := json.Marshal(map[string]any{"model": model, "state": state, "questions": questions})
	if err != nil {
		return "", nil, err
	}
	base := j.BaseURL
	if base == "" {
		base = "https://api.typesafe.ai"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(base, "/")+"/v1/systemone", bytes.NewReader(payload))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+j.APIKey)
	res, err := client(j.HTTP).Do(req)
	if err != nil {
		return "", nil, errors.New("serviço de análise indisponível")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return "", nil, errors.New("resposta da análise não pôde ser lida")
	}
	var parsed jevResponse
	if json.Unmarshal(body, &parsed) != nil {
		return "", nil, errors.New("resposta da análise é inválida")
	}
	if res.StatusCode != 200 {
		// A 422 lists the offending fields; anything else is reported without
		// provider detail so a key or an endpoint never leaks into the UI.
		if res.StatusCode == 422 && len(parsed.Detail) > 0 {
			return "", nil, fmt.Errorf("análise recusou a solicitação: %s", parsed.Detail[0].Msg)
		}
		return "", nil, fmt.Errorf("serviço de análise respondeu %d", res.StatusCode)
	}
	if len(parsed.Answers) == 0 {
		return "", nil, errors.New("a análise não retornou respostas")
	}
	return parsed.Model, parsed.Answers, nil
}
