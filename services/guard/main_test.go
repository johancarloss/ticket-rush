package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer   = "http://localhost:8180/auth/realms/ticket-rush"
	testAudience = "ticket-rush-api"
	testKid      = "test-key"
)

type fixture struct {
	key     *rsa.PrivateKey
	handler http.HandlerFunc
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": testKid,
			"kty": "RSA",
			"use": "sig",
			"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(jwksServer.Close)

	validator := &tokenValidator{keys: newKeySet(jwksServer.URL), issuer: testIssuer, audience: testAudience}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &fixture{key: key, handler: verifyHandler(logger, validator)}
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":          testIssuer,
		"aud":          testAudience,
		"sub":          "buyer-42",
		"exp":          time.Now().Add(5 * time.Minute).Unix(),
		"realm_access": map[string]any{"roles": []string{"buyer"}},
	}
}

func (f *fixture) sign(t *testing.T, claims jwt.MapClaims, kid string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(f.key)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func (f *fixture) verify(authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/verify", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	return rec
}

func TestVerifyAcceptsValidToken(t *testing.T) {
	f := newFixture(t)

	rec := f.verify("Bearer " + f.sign(t, validClaims(), testKid))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("X-User-Id"); got != "buyer-42" {
		t.Errorf("X-User-Id = %q, want buyer-42", got)
	}
	if got := rec.Header().Get("X-User-Roles"); got != "buyer" {
		t.Errorf("X-User-Roles = %q, want buyer", got)
	}
}

func TestVerifyRejects(t *testing.T) {
	f := newFixture(t)

	expired := validClaims()
	expired["exp"] = time.Now().Add(-time.Minute).Unix()
	otherIssuer := validClaims()
	otherIssuer["iss"] = "http://evil.example/realms/ticket-rush"
	otherAudience := validClaims()
	otherAudience["aud"] = "account"
	noExpiration := validClaims()
	delete(noExpiration, "exp")
	noSubject := validClaims()
	delete(noSubject, "sub")

	valid := f.sign(t, validClaims(), testKid)
	parts := strings.Split(valid, ".")
	forgedPayload, _ := json.Marshal(map[string]any{"iss": testIssuer, "aud": testAudience, "sub": "someone-else", "exp": time.Now().Add(time.Hour).Unix()})
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(forgedPayload) + "." + parts[2]

	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)

	// Classic algorithm confusion: HS256 using the public key bytes as the shared secret.
	publicDER, _ := x509.MarshalPKIXPublicKey(&f.key.PublicKey)
	hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
	hsToken.Header["kid"] = testKid
	confused, _ := hsToken.SignedString(publicDER)

	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	foreign := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims())
	foreign.Header["kid"] = testKid
	foreignSigned, _ := foreign.SignedString(otherKey)

	cases := []struct {
		name          string
		authorization string
	}{
		{"no header", ""},
		{"basic scheme", "Basic abc"},
		{"empty bearer", "Bearer "},
		{"garbage", "Bearer not-a-jwt"},
		{"expired", "Bearer " + f.sign(t, expired, testKid)},
		{"other issuer", "Bearer " + f.sign(t, otherIssuer, testKid)},
		{"other audience", "Bearer " + f.sign(t, otherAudience, testKid)},
		{"no expiration", "Bearer " + f.sign(t, noExpiration, testKid)},
		{"no subject", "Bearer " + f.sign(t, noSubject, testKid)},
		{"unknown kid", "Bearer " + f.sign(t, validClaims(), "other-kid")},
		{"tampered payload", "Bearer " + tampered},
		{"alg none", "Bearer " + unsigned},
		{"hs256 with public key", "Bearer " + confused},
		{"signed by another key", "Bearer " + foreignSigned},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.verify(tc.authorization)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if rec.Header().Get("X-User-Id") != "" {
				t.Error("X-User-Id set on a rejected request")
			}
		})
	}
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	handleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
