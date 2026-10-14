package kafka

import "github.com/golang-jwt/jwt/v5"

func registered(user string) jwt.RegisteredClaims { return jwt.RegisteredClaims{Subject: user} }
