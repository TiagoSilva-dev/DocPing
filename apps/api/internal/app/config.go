package app

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/go-sql-driver/mysql"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env, Port, Origin, Driver, DSN, MySQLCACert, DataDir, S3Endpoint, S3Bucket, S3Access, S3Secret, Scanner, SMTPHost, SMTPPort, SMTPFrom, SMTPUser, SMTPPass string
	SMTPRequireTLS, AllowUnscanned                                                                                                                            bool
	MaxUpload, MaxOrgBytes, MaxRequestBytes                                                                                                                   int64
	ClientLimit                                                                                                                                               int
	// Automated conformity analysis. AIReview is the server-wide switch; an
	// organization still chooses off/suggest/auto for itself. Extraction is
	// Docling with PaddleOCR as the OCR fallback for photographed pages, and
	// Jev composes nothing — AIApprove/AIReject gate the verdict in code.
	AIReview                                       bool
	DoclingURL, DoclingKey, DoclingLang, PaddleURL string
	JevURL, JevKey, JevModel                       string
	AIMaxChars                                     int
	AIApprove, AIReject                            float64
	Location                                       *time.Location
	// Origins holds every browser origin allowed to call the API. Origin is the
	// canonical one used to build links; the extras exist because a deployment
	// is commonly reachable through more than one hostname (loopback aliases in
	// development, an apex plus a www host in production).
	Origins []string
	// TrustedProxies are the CIDRs whose X-Forwarded-For header may be used as
	// the client address for rate limiting. Empty means the direct peer is the
	// only address ever trusted.
	TrustedProxies []*net.IPNet
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

// canonicalOrigin reduces a configured URL to the exact value a browser sends
// in Origin: lowercase scheme and host, explicit port only when non-default,
// no path, query or fragment. A configured path or an unsupported scheme is a
// configuration error rather than a silently unmatchable origin.
func canonicalOrigin(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", errors.New("origin must be an absolute URL such as https://app.example.com")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("origin must use http or https")
	}
	if p := strings.Trim(u.EscapedPath(), "/"); p != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("origin must not contain a path, query or fragment")
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if host == "" {
		return "", errors.New("origin must include a host")
	}
	if port == "" || scheme == "http" && port == "80" || scheme == "https" && port == "443" {
		return scheme + "://" + host, nil
	}
	return scheme + "://" + net.JoinHostPort(host, port), nil
}

// loopbackAliases expands a loopback origin into the names that reach the same
// server, so a developer who types 127.0.0.1 instead of localhost is not met
// with origin_rejected. Only development does this; production stays exact.
func loopbackAliases(origin string) []string {
	u, err := url.Parse(origin)
	if err != nil {
		return nil
	}
	host := u.Hostname()
	if host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return nil
	}
	out := []string{}
	for _, alias := range []string{"localhost", "127.0.0.1", "[::1]"} {
		host := alias
		if p := u.Port(); p != "" {
			host = net.JoinHostPort(strings.Trim(alias, "[]"), p)
		}
		out = append(out, u.Scheme+"://"+host)
	}
	return out
}
func parseOrigins(primary, extra, env, port string) ([]string, error) {
	canonical, err := canonicalOrigin(primary)
	if err != nil {
		return nil, errors.New("WEB_ORIGIN: " + err.Error())
	}
	out, seen := []string{canonical}, map[string]bool{canonical: true}
	add := func(origin string) {
		if origin != "" && !seen[origin] {
			seen[origin] = true
			out = append(out, origin)
		}
	}
	for _, raw := range strings.Split(extra, ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		value, err := canonicalOrigin(raw)
		if err != nil {
			return nil, errors.New("WEB_ORIGINS: " + err.Error())
		}
		add(value)
	}
	if env == "development" || env == "test" {
		// The API is also browsed directly in development (curl, the OpenAPI
		// explorer, a phone on the LAN), and Vite prints both loopback names.
		for _, origin := range append([]string{}, out...) {
			for _, alias := range loopbackAliases(origin) {
				add(alias)
			}
		}
		for _, alias := range loopbackAliases("http://localhost:" + port) {
			add(alias)
		}
	}
	return out, nil
}
func parseTrustedProxies(raw string) ([]*net.IPNet, error) {
	out := []*net.IPNet{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			ip := net.ParseIP(entry)
			if ip == nil {
				return nil, errors.New("TRUSTED_PROXY_CIDRS must list IPs or CIDRs")
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, errors.New("TRUSTED_PROXY_CIDRS must list IPs or CIDRs")
		}
		out = append(out, network)
	}
	return out, nil
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
	aiChars, _ := strconv.Atoi(env("AI_MAX_CHARS", "24000"))
	aiApprove, _ := strconv.ParseFloat(env("AI_APPROVE_CONFIDENCE", "0.70"), 64)
	aiReject, _ := strconv.ParseFloat(env("AI_REJECT_CONFIDENCE", "0.75"), 64)
	c := Config{Env: env("APP_ENV", "production"), Port: env("PORT", "8080"), Origin: env("WEB_ORIGIN", env("FRONTEND_URL", "http://localhost:5173")), Driver: env("STORE_DRIVER", "mysql"), DSN: env("MYSQL_DSN", os.Getenv("DATABASE_DSN")), MySQLCACert: os.Getenv("MYSQL_CA_CERT"), DataDir: env("DATA_DIR", ".data"), S3Endpoint: os.Getenv("S3_ENDPOINT"), S3Bucket: env("S3_BUCKET", "docping"), S3Access: os.Getenv("S3_ACCESS_KEY"), S3Secret: os.Getenv("S3_SECRET_KEY"), Scanner: os.Getenv("CLAMAV_ADDRESS"), AllowUnscanned: os.Getenv("ALLOW_UNSCANNED_UPLOADS") == "true", SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: env("SMTP_PORT", "1025"), SMTPFrom: os.Getenv("SMTP_FROM"), SMTPUser: os.Getenv("SMTP_USERNAME"), SMTPPass: os.Getenv("SMTP_PASSWORD"), SMTPRequireTLS: env("SMTP_REQUIRE_TLS", "true") == "true", MaxUpload: maxBytes, MaxOrgBytes: orgBytes, MaxRequestBytes: requestBytes, ClientLimit: limit, Location: loc, AIReview: os.Getenv("AI_REVIEW_ENABLED") == "true", DoclingURL: os.Getenv("DOCLING_URL"), DoclingKey: os.Getenv("DOCLING_API_KEY"), DoclingLang: env("DOCLING_OCR_LANG", "pt"), PaddleURL: os.Getenv("PADDLEOCR_URL"), JevURL: env("TYPESAFE_API_URL", "https://api.typesafe.ai"), JevKey: os.Getenv("TYPESAFE_API_KEY"), JevModel: env("TYPESAFE_MODEL", "jev-latest"), AIMaxChars: aiChars, AIApprove: aiApprove, AIReject: aiReject}
	if c.Env != "development" && c.Env != "production" && c.Env != "test" {
		return c, errors.New("APP_ENV must be development or production")
	}
	c.Origins, err = parseOrigins(c.Origin, os.Getenv("WEB_ORIGINS"), c.Env, c.Port)
	if err != nil {
		return c, err
	}
	c.Origin = c.Origins[0]
	if c.TrustedProxies, err = parseTrustedProxies(os.Getenv("TRUSTED_PROXY_CIDRS")); err != nil {
		return c, err
	}
	if c.Driver == "mysql" && c.DSN != "" {
		if e := c.registerMySQLCA(); e != nil {
			return c, e
		}
	}
	if c.Env == "production" && (c.Driver != "mysql" || c.DSN == "" || c.S3Endpoint == "" || c.Scanner == "" || c.AllowUnscanned || c.SMTPHost == "" || c.SMTPFrom == "" || !c.SMTPRequireTLS) {
		return c, errors.New("production requires HTTPS WEB_ORIGIN, MySQL, private S3, ClamAV and TLS SMTP; development bypasses are forbidden")
	}
	if c.Env == "production" {
		for _, origin := range c.Origins {
			if !strings.HasPrefix(origin, "https://") {
				return c, errors.New("production requires every allowed origin to use HTTPS")
			}
		}
		dsnConfig, e := mysql.ParseDSN(c.DSN)
		if e != nil || dsnConfig.TLSConfig != "true" && dsnConfig.TLSConfig != mysqlCATLSConfig {
			return c, errors.New("production MySQL DSN requires verified TLS (tls=true, or tls=" + mysqlCATLSConfig + " with MYSQL_CA_CERT)")
		}
	}
	if c.AIReview {
		if c.JevKey == "" || c.DoclingURL == "" && c.PaddleURL == "" {
			return c, errors.New("AI_REVIEW_ENABLED requires TYPESAFE_API_KEY and DOCLING_URL or PADDLEOCR_URL")
		}
		if !strings.HasPrefix(c.JevURL, "https://") {
			return c, errors.New("TYPESAFE_API_URL must use HTTPS")
		}
		// A verdict that decides on its own must never run on a threshold a
		// typo turned into zero, which would approve every document.
		if c.AIApprove < 0.5 || c.AIApprove > 1 || c.AIReject < 0.5 || c.AIReject > 1 || c.AIMaxChars < 500 || c.AIMaxChars > 400000 {
			return c, errors.New("AI_APPROVE_CONFIDENCE and AI_REJECT_CONFIDENCE must be between 0.5 and 1, AI_MAX_CHARS between 500 and 400000")
		}
	}
	if c.MaxOrgBytes < 1 || c.MaxRequestBytes < 1 || c.MaxRequestBytes > c.MaxOrgBytes || c.MaxUpload < 1 || c.MaxUpload > 50<<20 || c.ClientLimit < 1 {
		return c, errors.New("invalid upload/client limit")
	}
	return c, nil
}

// AllowedOriginList is the set an Origin header is matched against. A Config
// assembled directly, as tests do, falls back to the canonical origin so it
// never ends up rejecting every request.
func (c Config) AllowedOriginList() []string {
	if len(c.Origins) > 0 {
		return c.Origins
	}
	if c.Origin == "" {
		return nil
	}
	return []string{c.Origin}
}

// AllowedOrigin reports whether a browser Origin header may call the API.
func (c Config) AllowedOrigin(origin string) bool {
	for _, allowed := range c.AllowedOriginList() {
		if origin == allowed {
			return true
		}
	}
	return false
}

// ClientIP resolves the address used for rate limiting. X-Forwarded-For is read
// only when the direct peer is a configured trusted proxy, so a client can
// never spoof its way into a fresh rate-limit bucket.
func (c Config) ClientIP(remoteAddr, forwardedFor string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if forwardedFor == "" || len(c.TrustedProxies) == 0 {
		return host
	}
	peer := net.ParseIP(host)
	trusted := false
	for _, network := range c.TrustedProxies {
		if peer != nil && network.Contains(peer) {
			trusted = true
			break
		}
	}
	if !trusted {
		return host
	}
	parts := strings.Split(forwardedFor, ",")
	for n := len(parts) - 1; n >= 0; n-- {
		candidate := net.ParseIP(strings.Trim(strings.TrimSpace(parts[n]), "[]"))
		if candidate == nil {
			return host
		}
		inTrusted := false
		for _, network := range c.TrustedProxies {
			if network.Contains(candidate) {
				inTrusted = true
				break
			}
		}
		if !inTrusted {
			return candidate.String()
		}
	}
	return host
}

// mysqlCATLSConfig names the DSN tls= mode that verifies the server against a
// CA given in MYSQL_CA_CERT instead of the system roots. A managed MySQL
// usually presents a certificate signed by a private CA, and the stock mysql
// image presents one carrying no name at all, which tls=true can never match.
// This mode changes which CA is trusted, never whether the certificate is
// checked: the chain is verified and ServerName keeps the hostname check that
// tls=true would have done. A missing or unparsable CA is refused, so the mode
// cannot decay into an unverified connection.
const mysqlCATLSConfig = "docping-ca"

func (c Config) registerMySQLCA() error {
	if c.MySQLCACert == "" {
		if strings.Contains(c.DSN, "tls="+mysqlCATLSConfig) {
			return errors.New("MYSQL_DSN with tls=" + mysqlCATLSConfig + " requires MYSQL_CA_CERT")
		}
		return nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(c.MySQLCACert)) {
		return errors.New("MYSQL_CA_CERT must be a PEM-encoded certificate")
	}
	// ServerName is deliberately left empty: the driver fills it with the host
	// from the DSN, so the hostname is checked against the address actually
	// dialled rather than one restated here.
	return mysql.RegisterTLSConfig(mysqlCATLSConfig, &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
}
