package app

import (
	"html"
	"strings"
)

// The HTML shell for a transactional message. It is built here, in the
// repository, rather than stored in the provider's dashboard: the words
// already live in this code, the local stack sends the same message over
// SMTP, and a template kept on one provider would make the two drift.
//
// Mail clients are not browsers. Everything below is deliberate:
//
//   - Layout is tables with inline styles. A <style> block is dropped by
//     several clients and by Gmail's clipped view.
//   - No image, not even the logo. Most clients block remote images by
//     default and inline SVG is stripped, so a message built around one
//     arrives empty. The wordmark is text, which always renders.
//   - Colours are the product's own -- #141414 on white, #6b6b6b for the
//     secondary line, #e5e5e5 for rules -- and every surface declares its
//     background, so a client forcing dark mode cannot leave text on a
//     background of the same colour.
//   - Every message carries a plain-text alternative. Some people read mail
//     as text, and a message with no text part looks like spam to a filter.
const (
	emailInk       = "#141414"
	emailMuted     = "#6b6b6b"
	emailBorder    = "#e5e5e5"
	emailBackdrop  = "#fafafa"
	emailSurface   = "#ffffff"
	emailFontStack = "'DM Sans',-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif"
)

// emailAction is the one thing the message asks the reader to do. A
// transactional message has at most one.
type emailAction struct{ Label, URL string }

// emailContent is a message to be rendered in the shell.
type emailContent struct {
	// Preheader is the line a client shows next to the subject in the list.
	// Left empty, clients print whatever text comes first, which is usually
	// the greeting -- and every message then looks identical in the list.
	Preheader  string
	Heading    string
	Paragraphs []string
	Action     *emailAction
	// Note closes the message: what to do if this was not you, or how long
	// the link lasts. It is printed smaller and quieter, never in colour.
	Note string
}

func (a *App) renderEmail(c emailContent) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="x-apple-disable-message-reformatting"><title>`)
	b.WriteString(html.EscapeString(c.Heading))
	b.WriteString(`</title></head><body style="margin:0;padding:0;background:` + emailBackdrop + `;">`)
	if c.Preheader != "" {
		// Hidden from the body, read by the client for the list preview. The
		// trailing spaces stop the next line of markup being pulled in after it.
		b.WriteString(`<div style="display:none;max-height:0;overflow:hidden;opacity:0;">`)
		b.WriteString(html.EscapeString(c.Preheader))
		b.WriteString(strings.Repeat("&#8203;&nbsp;", 40))
		b.WriteString(`</div>`)
	}
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:` + emailBackdrop + `;"><tr><td align="center" style="padding:32px 16px;">`)
	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="max-width:560px;width:100%;background:` + emailSurface + `;border:1px solid ` + emailBorder + `;border-radius:12px;">`)

	// Wordmark, on its own rule-separated row.
	b.WriteString(`<tr><td style="padding:26px 32px 22px;border-bottom:1px solid ` + emailBorder + `;">`)
	b.WriteString(`<span style="font-family:` + emailFontStack + `;font-size:17px;font-weight:600;letter-spacing:-0.4px;color:` + emailInk + `;">DocPing</span>`)
	b.WriteString(`</td></tr>`)

	b.WriteString(`<tr><td style="padding:30px 32px 32px;font-family:` + emailFontStack + `;">`)
	b.WriteString(`<h1 style="margin:0 0 14px;font-size:21px;line-height:1.3;font-weight:600;letter-spacing:-0.4px;color:` + emailInk + `;">`)
	b.WriteString(html.EscapeString(c.Heading))
	b.WriteString(`</h1>`)
	for _, p := range c.Paragraphs {
		b.WriteString(`<p style="margin:0 0 14px;font-size:15px;line-height:1.6;color:` + emailInk + `;">`)
		b.WriteString(html.EscapeString(p))
		b.WriteString(`</p>`)
	}
	if c.Action != nil {
		// A table-wrapped button: the only shape every client renders as one.
		b.WriteString(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:22px 0 18px;"><tr><td style="background:` + emailInk + `;border-radius:9px;">`)
		b.WriteString(`<a href="` + html.EscapeString(c.Action.URL) + `" style="display:inline-block;padding:13px 22px;font-family:` + emailFontStack + `;font-size:15px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:9px;">`)
		b.WriteString(html.EscapeString(c.Action.Label))
		b.WriteString(`</a></td></tr></table>`)
		// The address in full, for a client that strips the button or a
		// reader who would rather see where the link goes before following it.
		b.WriteString(`<p style="margin:0 0 6px;font-size:13px;line-height:1.5;color:` + emailMuted + `;">Se o botão não funcionar, copie e cole este endereço no navegador:</p>`)
		b.WriteString(`<p style="margin:0 0 4px;font-size:13px;line-height:1.5;word-break:break-all;"><a href="` + html.EscapeString(c.Action.URL) + `" style="color:` + emailInk + `;">`)
		b.WriteString(html.EscapeString(c.Action.URL))
		b.WriteString(`</a></p>`)
	}
	if c.Note != "" {
		b.WriteString(`<p style="margin:22px 0 0;padding-top:18px;border-top:1px solid ` + emailBorder + `;font-size:13px;line-height:1.6;color:` + emailMuted + `;">`)
		b.WriteString(html.EscapeString(c.Note))
		b.WriteString(`</p>`)
	}
	b.WriteString(`</td></tr></table>`)

	b.WriteString(`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="max-width:560px;width:100%;"><tr><td style="padding:18px 32px;text-align:center;font-family:` + emailFontStack + `;font-size:12px;line-height:1.6;color:` + emailMuted + `;">`)
	b.WriteString(`DocPing — coleta de documentos do cliente<br>Mensagem automática, enviada por <a href="` + html.EscapeString(a.Config.Origin) + `" style="color:` + emailMuted + `;">docping.com.br</a>`)
	b.WriteString(`</td></tr></table>`)
	b.WriteString(`</td></tr></table></body></html>`)
	return b.String()
}

// plainEmail writes the same message as text. It is not a fallback that
// nobody reads: a filter treats a message with no text part as suspect, and
// some people read everything as text on purpose.
func plainEmail(c emailContent) string {
	var b strings.Builder
	b.WriteString(c.Heading + "\n\n")
	for _, p := range c.Paragraphs {
		b.WriteString(p + "\n\n")
	}
	if c.Action != nil {
		b.WriteString(c.Action.Label + ":\n" + c.Action.URL + "\n\n")
	}
	if c.Note != "" {
		b.WriteString(c.Note + "\n")
	}
	return b.String()
}
