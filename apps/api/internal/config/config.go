// Package config reads process configuration from the environment.
package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// IdentityProvider is one deployment-scoped OIDC binding from IDENTITY_PROVIDERS.
// It mirrors identity.ProviderConfig's JSON shape so the composition root can pass
// it to the generated Identity config; the Identity package owns semantic
// validation (realm/protocol rules, URL scheme, duplicate ids and clock skew).
type IdentityProvider struct {
	ID               string   `json:"id"`
	Protocol         string   `json:"protocol"`
	Realm            string   `json:"realm"`
	Enabled          *bool    `json:"enabled,omitempty"`
	IssuerURL        string   `json:"issuerURL"`
	ClientID         string   `json:"clientID"`
	ClientSecret     string   `json:"clientSecret"`
	RedirectURL      string   `json:"redirectURL"`
	LinkRedirectURL  string   `json:"linkRedirectURL,omitempty"`
	MetadataURL      string   `json:"metadataURL,omitempty"`
	EntityID         string   `json:"entityID,omitempty"`
	ACSURL           string   `json:"acsURL,omitempty"`
	SPPrivateKey     string   `json:"spPrivateKey,omitempty"`
	SPCertificate    string   `json:"spCertificate,omitempty"`
	EmailAttribute   string   `json:"emailAttribute,omitempty"`
	ClockSkewSeconds int      `json:"clockSkewSeconds"`
	TrustedMFAAMR    []string `json:"trustedMFAAMR,omitempty"`
}

type Config struct {
	Env         string // development | production
	HTTPAddr    string
	DatabaseURL string
	SecretsKey  string // base64, 32 bytes: seals secrets stored in the database (TOTP)
	// StorageLocalRoot is the directory of the local storage provider (development only, 11.15).
	StorageLocalRoot string
	// StorageProvider is "local" (development), "oss" (Alibaba Cloud OSS) or "s3-compatible" (AWS S3, MinIO,
	// ...); each provider's settings apply to that provider only.
	StorageProvider string
	OSSEndpoint     string
	OSSRegion       string
	OSSBucket       string
	OSSKeyID        string
	OSSKeyValue     string
	S3Endpoint      string
	S3Region        string
	S3Bucket        string
	S3KeyID         string
	S3KeyValue      string
	// S3ForcePathStyle selects endpoint/bucket/key addressing (the default) over bucket.endpoint/key.
	S3ForcePathStyle bool
	// Notification: the provider is "disabled" or "smtp" (11.8). The relay settings apply to smtp only.
	NotificationProvider string
	SMTPHost             string
	SMTPPort             int
	SMTPFrom             string
	SMTPUser             string
	SMTPPassword         string
	// WebPublicURL and AdminPublicURL are the public base URLs used to build links in emails (11.8). They come from
	// configuration only, never from request headers.
	WebPublicURL   string
	AdminPublicURL string
	// Private license verification: trusted Ed25519 public keys by key id, JSON
	// encoded, plus the deployment identity the license must name. Empty keys
	// disable license import.
	LicenseVerificationKeys string
	LicenseProductVersion   string
	LicenseInstanceID       string
	// MeterWindowSeconds enables Metering buckets; zero leaves metering off so
	// the minimal quota checks behave exactly as before.
	MeterWindowSeconds        int
	LicenseRenewalWarningDays int
	// PaymentProvider selects the billing adapter (decision 3A: one market, a
	// replaceable provider). Empty or "disabled" keeps purchase unavailable while
	// everything else works (PRD 10.2); "test" is the signed test adapter.
	PaymentProvider string
	// BillingWebhookSecret authenticates test-provider callbacks; required only by
	// the test adapter.
	BillingWebhookSecret string
	// BillingGraceSeconds is how long a past-due subscription keeps working.
	BillingGraceSeconds int
	// IdentityProviders is the deployment-scoped OIDC registry, supplied as a
	// JSON array in IDENTITY_PROVIDERS. It is empty when OIDC is not configured;
	// a disabled entry is retained but not registered.
	IdentityProviders []IdentityProvider

	// Report archive phase 1: cloud OCR is optional and credentials stay server-side.
	ReportOCRProvider       string
	ReportOCREndpoint       string
	ReportOCRAPIKey         string
	ReportOCRModel          string
	ReportOCRTimeoutSeconds int
	ReportOCRMaxConcurrency int
	ReportPollMilliseconds  int
	ReportRetryBaseSeconds  int
	ReportMaxUploadBytes    int64
	ReportMaxUploadFiles    int
	ReportMaxPDFPages       int
}

func Load() (Config, error) {
	c := Config{
		Env:                     get("APP_ENV", "production"),
		HTTPAddr:                get("HTTP_ADDR", ":8080"),
		DatabaseURL:             os.Getenv("DATABASE_URL"),
		SecretsKey:              os.Getenv("SECRETS_KEY"),
		StorageLocalRoot:        get("STORAGE_LOCAL_ROOT", ".objects"),
		StorageProvider:         get("STORAGE_PROVIDER", "local"),
		OSSEndpoint:             os.Getenv("OSS_ENDPOINT"),
		OSSRegion:               os.Getenv("OSS_REGION"),
		OSSBucket:               os.Getenv("OSS_BUCKET"),
		OSSKeyID:                os.Getenv("OSS_ACCESS_KEY_ID"),
		OSSKeyValue:             os.Getenv("OSS_ACCESS_KEY_SECRET"),
		S3Endpoint:              os.Getenv("S3_ENDPOINT"),
		S3Region:                os.Getenv("S3_REGION"),
		S3Bucket:                os.Getenv("S3_BUCKET"),
		S3KeyID:                 os.Getenv("S3_ACCESS_KEY_ID"),
		S3KeyValue:              os.Getenv("S3_ACCESS_KEY_SECRET"),
		NotificationProvider:    get("NOTIFICATION_PROVIDER", "disabled"),
		PaymentProvider:         os.Getenv("PAYMENT_PROVIDER"),
		BillingWebhookSecret:    os.Getenv("BILLING_WEBHOOK_SECRET"),
		SMTPHost:                os.Getenv("SMTP_HOST"),
		SMTPFrom:                os.Getenv("SMTP_FROM"),
		SMTPUser:                os.Getenv("SMTP_USER"),
		SMTPPassword:            os.Getenv("SMTP_PASSWORD"),
		WebPublicURL:            os.Getenv("WEB_PUBLIC_URL"),
		AdminPublicURL:          os.Getenv("ADMIN_PUBLIC_URL"),
		LicenseVerificationKeys: os.Getenv("LICENSE_VERIFICATION_KEYS"),
		LicenseProductVersion:   os.Getenv("LICENSE_PRODUCT_VERSION"),
		LicenseInstanceID:       os.Getenv("LICENSE_INSTANCE_ID"),
		ReportOCRProvider:       get("REPORT_OCR_PROVIDER", "disabled"),
		ReportOCREndpoint:       os.Getenv("REPORT_OCR_ENDPOINT"),
		ReportOCRAPIKey:         os.Getenv("REPORT_OCR_API_KEY"),
		ReportOCRModel:          os.Getenv("REPORT_OCR_MODEL"),
		ReportOCRTimeoutSeconds: 90,
		ReportOCRMaxConcurrency: 2,
		ReportPollMilliseconds:  1000,
		ReportRetryBaseSeconds:  5,
		ReportMaxUploadBytes:    50_000_000,
		ReportMaxUploadFiles:    50,
		ReportMaxPDFPages:       100,
	}
	// Provider bindings are a typed deployment input. Decode with unknown fields
	// rejected so a per-tenant IdP (tenantID) or a typo cannot be ignored; the
	// Identity provider registry applies the protocol/URL/duplicate rules when
	// the API service is constructed.
	if raw := strings.TrimSpace(os.Getenv("IDENTITY_PROVIDERS")); raw != "" {
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c.IdentityProviders); err != nil {
			return c, fmt.Errorf("IDENTITY_PROVIDERS must be a JSON array of OIDC provider objects: %w", err)
		}
	}
	port, err := strconv.Atoi(get("SMTP_PORT", "587"))
	if err != nil {
		return c, fmt.Errorf("SMTP_PORT must be a number")
	}
	c.SMTPPort = port
	// Path-style addressing is the norm for MinIO and private deployments, so it is the default.
	pathStyle, err := strconv.ParseBool(get("S3_FORCE_PATH_STYLE", "true"))
	if err != nil {
		return c, fmt.Errorf("S3_FORCE_PATH_STYLE must be a boolean")
	}
	c.S3ForcePathStyle = pathStyle
	// License renewal reminders default to 30 days; deployments may widen or
	// narrow the window without changing code.
	c.LicenseRenewalWarningDays = 30
	if raw := get("LICENSE_RENEWAL_WARNING_DAYS", ""); raw != "" {
		days, err := strconv.Atoi(raw)
		if err != nil || days < 1 || days > 365 {
			return c, fmt.Errorf("LICENSE_RENEWAL_WARNING_DAYS must be between 1 and 365")
		}
		c.LicenseRenewalWarningDays = days
	}
	// A billing grace window keeps a past-due subscription working until it expires.
	if raw := get("BILLING_GRACE_SECONDS", ""); raw != "" {
		seconds, err := strconv.Atoi(raw)
		if err != nil || seconds < 0 || seconds > 2592000 {
			return c, fmt.Errorf("BILLING_GRACE_SECONDS must be between 0 and 2592000")
		}
		c.BillingGraceSeconds = seconds
	}
	// Metering stays off unless a whole-minute window is configured, so enabling
	// it cannot silently change quota behaviour.
	if raw := get("ENTITLEMENT_METER_WINDOW_SECONDS", ""); raw != "" {
		window, err := strconv.Atoi(raw)
		if err != nil || window < 60 || window > 604800 || window%60 != 0 {
			return c, fmt.Errorf("ENTITLEMENT_METER_WINDOW_SECONDS must be whole minutes between 60 and 604800")
		}
		c.MeterWindowSeconds = window
	}
	for name, target := range map[string]*int{
		"REPORT_OCR_TIMEOUT_SECONDS": &c.ReportOCRTimeoutSeconds,
		"REPORT_OCR_MAX_CONCURRENCY": &c.ReportOCRMaxConcurrency,
		"REPORT_WORKER_POLL_MS":      &c.ReportPollMilliseconds,
		"REPORT_RETRY_BASE_SECONDS":  &c.ReportRetryBaseSeconds,
		"REPORT_MAX_UPLOAD_FILES":    &c.ReportMaxUploadFiles,
		"REPORT_MAX_PDF_PAGES":       &c.ReportMaxPDFPages,
	} {
		if raw := os.Getenv(name); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return c, fmt.Errorf("%s must be a number", name)
			}
			*target = n
		}
	}
	if raw := os.Getenv("REPORT_MAX_UPLOAD_BYTES"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return c, fmt.Errorf("REPORT_MAX_UPLOAD_BYTES must be a number")
		}
		c.ReportMaxUploadBytes = n
	}
	switch c.ReportOCRProvider {
	case "disabled":
	case "ocrspace":
		if strings.TrimSpace(c.ReportOCRAPIKey) == "" {
			return c, fmt.Errorf("REPORT_OCR_PROVIDER=ocrspace requires REPORT_OCR_API_KEY")
		}
	default:
		return c, fmt.Errorf("REPORT_OCR_PROVIDER must be disabled or ocrspace")
	}
	if c.ReportOCRTimeoutSeconds < 1 || c.ReportOCRTimeoutSeconds > 600 || c.ReportOCRMaxConcurrency < 1 || c.ReportOCRMaxConcurrency > 16 ||
		c.ReportPollMilliseconds < 100 || c.ReportPollMilliseconds > 60000 || c.ReportRetryBaseSeconds < 1 || c.ReportRetryBaseSeconds > 3600 ||
		c.ReportMaxUploadBytes < 1 || c.ReportMaxUploadFiles < 1 || c.ReportMaxUploadFiles > 50 || c.ReportMaxPDFPages < 1 {
		return c, fmt.Errorf("report archive limits are outside the supported range")
	}

	// Provider choices are validated at startup: a deployment that selects SMTP
	// or object storage but omits required settings must not start and discover
	// the gap on the first email or upload.
	switch c.StorageProvider {
	case "local":
	case "s3-compatible":
		if c.S3Endpoint == "" || c.S3Bucket == "" || c.S3Region == "" {
			return c, fmt.Errorf("STORAGE_PROVIDER=s3-compatible requires S3_ENDPOINT, S3_BUCKET and S3_REGION")
		}
	case "oss":
		if c.OSSEndpoint == "" || c.OSSBucket == "" || c.OSSKeyID == "" || c.OSSKeyValue == "" {
			return c, fmt.Errorf("STORAGE_PROVIDER=oss requires OSS_ENDPOINT, OSS_BUCKET, OSS_ACCESS_KEY_ID and OSS_ACCESS_KEY_SECRET")
		}
	default:
		return c, fmt.Errorf("STORAGE_PROVIDER must be local, s3-compatible or oss")
	}
	switch c.NotificationProvider {
	case "disabled":
	case "smtp":
		if c.SMTPHost == "" || c.SMTPFrom == "" {
			return c, fmt.Errorf("NOTIFICATION_PROVIDER=smtp requires SMTP_HOST and SMTP_FROM")
		}
		if c.SMTPPort < 1 || c.SMTPPort > 65535 {
			return c, fmt.Errorf("SMTP_PORT must be between 1 and 65535")
		}
	default:
		return c, fmt.Errorf("NOTIFICATION_PROVIDER must be disabled or smtp")
	}
	switch c.PaymentProvider {
	case "", "disabled":
	case "test":
		if len(c.BillingWebhookSecret) < 16 {
			return c, fmt.Errorf("PAYMENT_PROVIDER=test requires BILLING_WEBHOOK_SECRET of at least 16 characters")
		}
	default:
		return c, fmt.Errorf("PAYMENT_PROVIDER has no supported adapter; leave unset, disabled, or test")
	}
	if c.Env != "development" && c.Env != "production" {
		return c, fmt.Errorf("APP_ENV must be development or production, got %q", c.Env)
	}
	if c.Env == "production" && strings.TrimSpace(c.DatabaseURL) == "" {
		return c, fmt.Errorf("DATABASE_URL is required when APP_ENV=production")
	}
	if c.Env == "production" && strings.TrimSpace(c.SecretsKey) == "" {
		return c, fmt.Errorf("SECRETS_KEY is required when APP_ENV=production (32 random bytes, base64)")
	}
	if c.Env == "production" {
		// Links in emails carry one-time values, so they must never point at cleartext URLs (11.8).
		for name, v := range map[string]string{"WEB_PUBLIC_URL": c.WebPublicURL, "ADMIN_PUBLIC_URL": c.AdminPublicURL} {
			if v == "" {
				continue
			}
			u, err := url.Parse(v)
			if err != nil || u.Scheme != "https" || u.Host == "" {
				return c, fmt.Errorf("%s must be an https URL when APP_ENV=production", name)
			}
		}
	}
	return c, nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
