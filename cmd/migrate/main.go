package main

import (
	"context"
	"fmt"
	"healthcare/internal/framework/configuration"
	"healthcare/migrations"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func run() error {
	if err := configuration.LoadDotEnv(".env"); err != nil {
		return err
	}
	url := os.Getenv("MIGRATION_DATABASE_URL")
	if url == "" {
		return fmt.Errorf("MIGRATION_DATABASE_URL must use a migration owner, separate from DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(483290174)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, migrations.Initial); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Migrations applied")
}
