package app

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// serveSMTPConn is the conversation both fixtures speak: enough of SMTP for
// the client to deliver one message, which is then published on the channel.
func serveSMTPConn(conn net.Conn, messages chan<- string) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)
	send := func(s string) { fmt.Fprint(writer, s+"\r\n"); writer.Flush() }
	send("220 localhost SMTP test")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		upper := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			send("250 localhost")
		case strings.HasPrefix(upper, "MAIL"), strings.HasPrefix(upper, "RCPT"):
			send("250 accepted")
		case upper == "DATA":
			send("354 Send message")
			var message strings.Builder
			for {
				part, e := reader.ReadString('\n')
				if e != nil {
					return
				}
				if strings.TrimSpace(part) == "." {
					break
				}
				message.WriteString(part)
			}
			messages <- message.String()
			send("250 queued")
		case upper == "QUIT":
			send("221 bye")
			return
		default:
			send("250 ok")
		}
	}
}

// smtpImplicitTLSFixture answers in TLS from the first byte, the way port 465
// does. The certificate it generates is returned as a pool so the client can
// verify it properly: the test proves the real verification path works, not
// that verification can be skipped.
func smtpImplicitTLSFixture(t *testing.T) (string, string, <-chan string, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "smtp-fixture"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Skip("local TLS listener unavailable:", err)
	}
	t.Cleanup(func() { listener.Close() })
	messages := make(chan string, 10)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveSMTPConn(conn, messages)
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	return host, port, messages, pool
}

// Hostinger's recommended port is 465, which answers in TLS before any SMTP
// greeting. Reading that greeting from a plain socket is what the previous
// implementation did, and it could never work.
func TestSMTPImplicitTLSDelivers(t *testing.T) {
	app, _ := setup(t)
	host, port, messages, pool := smtpImplicitTLSFixture(t)
	app.Config.SMTPHost, app.Config.SMTPPort = host, port
	app.Config.SMTPFrom = "documentos@example.invalid"
	app.Config.SMTPTLSMode = smtpTLSImplicit
	app.Config.smtpRoots = pool
	if err := app.sendEmail("cliente@example.invalid", "Assunto com acento — ção", "corpo da mensagem"); err != nil {
		t.Fatalf("envio em TLS implícito falhou: %v", err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "corpo da mensagem") {
			t.Fatalf("mensagem entregue sem o corpo: %s", message)
		}
		if !strings.Contains(message, "To: cliente@example.invalid") {
			t.Fatalf("mensagem sem destinatário: %s", message)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nenhuma mensagem chegou ao servidor")
	}
}

// The mode must never degrade to plaintext when the port turns out not to
// speak TLS: that would send credentials and documents in the clear.
func TestSMTPImplicitTLSNeverFallsBackToPlaintext(t *testing.T) {
	app, _ := setup(t)
	host, port, messages := smtpFixture(t)
	app.Config.SMTPHost, app.Config.SMTPPort = host, port
	app.Config.SMTPFrom = "documentos@example.invalid"
	app.Config.SMTPTLSMode = smtpTLSImplicit
	if err := app.sendEmail("cliente@example.invalid", "Assunto", "corpo"); err == nil {
		t.Fatal("envio deveria falhar: a porta não oferece TLS")
	}
	select {
	case message := <-messages:
		t.Fatalf("mensagem trafegou sem TLS: %s", message)
	case <-time.After(300 * time.Millisecond):
	}
}

// A certificate the system roots do not sign has to be refused, or the
// implicit mode would be encryption without authentication.
func TestSMTPImplicitTLSRefusesUntrustedCertificate(t *testing.T) {
	app, _ := setup(t)
	host, port, _, _ := smtpImplicitTLSFixture(t)
	app.Config.SMTPHost, app.Config.SMTPPort = host, port
	app.Config.SMTPFrom = "documentos@example.invalid"
	app.Config.SMTPTLSMode = smtpTLSImplicit
	// No pool: the fixture's self-signed certificate is not a system root.
	if err := app.sendEmail("cliente@example.invalid", "Assunto", "corpo"); err == nil {
		t.Fatal("certificado não confiável foi aceito")
	}
}
func TestResolveSMTPTLSMode(t *testing.T) {
	for name, tc := range map[string]struct {
		configured, port, want string
	}{
		"465 é implícito por convenção":    {"", "465", smtpTLSImplicit},
		"587 é STARTTLS":                   {"", "587", smtpTLSStartTLS},
		"Mailpit local é STARTTLS":         {"", "1025", smtpTLSStartTLS},
		"configuração explícita prevalece": {"IMPLICIT", "587", smtpTLSImplicit},
		"STARTTLS explícito na 465":        {" starttls ", "465", smtpTLSStartTLS},
	} {
		t.Run(name, func(t *testing.T) {
			if got := resolveSMTPTLSMode(tc.configured, tc.port); got != tc.want {
				t.Fatalf("modo %q na porta %q: %q, esperado %q", tc.configured, tc.port, got, tc.want)
			}
		})
	}
}

// The Hostinger shape of the configuration has to start, and a typo in the
// mode has to be refused rather than silently treated as plaintext.
func TestLoadConfigAcceptsHostingerSMTPAndRefusesUnknownTLSMode(t *testing.T) {
	for name, tc := range map[string]struct {
		override map[string]string
		accepted bool
		want     string
	}{
		"Hostinger na 465": {map[string]string{
			"SMTP_HOST": "smtp.hostinger.com", "SMTP_PORT": "465",
			"SMTP_USERNAME": "documentos@docping.com.br", "SMTP_PASSWORD": "segredo",
			"SMTP_FROM": "documentos@docping.com.br",
		}, true, smtpTLSImplicit},
		"Hostinger na 587": {map[string]string{
			"SMTP_HOST": "smtp.hostinger.com", "SMTP_PORT": "587",
			"SMTP_USERNAME": "documentos@docping.com.br", "SMTP_PASSWORD": "segredo",
			"SMTP_FROM": "documentos@docping.com.br",
		}, true, smtpTLSStartTLS},
		"modo desconhecido": {map[string]string{"SMTP_TLS_MODE": "nenhum"}, false, ""},
		"modo vazio explícito não é aceito como plaintext": {map[string]string{"SMTP_TLS_MODE": "none"}, false, ""},
	} {
		t.Run(name, func(t *testing.T) {
			for k, v := range productionBase() {
				t.Setenv(k, v)
			}
			for k, v := range tc.override {
				t.Setenv(k, v)
			}
			c, err := LoadConfig()
			if tc.accepted {
				if err != nil {
					t.Fatalf("configuração válida recusada: %v", err)
				}
				if c.SMTPTLSMode != tc.want {
					t.Fatalf("modo resolvido %q, esperado %q", c.SMTPTLSMode, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatal("modo de TLS inválido foi aceito")
			}
		})
	}
}

// TestHostingerSMTPLive sends one real message through the configured
// provider. Opt-in, like the MySQL integration test: it runs only when
// SMTP_LIVE_TO is set, and it reads the mailbox password from the file named
// by SMTP_LIVE_PASSWORD_FILE so the secret never has to be typed on a command
// line or pasted into a conversation.
//
//	printf '%s' '<senha da caixa>' > ~/.docping-smtp-pass && chmod 600 ~/.docping-smtp-pass
//	SMTP_LIVE_TO=tiago@docping.com.br \
//	SMTP_LIVE_USERNAME=tiago@docping.com.br \
//	SMTP_LIVE_FROM=notificacao@docping.com.br \
//	SMTP_LIVE_PASSWORD_FILE=~/.docping-smtp-pass \
//	go test ./internal/app -run HostingerSMTPLive -v
//
// It proves what the offline tests cannot: that the credentials work, that the
// provider accepts an alias as the sender, and that the message is delivered.
func TestHostingerSMTPLive(t *testing.T) {
	to := os.Getenv("SMTP_LIVE_TO")
	if to == "" {
		t.Skip("set SMTP_LIVE_TO, SMTP_LIVE_USERNAME, SMTP_LIVE_FROM and SMTP_LIVE_PASSWORD_FILE to send one real message")
	}
	file := os.Getenv("SMTP_LIVE_PASSWORD_FILE")
	if file == "" {
		t.Fatal("SMTP_LIVE_PASSWORD_FILE is required: point it at a file holding only the mailbox password")
	}
	if strings.HasPrefix(file, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		file = home + file[1:]
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("could not read the password file: %v", err)
	}
	password := strings.TrimSpace(string(raw))
	if password == "" {
		t.Fatal("the password file is empty")
	}
	username := os.Getenv("SMTP_LIVE_USERNAME")
	from := os.Getenv("SMTP_LIVE_FROM")
	if username == "" || from == "" {
		t.Fatal("SMTP_LIVE_USERNAME and SMTP_LIVE_FROM are required")
	}
	host := os.Getenv("SMTP_LIVE_HOST")
	if host == "" {
		host = "smtp.hostinger.com"
	}
	port := os.Getenv("SMTP_LIVE_PORT")
	if port == "" {
		port = "465"
	}
	app, _ := setup(t)
	app.Config.SMTPHost, app.Config.SMTPPort = host, port
	app.Config.SMTPUser, app.Config.SMTPPass = username, password
	app.Config.SMTPFrom = from
	app.Config.SMTPRequireTLS = true
	// Resolved the same way LoadConfig does, so the test exercises the
	// defaulting rather than asserting a mode of its own.
	app.Config.SMTPTLSMode = resolveSMTPTLSMode(os.Getenv("SMTP_LIVE_TLS_MODE"), port)
	t.Logf("enviando por %s:%s em modo %s, remetente %s", host, port, app.Config.SMTPTLSMode, from)
	subject := "DocPing — teste de envio (" + time.Now().Format("15:04:05") + ")"
	body := "Mensagem de teste do DocPing.\n\nSe você recebeu isto, o transporte SMTP está funcionando: confira no cabeçalho se o remetente é " + from + " e se SPF e DKIM passaram.\n"
	if err := app.sendEmail(to, subject, body); err != nil {
		t.Fatalf("envio real falhou: %v", err)
	}
	t.Log("provedor aceitou a mensagem")
}
