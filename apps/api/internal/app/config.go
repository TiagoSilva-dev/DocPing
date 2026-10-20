package app

import (
	"errors"
	"github.com/go-sql-driver/mysql"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env, Port, Origin, Driver, DSN, DataDir, S3Endpoint, S3Bucket, S3Access, S3Secret, Scanner, SMTPHost, SMTPPort, SMTPFrom, SMTPUser, SMTPPass string
	SMTPRequireTLS, AllowUnscanned                                                                                                               bool
	MaxUpload, MaxOrgBytes, MaxRequestBytes                                                                                                      int64
	ClientLimit                                                                                                                                  int
	Location                                                                                                                                     *time.Location
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func LoadConfig() (Config, error) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return Config{}, err
	}
	maxBytes, _ := strconv.ParseInt(env("MAX_UPLOAD_BYTES", "20971520"), 10, 64)
	orgBytes, _ := strconv.ParseInt(env("MAX_ORG_STORAGE_BYTES", "1073741824"), 10, 64)
	requestBytes, _ := strconv.ParseInt(env("MAX_REQUEST_STORAGE_BYTES", "524288000"), 10, 64)
	limit, _ := strconv.Atoi(env("CLIENT_LIMIT", "30"))
	c := Config{Env: env("APP_ENV", "production"), Port: env("PORT", "8080"), Origin: env("WEB_ORIGIN", env("FRONTEND_URL", "http://localhost:5173")), Driver: env("STORE_DRIVER", "mysql"), DSN: env("MYSQL_DSN", os.Getenv("DATABASE_DSN")), DataDir: env("DATA_DIR", ".data"), S3Endpoint: os.Getenv("S3_ENDPOINT"), S3Bucket: env("S3_BUCKET", "docping"), S3Access: os.Getenv("S3_ACCESS_KEY"), S3Secret: os.Getenv("S3_SECRET_KEY"), Scanner: os.Getenv("CLAMAV_ADDRESS"), AllowUnscanned: os.Getenv("ALLOW_UNSCANNED_UPLOADS") == "true", SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: env("SMTP_PORT", "1025"), SMTPFrom: os.Getenv("SMTP_FROM"), SMTPUser: os.Getenv("SMTP_USERNAME"), SMTPPass: os.Getenv("SMTP_PASSWORD"), SMTPRequireTLS: env("SMTP_REQUIRE_TLS", "true") == "true", MaxUpload: maxBytes, MaxOrgBytes: orgBytes, MaxRequestBytes: requestBytes, ClientLimit: limit, Location: loc}
	if c.Env != "development" && c.Env != "production" && c.Env != "test" {
		return c, errors.New("APP_ENV must be development or production")
	}
	u, e := url.Parse(c.Origin)
	if e != nil || u.Host == "" {
		return c, errors.New("WEB_ORIGIN must be absolute URL")
	}
	c.Origin = strings.TrimRight(c.Origin, "/")
	if c.Env == "production" && (u.Scheme != "https" || c.Driver != "mysql" || c.DSN == "" || c.S3Endpoint == "" || c.Scanner == "" || c.AllowUnscanned || c.SMTPHost == "" || c.SMTPFrom == "" || !c.SMTPRequireTLS) {
		return c, errors.New("production requires HTTPS WEB_ORIGIN, MySQL, private S3, ClamAV and TLS SMTP; development bypasses are forbidden")
	}
	if c.Env == "production" {
		dsnConfig, e := mysql.ParseDSN(c.DSN)
		if e != nil || dsnConfig.TLSConfig != "true" {
			return c, errors.New("production MySQL DSN requires verified TLS (tls=true)")
		}
	}
	if c.MaxOrgBytes < 1 || c.MaxRequestBytes < 1 || c.MaxRequestBytes > c.MaxOrgBytes || c.MaxUpload < 1 || c.MaxUpload > 50<<20 || c.ClientLimit < 1 {
		return c, errors.New("invalid upload/client limit")
	}
	return c, nil
}
