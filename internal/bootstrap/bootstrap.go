package bootstrap

import (
	"context"
	"errors"
	"healthcare/internal/domain"
	"healthcare/internal/framework/configuration"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/security"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Store(ctx context.Context, c configuration.Config) (database.Store, error) {
	if c.Store == "memory" {
		return database.NewMemory(), nil
	}
	return database.OpenPostgres(ctx, c.DatabaseURL, c.MaxConnections)
}
func Auth(c configuration.Config, store database.Store) (*security.Auth, error) {
	a := &security.Auth{Key: []byte(c.AuthKey), Issuer: c.Issuer, Audience: c.Audience, Store: store}
	if c.AuthPublicKeyFile != "" {
		b, err := os.ReadFile(c.AuthPublicKeyFile)
		if err != nil {
			return nil, err
		}
		a.PublicKey, err = jwt.ParseRSAPublicKeyFromPEM(b)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}
func Seed(ctx context.Context, s database.Store) error {
	for _, v := range []struct{ ID, Name, User string }{{"north-health", "North Health Demo", "north-admin"}, {"south-health", "South Health Demo", "south-admin"}} {
		t := domain.Tenant{ID: v.ID, Name: v.Name, Status: "active", Timezone: "Asia/Kolkata", Region: "local", IsolationMode: "shared_schema", Version: 1, Quotas: domain.DefaultQuotas(), CreatedAt: time.Now().UTC()}
		m := domain.Membership{TenantID: t.ID, UserID: v.User, Roles: []string{"tenant_admin"}, Facilities: []string{"*"}, Active: true}
		if err := s.CreateTenant(ctx, t, m); err != nil && !errors.Is(err, domain.ErrConflict) {
			return err
		}
	}
	return nil
}
