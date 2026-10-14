package security

import (
	"context"
	"errors"
	"healthcare/internal/domain"
	"healthcare/internal/framework/database"
	"healthcare/internal/framework/tenancy"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct { TenantID string `json:"tenant_id,omitempty"`; PlatformAdmin bool `json:"platform_admin,omitempty"`;jwt.RegisteredClaims }
type Auth struct { Key []byte; Issuer string; Audience string;Store database.Store }
func(a *Auth)Validate(token string)(Claims,error){var c Claims;parsed,err:=jwt.ParseWithClaims(token,&c,func(t *jwt.Token)(any,error){return a.Key,nil},jwt.WithValidMethods([]string{"HS256"}),jwt.WithIssuer(a.Issuer),jwt.WithAudience(a.Audience),jwt.WithExpirationRequired(),jwt.WithIssuedAt());if err!=nil||!parsed.Valid||strings.TrimSpace(c.Subject)==""{return c,domain.ErrForbidden};return c,nil}
func(a *Auth)Resolve(ctx context.Context,c Claims,selected,correlation string)(context.Context,error){if c.TenantID!=""&&selected!=""&&c.TenantID!=selected{return ctx,domain.ErrForbidden};if selected==""{selected=c.TenantID};if selected==""{return ctx,domain.ErrForbidden};m,err:=a.Store.Membership(ctx,selected,c.Subject);if err!=nil||!m.Active{return ctx,domain.ErrForbidden};t,err:=a.Store.Tenant(ctx,selected);if err!=nil{return ctx,domain.ErrForbidden};if t.Status!="active"{return ctx,domain.ErrInactive};return tenancy.With(ctx,tenancy.Scope{TenantID:selected,Actor:c.Subject,Roles:m.Roles,Facilities:m.Facilities,CorrelationID:correlation}),nil}
// Token generation is an offline operator tool; the HTTP server never issues tokens.
func(a *Auth)Sign(subject,tenantID string,platform bool,ttl time.Duration)(string,error){if subject==""||ttl<=0||ttl>24*time.Hour{return "",errors.New("subject and a TTL between 0 and 24 hours are required")};now:=time.Now();c:=Claims{TenantID:tenantID,PlatformAdmin:platform,RegisteredClaims:jwt.RegisteredClaims{Subject:subject,Issuer:a.Issuer,Audience:jwt.ClaimStrings{a.Audience},IssuedAt:jwt.NewNumericDate(now),NotBefore:jwt.NewNumericDate(now),ExpiresAt:jwt.NewNumericDate(now.Add(ttl)),ID:domain.ID()}};return jwt.NewWithClaims(jwt.SigningMethodHS256,c).SignedString(a.Key)}
