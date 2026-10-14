package configuration

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address, Store, DatabaseURL, AuthKey, AuthPublicKeyFile, Issuer, Audience, Environment string
	Workers                                                                                int
	MemoryMB                                                                               int64
	MaxConnections                                                                         int32
	WorkerEnabled, DemoSeed                                                                bool
	TLSCert, TLSKey                                                                        string
	ShutdownTimeout                                                                        time.Duration
	GRPCAddress                                                                            string
}

// LoadDotEnv is intentionally literal: no variable interpolation or shell execution.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid environment file line")
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), "\"'")
		if k == "" {
			return errors.New("empty environment key")
		}
		if _, exists := os.LookupEnv(k); !exists {
			if err = os.Setenv(k, v); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func Load() (Config, error) {
	c := Config{Address: env("HTTP_ADDR", "127.0.0.1:18080"), Store: env("STORE", "memory"), DatabaseURL: os.Getenv("DATABASE_URL"), AuthKey: os.Getenv("AUTH_KEY"), AuthPublicKeyFile: os.Getenv("AUTH_PUBLIC_KEY_FILE"), Issuer: env("AUTH_ISSUER", "healthcare-local"), Audience: env("AUTH_AUDIENCE", "healthcare-api"), Environment: env("APP_ENV", "development"), TLSCert: os.Getenv("TLS_CERT_FILE"), TLSKey: os.Getenv("TLS_KEY_FILE"), ShutdownTimeout: 15 * time.Second}
	c.GRPCAddress = os.Getenv("GRPC_ADDR")
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return c, errors.New("APP_ENV must be development, test or production")
	}
	if strings.HasPrefix(c.AuthKey, "replace-with-") {
		return c, errors.New("replace the example AUTH_KEY with a randomly generated secret")
	}
	workers, err := strconv.Atoi(env("WORKERS", "4"))
	if err != nil || workers < 1 || workers > 64 {
		return c, errors.New("WORKERS must be between 1 and 64")
	}
	c.Workers = workers
	mb, err := strconv.ParseInt(env("MEMORY_BUDGET_MB", "256"), 10, 64)
	if err != nil || mb < 8 || mb > 65536 {
		return c, errors.New("MEMORY_BUDGET_MB must be between 8 and 65536")
	}
	c.MemoryMB = mb
	connections, err := strconv.Atoi(env("DB_MAX_CONNECTIONS", "16"))
	if err != nil || connections < 2 || connections > 128 {
		return c, errors.New("DB_MAX_CONNECTIONS must be between 2 and 128")
	}
	c.MaxConnections = int32(connections)
	c.WorkerEnabled, err = strconv.ParseBool(env("WORKER_ENABLED", "true"))
	if err != nil {
		return c, errors.New("invalid WORKER_ENABLED")
	}
	c.DemoSeed, err = strconv.ParseBool(env("DEMO_SEED", "false"))
	if err != nil {
		return c, errors.New("invalid DEMO_SEED")
	}
	if c.Store != "memory" && c.Store != "postgres" {
		return c, errors.New("STORE must be memory or postgres")
	}
	if c.Store == "postgres" && c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL is required")
	}
	if c.AuthPublicKeyFile == "" && len(c.AuthKey) < 32 {
		return c, errors.New("AUTH_KEY must contain at least 32 bytes, or configure AUTH_PUBLIC_KEY_FILE for RS256")
	}
	if (c.TLSCert == "") != (c.TLSKey == "") {
		return c, errors.New("TLS_CERT_FILE and TLS_KEY_FILE must be set together")
	}
	if c.Environment == "production" {
		if c.Store != "postgres" || c.DemoSeed {
			return c, errors.New("production requires PostgreSQL and DEMO_SEED=false")
		}
		if c.TLSCert == "" && os.Getenv("TLS_TERMINATED_AT_PROXY") != "true" {
			return c, errors.New("production requires TLS or TLS_TERMINATED_AT_PROXY=true")
		}
	}
	return c, nil
}
