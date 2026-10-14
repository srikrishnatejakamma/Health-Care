package main

import (
	"flag"
	"fmt"
	"healthcare/internal/bootstrap"
	"healthcare/internal/framework/configuration"
	"os"
	"time"
)

func run() error {
	subject := flag.String("subject", "", "user or service identity")
	tenant := flag.String("tenant", "", "tenant binding; leave empty for authorized tenant switching")
	platform := flag.Bool("platform-admin", false, "issue an operator token")
	ttl := flag.Duration("ttl", time.Hour, "token lifetime, maximum 24h")
	flag.Parse()
	if err := configuration.LoadDotEnv(".env"); err != nil {
		return err
	}
	c, err := configuration.Load()
	if err != nil {
		return err
	}
	if c.AuthPublicKeyFile != "" {
		return fmt.Errorf("RS256 mode accepts tokens from the configured identity provider; this local tool only signs HS256 tokens")
	}
	a, err := bootstrap.Auth(c, nil)
	if err != nil {
		return err
	}
	token, err := a.Sign(*subject, *tenant, *platform, *ttl)
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
