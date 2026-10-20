package storage

import (
	"context"
	"encoding/binary"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestClamAVProtocolFailClosed(t *testing.T) {
	for _, tc := range []struct {
		reply, status string
		wantError     bool
	}{{"stream: OK\x00", "clean", false}, {"stream: Eicar-Test-Signature FOUND\x00", "rejected", true}, {"stream: INSTREAM size limit exceeded. ERROR\x00", "quarantined", true}} {
		t.Run(tc.status, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Skip("local listener unavailable:", err)
			}
			defer listener.Close()
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				command := make([]byte, 10)
				_, _ = io.ReadFull(conn, command)
				if string(command) != "zINSTREAM\x00" {
					return
				}
				for {
					var size uint32
					if binary.Read(conn, binary.BigEndian, &size) != nil {
						return
					}
					if size == 0 {
						break
					}
					_, _ = io.CopyN(io.Discard, conn, int64(size))
				}
				_, _ = conn.Write([]byte(tc.reply))
			}()
			o := &Objects{scanner: listener.Addr().String()}
			status, err := o.Scan(context.Background(), []byte("sample"))
			if status != tc.status || (err != nil) != tc.wantError {
				t.Fatalf("status=%s error=%v", status, err)
			}
		})
	}
}

type fakeRoundTripper func(*http.Request) (*http.Response, error)

func (f fakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProductionSSERequiredAndAcknowledged(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		production, ack, wantError bool
	}{{"production encrypted", true, true, false}, {"production missing confirmation", true, false, true}, {"development no SSE requirement", false, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			deletes := 0
			transport := fakeRoundTripper(func(r *http.Request) (*http.Response, error) {
				if r.Body != nil {
					_, _ = io.Copy(io.Discard, r.Body)
				}
				header := make(http.Header)
				header.Set("Content-Type", "application/pdf")
				header.Set("Last-Modified", "Tue, 20 Oct 2026 03:00:00 GMT")
				switch r.Method {
				case "PUT":
					if tc.production && r.Header.Get("X-Amz-Server-Side-Encryption") != "AES256" {
						t.Fatal("production Put omitted required SSE-S3")
					}
					if !tc.production && r.Header.Get("X-Amz-Server-Side-Encryption") != "" {
						t.Fatal("development unexpectedly requires KMS")
					}
					header.Set("ETag", `"fixture"`)
				case "HEAD":
					header.Set("Content-Length", "7")
					if tc.ack {
						header.Set("X-Amz-Server-Side-Encryption", "AES256")
					}
				case "DELETE":
					deletes++
				}
				return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
			})
			client, err := minio.New("s3.example.invalid", &minio.Options{Creds: credentials.NewStaticV4("synthetic-access", "synthetic-secret", ""), Secure: true, Region: "us-east-1", Transport: transport})
			if err != nil {
				t.Fatal(err)
			}
			o := &Objects{client: client, bucket: "private-fixture", requireEncryption: tc.production}
			err = o.Put(context.Background(), "synthetic-object", "application/pdf", []byte("fixture"))
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v wantError=%v", err, tc.wantError)
			}
			if tc.wantError && deletes != 1 {
				t.Fatal("unconfirmed object was not removed")
			}
		})
	}
}
