package config

import (
	"strings"
	"testing"
)

func TestProductionRefusesCleartextPublicURLs(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://u@h/db")
	t.Setenv("SECRETS_KEY", "MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA=")
	t.Setenv("WEB_PUBLIC_URL", "http://app.example.com")
	if _, err := Load(); err == nil {
		t.Fatal("an http WEB_PUBLIC_URL must be refused in production")
	}
	t.Setenv("WEB_PUBLIC_URL", "https://app.example.com")
	t.Setenv("ADMIN_PUBLIC_URL", "https://admin.example.com")
	if _, err := Load(); err != nil {
		t.Fatalf("https URLs must be accepted: %v", err)
	}
}

// Provider selections are validated at startup, so a missing endpoint surfaces
// before the first email or upload instead of at request time.
func TestProviderConfigurationIsValidated(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("STORAGE_PROVIDER", "s3-compatible")
	if _, err := Load(); err == nil {
		t.Fatal("s3-compatible without endpoint/bucket must fail")
	}
	t.Setenv("S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("S3_REGION", "private")
	t.Setenv("S3_BUCKET", "files")
	if _, err := Load(); err != nil {
		t.Fatalf("complete s3-compatible config rejected: %v", err)
	}
	t.Setenv("STORAGE_PROVIDER", "ftp")
	if _, err := Load(); err == nil {
		t.Fatal("unknown storage provider accepted")
	}
	t.Setenv("STORAGE_PROVIDER", "local")
	t.Setenv("NOTIFICATION_PROVIDER", "smtp")
	if _, err := Load(); err == nil {
		t.Fatal("smtp without host/from must fail")
	}
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "noreply@example.com")
	if c, err := Load(); err != nil || c.NotificationProvider != "smtp" {
		t.Fatalf("complete smtp config rejected: %v", err)
	}
	t.Setenv("SMTP_PORT", "0")
	if _, err := Load(); err == nil {
		t.Fatal("invalid SMTP_PORT accepted")
	}
}
func TestDevelopmentAllowsLocalHTTPURLs(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("WEB_PUBLIC_URL", "http://localhost:3000")
	if _, err := Load(); err != nil {
		t.Fatalf("development may use http: %v", err)
	}
}

// 11.15: the s3-compatible provider reads its endpoint and credentials from the environment; path-style
// addressing is the default because MinIO and most private deployments use it.
func TestS3EnvironmentWiring(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("S3_REGION", "us-east-1")
	t.Setenv("S3_BUCKET", "files")
	t.Setenv("S3_ACCESS_KEY_ID", "key-id")
	t.Setenv("S3_ACCESS_KEY_SECRET", "key-secret")
	t.Setenv("S3_FORCE_PATH_STYLE", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.S3Endpoint != "http://127.0.0.1:9000" || c.S3Region != "us-east-1" || c.S3Bucket != "files" ||
		c.S3KeyID != "key-id" || c.S3KeyValue != "key-secret" {
		t.Fatalf("S3 settings: %+v", c)
	}
	if !c.S3ForcePathStyle {
		t.Fatal("S3_FORCE_PATH_STYLE must default to true")
	}
	t.Setenv("S3_FORCE_PATH_STYLE", "false")
	if c, err := Load(); err != nil || c.S3ForcePathStyle {
		t.Fatalf("S3_FORCE_PATH_STYLE=false: %+v %v", c, err)
	}
	t.Setenv("S3_FORCE_PATH_STYLE", "sometimes")
	if _, err := Load(); err == nil {
		t.Fatal("an invalid S3_FORCE_PATH_STYLE must be refused")
	}
}

func TestUnsupportedProviderInputsAreNotExposed(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("STORAGE_PROVIDER", "local")
	t.Setenv("NOTIFICATION_PROVIDER", "disabled")
	t.Setenv("PAYMENT_PROVIDER", "")
	marker := "opaque-provider-config-marker"
	for _, key := range []string{"STORAGE_PROVIDER", "NOTIFICATION_PROVIDER", "PAYMENT_PROVIDER"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, marker)
			_, err := Load()
			if err == nil || strings.Contains(err.Error(), marker) {
				t.Fatal("unsupported provider accepted or disclosed")
			}
		})
	}
	t.Setenv("PAYMENT_PROVIDER", "disabled")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}

// Identity provider bindings are deployment input, decoded once at startup. The
// Identity registry validates protocol/URL/duplicate rules; here we only prove
// the generated API config carries the typed values and rejects unknown keys.
func TestIdentityProviderConfigIsDecoded(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("IDENTITY_PROVIDERS", `[{"id":"corp-oidc","protocol":"oidc","realm":"tenant","enabled":false,"clockSkewSeconds":60}]`)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.IdentityProviders) != 1 {
		t.Fatalf("providers: %+v", c.IdentityProviders)
	}
	p := c.IdentityProviders[0]
	if p.ID != "corp-oidc" || p.Protocol != "oidc" || p.Realm != "tenant" || p.ClockSkewSeconds != 60 {
		t.Fatalf("provider: %+v", p)
	}
	if p.Enabled == nil || *p.Enabled {
		t.Fatalf("enabled flag: %+v", p.Enabled)
	}
}

func TestIdentityProviderConfigRejectsUnknownFields(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("IDENTITY_PROVIDERS", `[{"id":"corp-oidc","protocol":"oidc","realm":"tenant","tenantID":"tenant-1"}]`)
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "tenantID") {
		t.Fatalf("per-tenant provider field accepted or misreported: %v", err)
	}
}

func TestIdentityProviderConfigRejectsMalformedJSON(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("IDENTITY_PROVIDERS", `not-json`)
	if _, err := Load(); err == nil {
		t.Fatal("malformed IDENTITY_PROVIDERS accepted")
	}
}

func TestReportOCRProviderConfiguration(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("REPORT_OCR_PROVIDER", "paddleocr")
	t.Setenv("REPORT_OCR_API_KEY", "")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "paddleocr") {
		t.Fatalf("paddleocr without token accepted or misreported: %v", err)
	}
	t.Setenv("REPORT_OCR_API_KEY", "token-from-deployment")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ReportOCRProvider != "paddleocr" || c.ReportOCRAPIKey != "token-from-deployment" {
		t.Fatalf("report OCR config: %+v", c)
	}
	t.Setenv("REPORT_OCR_PROVIDER", "unknown-provider")
	if _, err := Load(); err == nil {
		t.Fatal("unknown report OCR provider accepted")
	}
}
