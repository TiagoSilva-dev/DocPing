package app

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type gatedReader struct {
	reader           io.Reader
	entered, release chan struct{}
	once             sync.Once
}

func (g *gatedReader) Read(p []byte) (int, error) {
	g.once.Do(func() { close(g.entered) })
	<-g.release
	return g.reader.Read(p)
}
func TestSlowUploadDoesNotBlockOtherWorkAndRevalidatesToken(t *testing.T) {
	_, h := setup(t)
	s := registerTest(t, h, "concurrency@example.invalid")
	request := createFlow(t, h, s)
	_, data, _ := req(t, h, "POST", "/api/requests/"+request.ID+"/share", nil, s)
	var link struct{ Token string }
	_ = json.Unmarshal(data, &link)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "sample.pdf")
	_, _ = part.Write(samplePDF())
	_ = writer.Close()
	release := make(chan struct{})
	entered := make(chan struct{})
	gate := &gatedReader{reader: bytes.NewReader(body.Bytes()), entered: entered, release: release}
	r := httptest.NewRequest("POST", "/api/portal/"+link.Token+"/items/"+request.Items[0].ID+"/upload", gate)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", "http://localhost:5173")
	done := make(chan int, 1)
	go func() { w := httptest.NewRecorder(); h.ServeHTTP(w, r); done <- w.Code }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("upload did not start")
	}
	other := make(chan int, 1)
	go func() {
		status, _, _ := req(t, h, "POST", "/api/requests/"+request.ID+"/revoke", nil, s)
		other <- status
	}()
	select {
	case status := <-other:
		if status != 200 {
			t.Fatal(status)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("slow upload held global transaction")
	}
	close(release)
	select {
	case status := <-done:
		if status != 410 {
			t.Fatalf("upload did not recheck revoked token: %d", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("upload did not finish")
	}
}
func TestSlowSMTPDoesNotBlockSession(t *testing.T) {
	app, h := setup(t)
	s := registerTest(t, h, "slow-smtp@example.invalid")
	request := createFlow(t, h, s)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("local listener unavailable:", err)
	}
	defer listener.Close()
	release := make(chan struct{})
	entered := make(chan struct{})
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		close(entered)
		<-release
	}()
	app.Config.SMTPHost, app.Config.SMTPPort, _ = net.SplitHostPort(listener.Addr().String())
	app.Config.SMTPFrom = "docping@example.invalid"
	done := make(chan int, 1)
	go func() {
		status, _, _ := req(t, h, "POST", "/api/requests/"+request.ID+"/notify", map[string]string{"channel": "email"}, s)
		done <- status
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("SMTP did not start")
	}
	session := make(chan int, 1)
	go func() { status, _, _ := req(t, h, "GET", "/api/session", nil, s); session <- status }()
	select {
	case status := <-session:
		if status != 200 {
			t.Fatal(status)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("SMTP held global transaction")
	}
	close(release)
	select {
	case status := <-done:
		if status != 200 {
			t.Fatal(status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP completion missing")
	}
}
