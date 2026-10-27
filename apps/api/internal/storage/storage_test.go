package storage

import (
	"context"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeRoundTripper func(*http.Request) (*http.Response, error)

func (f fakeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProductionSSERequiredAndAcknowledged(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		production, ack, accept, wantError bool
	}{
		{"production encrypted", true, true, false, false},
		{"production missing confirmation", true, false, false, true},
		{"production accepting an unconfirmed provider", true, false, true, false},
		{"development no SSE requirement", false, false, false, false},
	} {
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
			o := &Objects{client: client, bucket: "private-fixture", requireEncryption: tc.production, acceptUnconfirmed: tc.accept}
			err = o.Put(context.Background(), "synthetic-object", "application/pdf", []byte("fixture"))
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v wantError=%v", err, tc.wantError)
			}
			if tc.wantError && deletes != 1 {
				t.Fatal("unconfirmed object was not removed")
			}
			if !tc.wantError && deletes != 0 {
				t.Fatal("an accepted object was removed anyway")
			}
		})
	}
}
