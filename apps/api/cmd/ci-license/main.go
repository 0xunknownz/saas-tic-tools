// ci-license prepares the ephemeral database used by the GitHub security job.
// It signs a short-lived fixture through the same importer used by real licences.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/0xunknownz/saas.starter/foundation/audit"
	"github.com/0xunknownz/saas.starter/foundation/db"
	"github.com/0xunknownz/saas.starter/foundation/license"
	"saas-tic-tools/apps/api/internal/bootstrap/generated"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := install(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func install(ctx context.Context) error {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("APP_ENV") != "development" {
		return fmt.Errorf("CI licence fixture requires GITHUB_ACTIONS=true and APP_ENV=development")
	}
	envPath := os.Getenv("GITHUB_ENV")
	if envPath == "" {
		return fmt.Errorf("GITHUB_ENV is required for the CI verification key")
	}
	database, err := db.NewService(db.Config{URL: os.Getenv("DATABASE_URL")}, db.Deps{})
	if err != nil {
		return err
	}
	if err := database.Start(ctx); err != nil {
		return err
	}
	defer database.Stop(context.Background())
	writer, err := audit.NewService(audit.Config{}, audit.Deps{Db: database})
	if err != nil {
		return err
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	const keyID = "ci-security"
	const instance = "ci-security-instance"
	const version = "1.0.0"
	config := license.ImportConfig{Keys: map[string]ed25519.PublicKey{keyID: public},
		Product: generated.Product.ID, Instance: instance, Version: version}
	usage, err := license.NewInstanceUsage(config)
	if err != nil {
		return err
	}
	importer, err := license.NewImporter(database, writer, usage, config)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	candidate := license.Candidate{ProductID: generated.Product.ID, InstanceID: instance,
		CustomerID: "ci-security-fixture", MinVersion: version, MaxVersion: version,
		IssuedAt: now.Add(-time.Minute), NotBefore: now.Add(-time.Minute),
		ExpiresAt: now.Add(24 * time.Hour), Features: []string{"advanced"},
		Limits: map[string]int64{"tenants": 1000, "users": 10000}}
	payload, err := json.Marshal(candidate)
	if err != nil {
		return err
	}
	envelope, err := json.Marshal(map[string]string{"keyId": keyID,
		"payload":   base64.StdEncoding.EncodeToString(payload),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))})
	if err != nil {
		return err
	}
	preview, err := importer.CreatePreview(ctx, envelope)
	if err != nil {
		return err
	}
	if _, err := importer.Apply(ctx, preview.ID, "ci-license-"+preview.ID, "ci:security", true); err != nil {
		return err
	}
	encoded, err := x509.MarshalPKIXPublicKey(public)
	if err != nil {
		return err
	}
	keys, err := json.Marshal(map[string]string{keyID: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded}))})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, "LICENSE_VERIFICATION_KEYS=%s\nLICENSE_INSTANCE_ID=%s\nLICENSE_PRODUCT_VERSION=%s\n", keys, instance, version); err != nil {
		return err
	}
	fmt.Printf("Installed signed CI licence through importer; expires %s. Private key retained only in memory.\n", candidate.ExpiresAt.Format(time.RFC3339))
	return nil
}
