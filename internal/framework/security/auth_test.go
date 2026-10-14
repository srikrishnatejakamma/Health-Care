package security

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestClaimsAndAlgorithmValidation(t *testing.T) {
	a := &Auth{Key: bytes.Repeat([]byte("k"), 32), Issuer: "issuer", Audience: "api"}
	valid, err := a.Sign("user", "tenant", false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Validate(valid); err != nil {
		t.Fatal(err)
	}
	for name, claims := range map[string]jwt.RegisteredClaims{"wrong issuer": {Subject: "user", Issuer: "other", Audience: jwt.ClaimStrings{"api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}, "wrong audience": {Subject: "user", Issuer: "issuer", Audience: jwt.ClaimStrings{"other"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}, "expired": {Subject: "user", Issuer: "issuer", Audience: jwt.ClaimStrings{"api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour))}, "missing expiry": {Subject: "user", Issuer: "issuer", Audience: jwt.ClaimStrings{"api"}}} {
		t.Run(name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{RegisteredClaims: claims}).SignedString(a.Key)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = a.Validate(token); err == nil {
				t.Fatal("invalid claims accepted")
			}
		})
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS384, Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user", Issuer: "issuer", Audience: jwt.ClaimStrings{"api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}).SignedString(a.Key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Validate(token); err == nil {
		t.Fatal("unexpected signing algorithm accepted")
	}
}
func TestRS256AndHMACConfusion(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	a := &Auth{PublicKey: &key.PublicKey, Issuer: "issuer", Audience: "api"}
	claims := Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: "user", Issuer: "issuer", Audience: jwt.ClaimStrings{"api"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Validate(token); err != nil {
		t.Fatal(err)
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("pretend-public-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Validate(forged); err == nil {
		t.Fatal("HMAC token accepted by RSA validator")
	}
}
