// Package bootstrap is the Composition Root: it owns process wiring. The generated/ subdirectory is
// written by `go tool saasctl generate`; everything else here belongs to the Product.
package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"saas-tic-tools/apps/api/internal/bootstrap/generated"
	"saas-tic-tools/apps/api/internal/config"
	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/0xunknownz/saas.starter/foundation/lifecycle"
	"github.com/0xunknownz/saas.starter/foundation/plugin"
	apihttp "github.com/0xunknownz/saas.starter/foundation/http"
)

// App is a fully wired, not yet started process.
type App struct {
	Handler   http.Handler
	Lifecycle *lifecycle.Manager
}

// New builds the composition. It creates services but starts no background work; call
// Lifecycle.Start next.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	lc := &lifecycle.Manager{}
	// Per-plugin settings, keyed by canonical plugin id, only for plugins in the composition.
	sections := map[string]json.RawMessage{}
	if generated.Plugins["foundation.identity"] {
		sections["foundation.identity"], _ = json.Marshal(map[string]any{"secretKey": cfg.SecretsKey, "secureCookies": cfg.Env == "production",
			"selfSignup": envBool("TENANT_SELF_SIGNUP", generated.Product.SelfSignup), "emailEnabled": cfg.NotificationProvider != "disabled",
			// The Admin session context reports the module composition and deployment mode (FAS-005, 4.5).
			"modules": pluginIDs(generated.Plugins), "productPermissions": productPlatformPermissions(generated.Permissions), "deploymentMode": cfg.Env,
			"storageProvider": cfg.StorageProvider, "notificationProvider": cfg.NotificationProvider,
			"paymentProvider": cfg.PaymentProvider,
			// Deployment-scoped provider bindings. Identity validates the typed
			// registry at construction; stage 1 starts no network flow.
			"identityProviders": cfg.IdentityProviders})
	}
	if generated.Plugins["foundation.tenant"] {
		sections["foundation.tenant"], _ = json.Marshal(map[string]any{"selfServiceCreate": envBool("TENANT_SELF_SERVICE_CREATE", generated.Product.SelfServiceCreate), "invitationEmailMatch": generated.Product.InvitationEmailMatch})
	}
	if generated.Plugins["foundation.notification"] {
		sections["foundation.notification"], _ = json.Marshal(map[string]any{"provider": cfg.NotificationProvider, "secretKey": cfg.SecretsKey, "smtpHost": cfg.SMTPHost, "smtpPort": cfg.SMTPPort, "smtpFrom": cfg.SMTPFrom, "smtpUser": cfg.SMTPUser, "smtpPassword": cfg.SMTPPassword, "webUrl": cfg.WebPublicURL, "adminUrl": cfg.AdminPublicURL})
	}
	if generated.Plugins["foundation.entitlement"] {
		section := map[string]any{}
		if cfg.MeterWindowSeconds > 0 {
			section["meterWindowSeconds"] = cfg.MeterWindowSeconds
		}
		sections["foundation.entitlement"], _ = json.Marshal(section)
	}
	if generated.Plugins["foundation.license"] {
		// Trusted verification keys come from the deployment, never from the
		// license file itself. An empty map disables import; malformed JSON is a
		// startup error rather than a silently disabled check.
		keys := map[string]string{}
		if raw := strings.TrimSpace(cfg.LicenseVerificationKeys); raw != "" {
			if err := json.Unmarshal([]byte(raw), &keys); err != nil {
				return nil, fmt.Errorf("LICENSE_VERIFICATION_KEYS must be a JSON object mapping key id to PEM: %w", err)
			}
		}
		sections["foundation.license"], _ = json.Marshal(map[string]any{
			"verificationKeys":   keys,
			"productId":          generated.Product.ID,
			"instanceId":         cfg.LicenseInstanceID,
			"version":            cfg.LicenseProductVersion,
			"renewalWarningDays": cfg.LicenseRenewalWarningDays,
		})
	}
	if generated.Plugins["foundation.billing"] {
		provider := cfg.PaymentProvider
		if provider == "disabled" {
			provider = "none"
		}
		sections["foundation.billing"], _ = json.Marshal(map[string]any{"payment": map[string]any{
			"provider": provider, "webhookSecret": cfg.BillingWebhookSecret, "graceSeconds": cfg.BillingGraceSeconds,
		}})
	}
	if generated.Plugins["foundation.storage"] {
		sections["foundation.storage"], _ = json.Marshal(map[string]any{"provider": cfg.StorageProvider, "localRoot": cfg.StorageLocalRoot, "appEnv": cfg.Env, "signingKey": cfg.SecretsKey,
			"ossEndpoint": cfg.OSSEndpoint, "ossRegion": cfg.OSSRegion, "ossBucket": cfg.OSSBucket, "ossKeyId": cfg.OSSKeyID, "ossKeyValue": cfg.OSSKeyValue,
			"s3Endpoint": cfg.S3Endpoint, "s3Region": cfg.S3Region, "s3Bucket": cfg.S3Bucket, "s3KeyId": cfg.S3KeyID,
			"s3KeyValue": cfg.S3KeyValue, "s3ForcePathStyle": cfg.S3ForcePathStyle})
	}
	pcfg, err := generated.LoadConfig(sections)
	if err != nil {
		return nil, err
	}
	// The database plugin owns the pool, readiness and the "database is migrated" check (10).
	pcfg.Db.URL = cfg.DatabaseURL
	for _, m := range generated.RequiredMigrations {
		pcfg.Db.RequiredMigrations = append(pcfg.Db.RequiredMigrations, db.Migration(m))
	}
	r := &router{mux: http.NewServeMux()}
	if err := generated.Build(ctx, pcfg, plugin.Runtime{Router: r, Lifecycle: lc}); err != nil {
		return nil, err
	}
	if r.err != nil {
		return nil, r.err
	}
	return &App{Handler: apihttp.New(lc.Ready, r.mux), Lifecycle: lc}, nil
}

// Shutdown follows technical design 6.3: revoke readiness, stop accepting requests and wait for
// in-flight ones, then stop components in reverse order. ctx bounds the whole sequence.
func (a *App) Shutdown(ctx context.Context, srv *http.Server) error {
	a.Lifecycle.Revoke()
	var errs []error
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		errs = append(errs, err)
	}
	return errors.Join(append(errs, a.Lifecycle.Stop(ctx))...)
}

// router turns the ServeMux duplicate-route panic into a startup error (6.2.2).
type router struct {
	mux *http.ServeMux
	err error
}

func (r *router) Handle(pattern string, h http.Handler) {
	defer func() {
		if p := recover(); p != nil {
			r.err = errors.Join(r.err, fmt.Errorf("register route %q: %v", pattern, p))
		}
	}()
	r.mux.Handle(pattern, h)
}

// pluginIDs returns the canonical plugin ids of the composition, sorted, for the Admin session context.
func pluginIDs(plugins map[string]bool) []string {
	out := make([]string, 0, len(plugins))
	for id := range plugins {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// productPlatformPermissions comes only from the statically generated, validated plugin registry.
// Foundation role definitions stay immutable; ops_admin receives no Product permission.
func productPlatformPermissions(perms []plugin.Permission) []string {
	var out []string
	for _, p := range perms {
		if strings.HasPrefix(p.Plugin, "product.") && strings.HasPrefix(p.Key, "platform.product.") {
			out = append(out, p.Key)
		}
	}
	sort.Strings(out)
	return out
}

// envBool lets a deployment override a product.yaml switch with "true" or "false"; anything else keeps the declared value.
func envBool(name string, declared bool) bool {
	switch os.Getenv(name) {
	case "true":
		return true
	case "false":
		return false
	}
	return declared
}
