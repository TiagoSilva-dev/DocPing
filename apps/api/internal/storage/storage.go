package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type Objects struct {
	dir               string
	client            *minio.Client
	bucket            string
	requireEncryption bool
	acceptUnconfirmed bool
}

func New(ctx context.Context, dir, endpoint, bucket, access, secret, env string, acceptUnconfirmedEncryption bool) (*Objects, error) {
	o := &Objects{dir: filepath.Join(dir, "objects"), bucket: bucket, requireEncryption: env == "production", acceptUnconfirmed: acceptUnconfirmedEncryption}
	if endpoint == "" {
		if env != "development" {
			return nil, errors.New("private S3 storage is required outside development")
		}
		return o, os.MkdirAll(o.dir, 0700)
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, errors.New("S3_ENDPOINT must be an http(s) URL")
	}
	if env != "development" && u.Scheme != "https" {
		return nil, errors.New("S3 requires HTTPS outside development")
	}
	o.client, err = minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(access, secret, ""), Secure: u.Scheme == "https", Transport: &httpTransport})
	if err != nil {
		return nil, err
	}
	exists, err := o.client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("S3 bucket check: %w", err)
	}
	if !exists {
		if env != "development" {
			return nil, errors.New("S3 bucket must be provisioned privately")
		}
		if err = o.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, err
		}
	}
	return o, nil
}

var httpTransport = http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, ResponseHeaderTimeout: 20 * time.Second}

func (o *Objects) Put(ctx context.Context, key, mime string, data []byte) error {
	if o.client != nil {
		uploaded, err := o.client.PutObject(ctx, o.bucket, key, bytes.NewReader(data), int64(len(data)), o.putOptions(mime))
		if err != nil {
			return err
		}
		if o.requireEncryption {
			info, err := o.client.StatObject(ctx, o.bucket, key, minio.StatObjectOptions{VersionID: uploaded.VersionID})
			confirmed := err == nil && info.Metadata.Get("X-Amz-Server-Side-Encryption") == "AES256"
			// acceptUnconfirmed exists because some providers encrypt at rest
			// without reporting it -- Cloudflare R2 leaves the header
			// unimplemented, and Railway's bucket omits it. Turning it on trades
			// a checked fact for trust in the provider, so it is never the
			// default: the operator sets S3_ACCEPT_UNCONFIRMED_ENCRYPTION and
			// the server says so at every start.
			if !confirmed && !o.acceptUnconfirmed {
				cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_ = o.client.RemoveObject(cleanup, o.bucket, key, minio.RemoveObjectOptions{VersionID: uploaded.VersionID})
				cancel()
				return errors.New("S3 provider did not confirm required SSE-S3 encryption")
			}
		}
		return nil
	}
	p := filepath.Join(o.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// Production requests SSE-S3 explicitly and verifies the provider's HEAD
// response before metadata can expose the uploaded object to any user.
func (o *Objects) putOptions(mime string) minio.PutObjectOptions {
	options := minio.PutObjectOptions{ContentType: mime}
	if o.requireEncryption {
		options.ServerSideEncryption = encrypt.NewSSE()
	}
	return options
}
func (o *Objects) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if o.client != nil {
		obj, err := o.client.GetObject(ctx, o.bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return nil, err
		}
		if _, err = obj.Stat(); err != nil {
			obj.Close()
			return nil, err
		}
		return obj, nil
	}
	return os.Open(filepath.Join(o.dir, filepath.FromSlash(key)))
}
func (o *Objects) Delete(ctx context.Context, key string) error {
	if o.client != nil {
		return o.client.RemoveObject(ctx, o.bucket, key, minio.RemoveObjectOptions{})
	}
	return os.Remove(filepath.Join(o.dir, filepath.FromSlash(key)))
}
