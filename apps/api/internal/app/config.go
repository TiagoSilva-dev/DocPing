package app

import (
	"crypto/x509"
	"docping/api/internal/domain"
	"docping/api/internal/persistence"
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
	SMTPRequireTLS, AllowUnscanned, S3AcceptUnconfirmedEncryption                                                                                             bool
	// SMTPTLSMode is how the connection reaches TLS: "starttls" upgrades a
	// plain connection (port 587 and most providers), "implicit" speaks TLS
	// from the first byte (port 465, which Hostinger recommends). Reading the
	// greeting of an implicit-TLS port over a plain socket never works, so the
	// mode is part of the configuration rather than something to discover.
	SMTPTLSMode string
	// smtpRoots overrides the trust anchors for the SMTP connection. It stays
	// unexported and nil in every deployment — the system roots — and exists
	// so a test can serve implicit TLS with a certificate it generated
	// instead of asking the code to skip verification.
	smtpRoots                               *x509.CertPool
	MaxUpload, MaxOrgBytes, MaxRequestBytes int64
	ClientLimit                             int
	// Abuse limits for the two routes that create an organization without
	// anyone paying or proving anything. DemoTTL and UnverifiedTTL are how
	// long such an organization survives before the cleanup job deletes it;
	// DemoMaxActive caps how many demos may exist at once, so a distributed
	// flood cannot grow the state without bound even within the rate limit.
	DemoTTL, UnverifiedTTL time.Duration
	DemoMaxActive          int
	DemoRateHourly         int
	// ResendKey turns on sending over HTTPS instead of SMTP. It exists
	// because SMTP is not always reachable: a platform that blocks the port
	// makes it time out silently, and a product whose only transport is SMTP
	// then cannot tell a customer anything. When set, it wins over SMTP.
	ResendKey string
	// MailFrom overrides the sender for either transport. Read it through
	// Sender(), which falls back to SMTPFrom so a deployment configured
	// before this field existed keeps working unchanged.
	MailFrom string
	// Billing. StripeKey being set is what turns subscriptions on: a
	// deployment without it behaves exactly as the product did before money
	// existed, which is what keeps a development machine and the test suite
	// usable. It is never inferred from APP_ENV.
	StripeKey, StripeWebhookSecret string
	// Prices maps a plan slug to the Stripe price that sells it. A price is
	// replaced in the dashboard without this code knowing, so the mapping is
	// configuration rather than a constant.
	Prices map[string]string
	// VerifyEmail gates login until the registrant confirms the address. It is
	// derived from SMTP being configured, never set by hand: production always
	// has SMTP, and a development machine without it must stay usable.
	VerifyEmail bool
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

// SMTP TLS modes. Both are encrypted; they differ only in when.
const (
	smtpTLSStartTLS = "starttls"
	smtpTLSImplicit = "implicit"
)

// resolveSMTPTLSMode defaults by port, because 465 is implicit TLS by
// convention and 587/25 are STARTTLS. An explicit SMTP_TLS_MODE always wins.
func resolveSMTPTLSMode(configured, port string) string {
	if configured != "" {
		return strings.ToLower(strings.TrimSpace(configured))
	}
	if port == "465" {
		return smtpTLSImplicit
	}
	return smtpTLSStartTLS
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
	prices := map[string]string{}
	for plan, key := range map[string]string{
		domain.PlanEssencial:  "STRIPE_PRICE_ESSENCIAL",
		domain.PlanEscritorio: "STRIPE_PRICE_ESCRITORIO",
		domain.PlanRede:       "STRIPE_PRICE_REDE",
	} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			prices[plan] = v
		}
	}
	demoHours, _ := strconv.Atoi(env("DEMO_TTL_HOURS", "24"))
	unverifiedHours, _ := strconv.Atoi(env("UNVERIFIED_TTL_HOURS", "168"))
	demoMax, _ := strconv.Atoi(env("DEMO_MAX_ACTIVE", "200"))
	demoRate, _ := strconv.Atoi(env("DEMO_RATE_PER_HOUR", "3"))
	c := Config{Env: env("APP_ENV", "production"), Port: env("PORT", "8080"), Origin: env("WEB_ORIGIN", env("FRONTEND_URL", "http://localhost:5173")), Driver: env("STORE_DRIVER", "mysql"), DSN: env("MYSQL_DSN", os.Getenv("DATABASE_DSN")), MySQLCACert: os.Getenv("MYSQL_CA_CERT"), DataDir: env("DATA_DIR", ".data"), S3Endpoint: os.Getenv("S3_ENDPOINT"), S3Bucket: env("S3_BUCKET", "docping"), S3Access: os.Getenv("S3_ACCESS_KEY"), S3Secret: os.Getenv("S3_SECRET_KEY"), Scanner: os.Getenv("CLAMAV_ADDRESS"), AllowUnscanned: os.Getenv("ALLOW_UNSCANNED_UPLOADS") == "true", S3AcceptUnconfirmedEncryption: os.Getenv("S3_ACCEPT_UNCONFIRMED_ENCRYPTION") == "true", SMTPHost: os.Getenv("SMTP_HOST"), SMTPPort: env("SMTP_PORT", "1025"), SMTPFrom: os.Getenv("SMTP_FROM"), SMTPUser: os.Getenv("SMTP_USERNAME"), SMTPPass: os.Getenv("SMTP_PASSWORD"), SMTPRequireTLS: env("SMTP_REQUIRE_TLS", "true") == "true", SMTPTLSMode: resolveSMTPTLSMode(os.Getenv("SMTP_TLS_MODE"), env("SMTP_PORT", "1025")), MaxUpload: maxBytes, MaxOrgBytes: orgBytes, MaxRequestBytes: requestBytes, ClientLimit: limit, Location: loc, AIReview: os.Getenv("AI_REVIEW_ENABLED") == "true", DoclingURL: os.Getenv("DOCLING_URL"), DoclingKey: os.Getenv("DOCLING_API_KEY"), DoclingLang: env("DOCLING_OCR_LANG", "pt"), PaddleURL: os.Getenv("PADDLEOCR_URL"), JevURL: env("TYPESAFE_API_URL", "https://api.typesafe.ai"), JevKey: os.Getenv("TYPESAFE_API_KEY"), JevModel: env("TYPESAFE_MODEL", "jev-latest"), AIMaxChars: aiChars, AIApprove: aiApprove, AIReject: aiReject, DemoTTL: time.Duration(demoHours) * time.Hour, UnverifiedTTL: time.Duration(unverifiedHours) * time.Hour, DemoMaxActive: demoMax, ResendKey: strings.TrimSpace(os.Getenv("RESEND_API_KEY")), MailFrom: strings.TrimSpace(os.Getenv("MAIL_FROM")), DemoRateHourly: demoRate, StripeKey: strings.TrimSpace(os.Getenv("STRIPE_SECRET_KEY")), StripeWebhookSecret: strings.TrimSpace(os.Getenv("STRIPE_WEBHOOK_SECRET")), Prices: prices}
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
	// The mail rule is about having a way to send, not about which one: an
	// HTTPS provider satisfies it on its own, and SMTP still has to be TLS
	// when SMTP is the path being used.
	if c.Env == "production" && (c.Driver != "mysql" || c.DSN == "" || c.S3Endpoint == "" || c.Scanner == "" || c.AllowUnscanned || !c.CanSendEmail() || c.SMTPHost != "" && !c.SMTPRequireTLS) {
		return c, errors.New("production requires HTTPS WEB_ORIGIN, MySQL, private S3, ClamAV and a way to send e-mail (RESEND_API_KEY, or SMTP with TLS); development bypasses are forbidden")
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
	if c.SMTPTLSMode != smtpTLSStartTLS && c.SMTPTLSMode != smtpTLSImplicit {
		return c, errors.New("SMTP_TLS_MODE must be " + smtpTLSStartTLS + " or " + smtpTLSImplicit)
	}
	if c.DemoTTL < time.Hour || c.DemoTTL > 720*time.Hour || c.UnverifiedTTL < time.Hour || c.UnverifiedTTL > 8760*time.Hour || c.DemoMaxActive < 1 || c.DemoRateHourly < 1 || c.DemoRateHourly > 600 {
		return c, errors.New("DEMO_TTL_HOURS 1-720, UNVERIFIED_TTL_HOURS 1-8760, DEMO_MAX_ACTIVE and DEMO_RATE_PER_HOUR must be positive")
	}
	if err := c.checkBilling(); err != nil {
		return c, err
	}
	// An unconfirmed registration is only worth gating when there is a way to
	// send the confirmation, by either transport. Production is already
	// required to have one above, so this is on wherever it can be, and off
	// only on a development machine with no way to send at all.
	c.VerifyEmail = c.CanSendEmail()
	if c.Env == "production" && !c.VerifyEmail {
		return c, errors.New("production requires a way to send e-mail so new accounts can confirm their address")
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
// The anchor itself is registered by the persistence package, which is what
// opens the connection; this is only the configuration rule around it.
const mysqlCATLSConfig = persistence.CATLSConfig

func (c Config) registerMySQLCA() error {
	if c.MySQLCACert == "" {
		if strings.Contains(c.DSN, "tls="+mysqlCATLSConfig) {
			return errors.New("MYSQL_DSN with tls=" + mysqlCATLSConfig + " requires MYSQL_CA_CERT")
		}
		return nil
	}
	return persistence.RegisterCA(c.MySQLCACert)
}

// Billing is on when a Stripe key is configured. Everything about charging --
// the gate on a workspace, the checkout route, the webhook -- hangs off this,
// so a deployment with no key is the product as it was before subscriptions.
func (c Config) Billing() bool { return c.StripeKey != "" }

// PlanForPrice resolves a Stripe price back to the plan it sells. An unknown
// price returns an empty slug, and the caller must treat that as a refusal
// rather than as a default plan: it means money arrived for something this
// deployment does not sell, and guessing would hand out the wrong allowance.
func (c Config) PlanForPrice(price string) string {
	for plan, id := range c.Prices {
		if id == price {
			return plan
		}
	}
	return ""
}

// checkBilling refuses a configuration that would charge, or fail to charge,
// by halves. The rules exist because each missing piece fails silently and
// late: an unverified webhook is a forged subscription, a missing price is a
// checkout that cannot open, and a test key in production is a payment that
// never happened against a workspace that was let in.
func (c Config) checkBilling() error {
	if !c.Billing() {
		// Without a key the three prices are inert, but a half-configured
		// deployment is nearly always a mistake worth naming at boot.
		if len(c.Prices) > 0 || c.StripeWebhookSecret != "" {
			return errors.New("STRIPE_PRICE_*/STRIPE_WEBHOOK_SECRET set without STRIPE_SECRET_KEY")
		}
		return nil
	}
	if c.StripeWebhookSecret == "" {
		return errors.New("STRIPE_WEBHOOK_SECRET is required: without it a subscription could be forged by anyone who can reach the webhook")
	}
	for _, plan := range []string{domain.PlanEssencial, domain.PlanEscritorio, domain.PlanRede} {
		if c.Prices[plan] == "" {
			return errors.New("STRIPE_PRICE_* missing for plan " + plan)
		}
	}
	seen := map[string]string{}
	for plan, id := range c.Prices {
		if other, ok := seen[id]; ok {
			return errors.New("the same Stripe price sells both " + other + " and " + plan)
		}
		seen[id] = plan
	}
	if c.Env == "production" {
		if strings.HasPrefix(c.StripeKey, "sk_test") || strings.HasPrefix(c.StripeKey, "rk_test") {
			return errors.New("production refuses a Stripe test key: it would open a workspace against a payment that never happened")
		}
		if !strings.HasPrefix(c.StripeKey, "sk_live") && !strings.HasPrefix(c.StripeKey, "rk_live") {
			return errors.New("STRIPE_SECRET_KEY does not look like a live key")
		}
	}
	return nil
}

// Sender is the address messages go out as. MAIL_FROM names it; SMTP_FROM is
// what named it before a second transport existed, and still does when the
// newer name is absent.
func (c Config) Sender() string {
	if c.MailFrom != "" {
		return c.MailFrom
	}
	return c.SMTPFrom
}

// CanSendEmail answers whether this deployment has any way to deliver a
// message. Both transports need a sender; neither can invent one.
func (c Config) CanSendEmail() bool {
	return c.Sender() != "" && (c.ResendKey != "" || c.SMTPHost != "")
}

// MailTransport names the path a message takes, for the start-up log. Which
// one is in use is the first thing to know when a message does not arrive.
func (c Config) MailTransport() string {
	switch {
	case c.ResendKey != "":
		return "resend (https)"
	case c.SMTPHost != "":
		return "smtp " + net.JoinHostPort(c.SMTPHost, c.SMTPPort) + " (" + c.SMTPTLSMode + ")"
	}
	return "none"
}
