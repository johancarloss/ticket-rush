package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type tokenValidator struct {
	keys     *keySet
	issuer   string
	audience string
}

type buyer struct {
	ID    string
	Roles []string
}

type keycloakClaims struct {
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	jwt.RegisteredClaims
}

func (v *tokenValidator) validate(raw string) (buyer, error) {
	claims := &keycloakClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, v.signingKey,
		// Only RS256 is accepted; this blocks "alg: none" and HS256 signed with the public key.
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return buyer{}, err
	}
	if claims.Subject == "" {
		return buyer{}, errors.New("token without subject")
	}
	return buyer{ID: claims.Subject, Roles: claims.RealmAccess.Roles}, nil
}

func (v *tokenValidator) signingKey(token *jwt.Token) (any, error) {
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, errors.New("token without kid")
	}
	key, err := v.keys.key(kid)
	if err != nil {
		return nil, fmt.Errorf("signing key %q: %w", kid, err)
	}
	return key, nil
}

// denyReason turns a validation error into a short label for the logs.
func denyReason(err error) string {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return "token_expired"
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return "wrong_issuer"
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		return "wrong_audience"
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return "bad_signature"
	case errors.Is(err, jwt.ErrTokenMalformed):
		return "malformed_token"
	case errors.Is(err, errUnknownKey):
		return "unknown_key"
	case strings.Contains(err.Error(), "signing method"):
		return "bad_algorithm"
	default:
		return "invalid_token"
	}
}
