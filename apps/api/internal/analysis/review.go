package analysis

import (
	"docping/api/internal/domain"
	"sort"
	"strings"
)

// The question text is Portuguese on purpose. The documents are Brazilian
// fiscal and payroll papers, the state field names are the ones the office
// already uses, and the problem labels become the correction reason the
// client reads in the portal. Everything else in this package stays English.

// Subject is the state every question refers to. Field names are the
// backticked paths the instructions use, so the model and the code name the
// same things.
type Subject struct {
	Solicitacao struct {
		Documento   string `json:"documento"`
		Orientacao  string `json:"orientacao,omitempty"`
		Competencia string `json:"competencia"`
		Vencimento  string `json:"vencimento"`
		Cliente     string `json:"cliente"`
	} `json:"solicitacao"`
	Arquivo struct {
		Nome string `json:"nome"`
		Tipo string `json:"tipo"`
	} `json:"arquivo"`
	Documento struct {
		Texto string `json:"texto"`
	} `json:"documento"`
}

// NewSubject assembles the state from what the office asked for and what the
// extractor recovered. Text longer than maxChars is cut: the identifying
// header and footer of a fiscal document are what the questions need, and the
// middle of a 200-page ledger only costs input tokens.
func NewSubject(documentName, guidance, competence, dueDate, clientName, fileName, mimeType, text string, maxChars int) Subject {
	var s Subject
	s.Solicitacao.Documento = documentName
	s.Solicitacao.Orientacao = guidance
	s.Solicitacao.Competencia = competence
	s.Solicitacao.Vencimento = dueDate
	s.Solicitacao.Cliente = clientName
	s.Arquivo.Nome = fileName
	s.Arquivo.Tipo = mimeType
	s.Documento.Texto = Truncate(text, maxChars)
	return s
}

// Truncate keeps the head and the tail of the text around an explicit marker,
// so the model is never led to believe a cut document ended where it stops.
func Truncate(text string, maxChars int) string {
	runes := []rune(text)
	if maxChars <= 0 || len(runes) <= maxChars {
		return text
	}
	head := maxChars * 2 / 3
	tail := maxChars - head
	return string(runes[:head]) + "\n\n[...trecho omitido por tamanho...]\n\n" + string(runes[len(runes)-tail:])
}

// problemLabels are the blocking problems the model may report. The key is
// what Jev answers; the value is the sentence the client reads when the
// analysis sends the document back.
var problemLabels = map[string]string{
	"nenhum":              "",
	"ilegivel":            "O arquivo está ilegível: parte do conteúdo não pôde ser lida. Envie novamente com melhor qualidade ou em PDF.",
	"documento_diferente": "O arquivo enviado não corresponde ao documento solicitado. Confira e envie o documento correto.",
	"competencia_errada":  "A competência do documento não é a solicitada. Envie o documento referente ao período pedido.",
	"titular_diferente":   "O documento está em nome de outra pessoa ou empresa. Envie o documento do titular correto.",
	"incompleto":          "O documento está incompleto: faltam páginas ou há conteúdo cortado. Envie o arquivo completo.",
	"indicio_adulteracao": "O documento apresenta sinais de edição ou rasura. Envie o arquivo original emitido.",
	"vencido":             "O documento está vencido ou fora do prazo de validade. Envie uma via vigente.",
}

// Questions is the judgement set, asked in a single request. They are
// independent views of the same document, so none of them can see another's
// answer and all of them run in parallel.
func Questions() map[string]Question {
	return map[string]Question{
		"tipo_correto": {
			Type:         "noul",
			Instructions: "O conteúdo de `documento.texto` é mesmo o documento descrito em `solicitacao.documento`, conforme a orientação em `solicitacao.orientacao`?",
			Criteria: map[string]string{
				"true":  "O texto é desse tipo de documento, mesmo que o layout ou o emissor sejam diferentes do habitual.",
				"false": "O texto é de outro tipo de documento, está em branco ou não é possível identificar o tipo.",
			},
		},
		"integridade": {
			Type:         "noul",
			Instructions: "O documento em `documento.texto` está completo, com todas as páginas e sem conteúdo cortado ou faltando?",
			Criteria: map[string]string{
				"true":  "O documento tem começo, meio e fim coerentes e os campos essenciais do tipo estão presentes.",
				"false": "Faltam páginas, campos essenciais ou o conteúdo termina no meio de uma seção.",
			},
		},
		"legibilidade": {
			Type:         "score",
			Instructions: "Avalie a legibilidade do conteúdo recuperado em `documento.texto` para conferência por um contador.",
			Criteria: []string{
				"Ilegível: o texto é ruído, está vazio ou não permite identificar nenhum campo.",
				"Ruim: trechos isolados são legíveis, mas valores, datas ou nomes estão corrompidos ou ausentes.",
				"Aceitável: os campos principais são legíveis, com algumas falhas pontuais que não impedem a conferência.",
				"Boa: o texto está íntegro e todos os campos relevantes são lidos sem ambiguidade.",
			},
		},
		"competencia": {
			Type:         "choice",
			Instructions: "Comparando o período, a data de emissão ou o mês de referência que aparecem em `documento.texto` com a competência pedida em `solicitacao.competencia` (formato ano-mês), qual é a situação?",
			Criteria: map[string]string{
				"correta":    "O documento se refere à competência pedida.",
				"divergente": "O documento se refere a outro período, claramente diferente do pedido.",
				"ausente":    "O documento não informa período, ou o tipo de documento não tem competência.",
			},
		},
		"titular": {
			Type:         "choice",
			Instructions: "Comparando o titular, a razão social ou o CNPJ/CPF que aparecem em `documento.texto` com o cliente em `solicitacao.cliente`, qual é a situação?",
			Criteria: map[string]string{
				"confere":    "O titular do documento é o cliente indicado, admitindo abreviações e variações de grafia do nome.",
				"divergente": "O titular é outra pessoa física ou jurídica.",
				"ausente":    "O documento não identifica o titular.",
			},
		},
		"problema": {
			Type:         "choice",
			Instructions: "Qual é o principal motivo que impediria um contador de aceitar o documento em `documento.texto` para a solicitação descrita? Escolha `nenhum` quando o documento serve.",
			Criteria: map[string]string{
				"nenhum":              "O documento atende ao que foi pedido e pode ser aceito.",
				"ilegivel":            "A qualidade da imagem ou da digitalização impede a leitura dos campos.",
				"documento_diferente": "É outro tipo de documento, não o que foi solicitado.",
				"competencia_errada":  "É o documento certo, mas de outro período.",
				"titular_diferente":   "É o documento certo, mas de outro titular.",
				"incompleto":          "Faltam páginas ou campos essenciais.",
				"indicio_adulteracao": "Há sinais visíveis de edição, rasura ou montagem.",
				"vencido":             "O documento perdeu a validade.",
			},
		},
	}
}

// Policy holds the thresholds the verdict is composed with. They live in
// configuration because the right cut-off depends on the office's tolerance
// and has to be calibrated on its own documents, not assumed.
type Policy struct {
	ApproveMatch     float64 // minimum P(tipo_correto) to approve
	ApproveIntegrity float64 // minimum P(integridade) to approve
	ApproveSignal    float64 // minimum confidence on the supporting choices
	ApproveLegible   float64 // minimum legibility score to approve
	RejectMatch      float64 // P(tipo_correto) at or below this rejects
	RejectIssue      float64 // confidence needed to reject on a stated problem
	RejectLegible    float64 // legibility score at or below this rejects
}

// DefaultPolicy is deliberately cautious: it approves only when every signal
// agrees and sends everything else to a person. These numbers are a starting
// point to calibrate against the office's own documents, not a guarantee.
func DefaultPolicy() Policy {
	return Policy{ApproveMatch: 0.90, ApproveIntegrity: 0.85, ApproveSignal: 0.70, ApproveLegible: 2.0, RejectMatch: 0.20, RejectIssue: 0.75, RejectLegible: 1.0}
}

// Outcome is the composed verdict. Reason is user-facing Portuguese and
// becomes the correction reason when the verdict is rejected.
type Outcome struct {
	Verdict    string
	Confidence float64
	Reason     string
	Signals    []domain.AnalysisSignal
}

var signalLabels = map[string]string{
	"tipo_correto": "Documento corresponde ao solicitado",
	"integridade":  "Documento completo",
	"legibilidade": "Legibilidade",
	"competencia":  "Competência",
	"titular":      "Titular",
	"problema":     "Impedimento encontrado",
}

// Decide composes the verdict from the raw answers. The model judges; this
// function alone decides, so the policy can change without new inference and
// a missing or unparseable answer always lands on review rather than on an
// automatic approval.
func Decide(answers map[string]Answer, p Policy) Outcome {
	out := Outcome{Verdict: "review", Signals: Signals(answers)}
	required := []string{"tipo_correto", "integridade", "legibilidade", "competencia", "titular", "problema"}
	for _, key := range required {
		if _, ok := answers[key]; !ok {
			out.Reason = "A análise automática não respondeu a todos os critérios. Confira manualmente."
			return out
		}
	}
	match := answers["tipo_correto"].Noul
	integrity := answers["integridade"].Noul
	legible := answers["legibilidade"].Score
	competence := answers["competencia"]
	holder := answers["titular"]
	problem := answers["problema"]

	// Rejection first: a stated blocking problem the model is confident about,
	// a document that is plainly not the one requested, or text nobody could
	// confer. Each path carries the sentence the client will read.
	switch {
	case problem.Choice != "nenhum" && problem.Choice != "" && problem.Certainty() >= p.RejectIssue && problemLabels[problem.Choice] != "":
		out.Verdict, out.Confidence, out.Reason = "rejected", problem.Certainty(), problemLabels[problem.Choice]
		return out
	case match <= p.RejectMatch:
		out.Verdict, out.Confidence, out.Reason = "rejected", 1-match, problemLabels["documento_diferente"]
		return out
	case legible <= p.RejectLegible && answers["legibilidade"].Certainty() >= p.RejectIssue:
		out.Verdict, out.Confidence, out.Reason = "rejected", answers["legibilidade"].Certainty(), problemLabels["ilegivel"]
		return out
	}

	// Approval needs every signal to agree. Competência and titular are
	// accepted as "ausente" because plenty of valid documents carry neither.
	approvals := []struct {
		ok   bool
		note string
	}{
		{match >= p.ApproveMatch, "o tipo do documento não ficou claro"},
		{integrity >= p.ApproveIntegrity, "não foi possível confirmar que o documento está completo"},
		{legible >= p.ApproveLegible, "a legibilidade ficou abaixo do aceitável"},
		{competence.Choice != "divergente" && competence.Certainty() >= p.ApproveSignal, "a competência não pôde ser confirmada"},
		{holder.Choice != "divergente" && holder.Certainty() >= p.ApproveSignal, "o titular não pôde ser confirmado"},
		{problem.Choice == "nenhum" && problem.Certainty() >= p.ApproveSignal, "restou dúvida sobre um possível impedimento"},
	}
	pending := []string{}
	confidence := 1.0
	for _, a := range approvals {
		if !a.ok {
			pending = append(pending, a.note)
		}
	}
	for _, key := range required {
		if c := answers[key].Certainty(); c < confidence {
			confidence = c
		}
	}
	out.Confidence = confidence
	if len(pending) == 0 {
		out.Verdict = "approved"
		out.Reason = "Todos os critérios conferem com o que foi solicitado."
		return out
	}
	out.Reason = "Revisão humana recomendada: " + strings.Join(pending, "; ") + "."
	return out
}

// Signals flattens the answers for storage and display, in a stable order so
// the panel does not reshuffle between two analyses of the same item.
func Signals(answers map[string]Answer) []domain.AnalysisSignal {
	out := []domain.AnalysisSignal{}
	for key, a := range answers {
		label := signalLabels[key]
		if label == "" {
			label = key
		}
		signal := domain.AnalysisSignal{Key: key, Label: label, Kind: a.Type, Confidence: a.Certainty()}
		switch a.Type {
		case "noul":
			signal.Value = a.Noul
		case "score":
			signal.Value = a.Score
		case "choice":
			signal.Answer = a.Choice
			signal.Value = a.Probabilities[a.Choice]
		}
		out = append(out, signal)
	}
	order := map[string]int{"tipo_correto": 0, "competencia": 1, "titular": 2, "integridade": 3, "legibilidade": 4, "problema": 5}
	sort.Slice(out, func(i, j int) bool {
		a, ok := order[out[i].Key]
		b, okb := order[out[j].Key]
		if !ok {
			a = 99
		}
		if !okb {
			b = 99
		}
		if a == b {
			return out[i].Key < out[j].Key
		}
		return a < b
	})
	return out
}
