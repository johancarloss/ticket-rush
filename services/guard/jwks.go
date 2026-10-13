package main

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// keySet keeps Keycloak's public keys in memory. Tokens are verified locally, so Keycloak is only
// contacted when a token arrives signed with a key we do not know yet (first use or key rotation).
type keySet struct {
	url         string
	client      *http.Client
	minInterval time.Duration

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	lastFetch time.Time
}

func newKeySet(url string) *keySet {
	return &keySet{
		url:         url,
		client:      &http.Client{Timeout: 5 * time.Second},
		minInterval: 30 * time.Second,
		keys:        map[string]*rsa.PublicKey{},
	}
}

var errUnknownKey = errors.New("unknown signing key")

func (ks *keySet) key(kid string) (*rsa.PublicKey, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if key, ok := ks.keys[kid]; ok {
		return key, nil
	}
	// Limits refetches so a flood of tokens with made-up key IDs cannot hammer Keycloak.
	if time.Since(ks.lastFetch) < ks.minInterval {
		return nil, errUnknownKey
	}
	if err := ks.refresh(); err != nil {
		return nil, err
	}
	if key, ok := ks.keys[kid]; ok {
		return key, nil
	}
	return nil, errUnknownKey
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (ks *keySet) refresh() error {
	ks.lastFetch = time.Now()

	resp, err := ks.client.Get(ks.url)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: status %d", resp.StatusCode)
	}

	var body struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := map[string]*rsa.PublicKey{}
	for _, k := range body.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		key, err := rsaKey(k.N, k.E)
		if err != nil {
			return fmt.Errorf("key %s: %w", k.Kid, err)
		}
		keys[k.Kid] = key
	}
	ks.keys = keys
	return nil
}

func rsaKey(n, e string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return nil, err
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
