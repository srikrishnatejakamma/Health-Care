package configuration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestProductionGuardsAndSampleSecret(t *testing.T) {
	for _, key := range []string{"AUTH_PUBLIC_KEY_FILE", "DATABASE_URL", "TLS_CERT_FILE", "TLS_KEY_FILE", "TLS_TERMINATED_AT_PROXY", "DEMO_SEED", "STORE", "APP_ENV", "WORKERS", "MEMORY_BUDGET_MB", "DB_MAX_CONNECTIONS", "WORKER_ENABLED"} {
		t.Setenv(key, "")
	}
	t.Setenv("AUTH_KEY", string(bytes.Repeat([]byte("x"), 32)))
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_ENV", "production")
	if _, err := Load(); err == nil {
		t.Fatal("production memory mode accepted")
	}
	t.Setenv("STORE", "postgres")
	t.Setenv("DATABASE_URL", "postgres://example")
	if _, err := Load(); err == nil {
		t.Fatal("production without transport protection accepted")
	}
	t.Setenv("TLS_TERMINATED_AT_PROXY", "true")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTH_KEY", "replace-with-a-random-secret-at-least-32-bytes-long")
	if _, err := Load(); err == nil {
		t.Fatal("known example secret accepted")
	}
}
func TestDotEnvPreservesExplicitEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("HEALTHCARE_TEST_VALUE=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HEALTHCARE_TEST_VALUE", "explicit")
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("HEALTHCARE_TEST_VALUE") != "explicit" {
		t.Fatal("explicit environment was overridden")
	}
}
