package app

import (
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"strings"
	"testing"
	"time"
)

func sampleContent() emailContent {
	return emailContent{
		Preheader:  "Falta um clique para ativar o acesso do seu escritório.",
		Heading:    "Confirme seu e-mail",
		Paragraphs: []string{"Falta um passo para ativar o acesso do seu escritório no DocPing."},
		Action:     &emailAction{Label: "Confirmar meu e-mail", URL: "https://docping.com.br/confirmar-email?token=abc123"},
		Note:       "Se não foi você que criou a conta, ignore esta mensagem.",
	}
}

func TestTheRenderedMessageCarriesTheLinkTwice(t *testing.T) {
	app, _ := setup(t)
	out := app.renderEmail(sampleContent())
	// Once in the button and once in full, because a client may strip the
	// button and a careful reader wants to see where the link goes.
	if strings.Count(out, "https://docping.com.br/confirmar-email?token=abc123") < 2 {
		t.Fatalf("the address should appear as a button and in full:\n%s", out)
	}
	for _, want := range []string{"Confirme seu e-mail", "Confirmar meu e-mail", "DocPing"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%q missing from the message", want)
		}
	}
}

// A message built around an image arrives empty wherever remote images are
// blocked, which is most clients by default.
func TestTheMessageCarriesNoImage(t *testing.T) {
	app, _ := setup(t)
	out := app.renderEmail(sampleContent())
	for _, forbidden := range []string{"<img", "<svg", "background-image"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("the message depends on %q, which many clients block", forbidden)
		}
	}
}

// A <style> block is dropped by several clients and by Gmail's clipped view,
// so every rule has to be inline.
func TestTheMessageHasNoStyleBlock(t *testing.T) {
	app, _ := setup(t)
	if out := app.renderEmail(sampleContent()); strings.Contains(out, "<style") {
		t.Fatal("styles must be inline")
	}
}

// Content is escaped. Nothing in a transactional message is customer-written
// today, but an organization name in a future message would be.
func TestTheMessageEscapesItsContent(t *testing.T) {
	app, _ := setup(t)
	out := app.renderEmail(emailContent{
		Heading:    `Aurora <script>alert(1)</script>`,
		Paragraphs: []string{`"aspas" & <b>tags</b>`},
		Action:     &emailAction{Label: "Abrir", URL: "https://docping.com.br/p/x?a=1&b=2"},
	})
	if strings.Contains(out, "<script>") || strings.Contains(out, "<b>tags</b>") {
		t.Fatalf("content was not escaped:\n%s", out)
	}
	if !strings.Contains(out, "&amp;b=2") {
		t.Fatalf("the address was not escaped as an attribute:\n%s", out)
	}
}

// The text alternative has to say everything the html says, including the
// address: it is what a text reader, and a spam filter, actually sees.
func TestThePlainAlternativeSaysTheSameThing(t *testing.T) {
	text := plainEmail(sampleContent())
	for _, want := range []string{"Confirme seu e-mail", "https://docping.com.br/confirmar-email?token=abc123", "ignore esta mensagem"} {
		if !strings.Contains(text, want) {
			t.Fatalf("%q missing from the text part:\n%s", want, text)
		}
	}
	if strings.Contains(text, "<") {
		t.Fatalf("the text part carries markup:\n%s", text)
	}
}

// With both parts the message is multipart/alternative, the html part is
// quoted-printable -- its lines are longer than SMTP allows raw -- and both
// parts survive a round trip through a real MIME reader.
func TestTheMIMEMessageIsReadableByAMIMEReader(t *testing.T) {
	app, _ := setup(t)
	app.Config.SMTPFrom = "notificacao@docping.com.br"
	content := sampleContent()
	raw := app.mimeMessage("cliente@exemplo.com", "Confirme seu e-mail — DocPing", plainEmail(content), app.renderEmail(content))

	headers, body, found := strings.Cut(raw, "\r\n\r\n")
	if !found {
		t.Fatal("message has no body")
	}
	for _, line := range strings.Split(headers, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("a header line is longer than SMTP allows: %d", len(line))
		}
	}
	_, params, err := mime.ParseMediaType(headerValue(headers, "Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(strings.NewReader(body), params["boundary"])
	seen := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		kind := part.Header.Get("Content-Type")
		data, _ := io.ReadAll(part)
		if part.Header.Get("Content-Transfer-Encoding") == "quoted-printable" {
			decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(string(data))))
			if err != nil {
				t.Fatalf("html part does not decode: %v", err)
			}
			data = decoded
		}
		switch {
		case strings.HasPrefix(kind, "text/plain"):
			seen["text"] = string(data)
		case strings.HasPrefix(kind, "text/html"):
			seen["html"] = string(data)
		}
	}
	if !strings.Contains(seen["text"], "confirmar-email?token=abc123") {
		t.Fatalf("text part lost the address: %q", seen["text"])
	}
	if !strings.Contains(seen["html"], "confirmar-email?token=abc123") || !strings.Contains(seen["html"], "<!doctype html>") {
		t.Fatalf("html part did not survive encoding: %q", seen["html"])
	}
	// Every line of the encoded message must fit what SMTP accepts.
	for _, line := range strings.Split(raw, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("a line is longer than SMTP allows: %d characters", len(line))
		}
	}
}

// Without an html part the message stays exactly what it was before: a
// single text body, no multipart wrapper.
func TestAMessageWithoutHTMLStaysPlain(t *testing.T) {
	app, _ := setup(t)
	app.Config.SMTPFrom = "notificacao@docping.com.br"
	raw := app.mimeMessage("cliente@exemplo.com", "Aviso", "um aviso curto", "")
	if strings.Contains(raw, "multipart/alternative") {
		t.Fatal("a text-only message was wrapped in multipart")
	}
	if !strings.Contains(raw, "Content-Type: text/plain; charset=UTF-8") || !strings.HasSuffix(raw, "um aviso curto\r\n") {
		t.Fatalf("plain message changed shape:\n%s", raw)
	}
}

func headerValue(headers, name string) string {
	for _, line := range strings.Split(headers, "\r\n") {
		if key, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(key), name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// The portal message is the only one carrying text the office wrote, so the
// escaping matters here for real: an organization called "Alves & Cia" must
// not break the markup, and a title must not be able to inject any.
func TestThePortalMessageEscapesWhatTheOfficeWrote(t *testing.T) {
	app, _ := setup(t)
	out := app.renderEmail(emailContent{
		Heading:    "Documentos solicitados",
		Paragraphs: []string{`Alves & Cia solicita: Fechamento <b>mensal</b> (10/2026).`},
		Action:     &emailAction{Label: "Enviar meus documentos", URL: "https://docping.com.br/p/tok&en"},
	})
	if strings.Contains(out, "<b>mensal</b>") {
		t.Fatalf("a title injected markup:\n%s", out)
	}
	if !strings.Contains(out, "Alves &amp; Cia") {
		t.Fatalf("the ampersand was not escaped:\n%s", out)
	}
	if !strings.Contains(out, "/p/tok&amp;en") {
		t.Fatalf("the address was not escaped as an attribute:\n%s", out)
	}
}

// Dates reach a client of the office, so they are written the way a reader
// in Brazil expects -- and an unparsable one is printed as stored rather
// than hidden.
func TestDatesAreWrittenForTheReader(t *testing.T) {
	if got := civilDate("2026-11-20"); got != "20/11/2026" {
		t.Fatalf("due date: %q", got)
	}
	if got := competenceLabel("2026-10"); got != "10/2026" {
		t.Fatalf("competence: %q", got)
	}
	if got := civilDate("amanhã"); got != "amanhã" {
		t.Fatalf("an unreadable date should survive: %q", got)
	}
	if got := competenceLabel(""); got != "" {
		t.Fatalf("empty competence: %q", got)
	}
}

// The whole flow, through a real SMTP server: a registration and a password
// reset both arrive as two-part messages carrying their own link. This is
// the check that the layout is actually wired into what the product sends,
// not only into a function a test calls directly.
func TestTheSentMessagesCarryBothParts(t *testing.T) {
	app, h := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost = host
	app.Config.SMTPPort = port
	app.Config.SMTPFrom = "docping@example.invalid"
	app.Config.SMTPRequireTLS = false
	app.Config.VerifyEmail = true

	next := func(what string) string {
		t.Helper()
		select {
		case m := <-messages:
			return m
		case <-time.After(3 * time.Second):
			t.Fatalf("%s was not sent", what)
			return ""
		}
	}
	bothParts := func(t *testing.T, message, what string) {
		t.Helper()
		if !strings.Contains(message, "multipart/alternative") {
			t.Fatalf("%s has no html part:\n%s", what, message)
		}
		if !strings.Contains(message, "Content-Type: text/plain") || !strings.Contains(message, "Content-Type: text/html") {
			t.Fatalf("%s is missing one of the parts:\n%s", what, message)
		}
	}

	// Not registerTest: with confirmation on, registration deliberately
	// returns no session, and the helper expects one.
	if status, body, _ := req(t, h, "POST", "/api/auth/register", map[string]string{
		"organizationName": "Escritório Teste", "name": "Mariana",
		"email": "layout@example.invalid", "password": "uma senha segura 123",
	}, testSession{}); status != 200 {
		t.Fatalf("register: %d %s", status, body)
	}
	confirmation := next("the confirmation")
	bothParts(t, confirmation, "the confirmation")
	if !strings.Contains(confirmation, "Confirme seu e-mail") {
		t.Fatalf("confirmation lost its words:\n%s", confirmation)
	}

	if status, body, _ := req(t, h, "POST", "/api/auth/forgot-password", map[string]string{"email": "layout@example.invalid"}, testSession{}); status != 200 {
		t.Fatalf("forgot-password: %d %s", status, body)
	}
	reset := next("the password reset")
	bothParts(t, reset, "the password reset")
	for _, want := range []string{"Redefina sua senha", "redefinir-senha?token=3D", "Sua senha atual continua valendo"} {
		if !strings.Contains(reset, want) {
			t.Fatalf("%q missing from the reset message:\n%s", want, reset)
		}
	}
}
