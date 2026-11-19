// Package keycloak talks to the two Keycloak APIs the load tests need: the admin API, to create and remove the test
// accounts, and the token endpoint of the ticket-rush realm, to log those accounts in like a real client would.
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Realm = "ticket-rush"

// Client holds the base URL up to /auth, for example http://127.0.0.1:8185/auth.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 60 * time.Second}}
}

// Token is what the password grant returns, with the moment the access token expires.
type Token struct {
	Username     string    `json:"username"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// Admin calls the admin API as the Keycloak administrator. Its token, from the master realm, lasts only a minute,
// and importing a batch of accounts can take longer than that, so it is renewed before each call when close to expiring.
type Admin struct {
	client   *Client
	user     string
	password string
	token    Token
}

func (c *Client) Admin(user, password string) *Admin {
	return &Admin{client: c, user: user, password: password}
}

func (a *Admin) bearer(ctx context.Context) (string, error) {
	if time.Until(a.token.ExpiresAt) > 15*time.Second {
		return a.token.AccessToken, nil
	}
	tok, err := a.client.passwordGrant(ctx, "master", "admin-cli", a.user, a.password)
	if err != nil {
		return "", fmt.Errorf("admin login: %w", err)
	}
	a.token = tok
	return tok.AccessToken, nil
}

func (a *Admin) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	token, err := a.bearer(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, a.client.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}

// Login logs a buyer in on the ticket-rush realm through the same client the front end uses.
func (c *Client) Login(ctx context.Context, user, password string) (Token, error) {
	return c.passwordGrant(ctx, Realm, "ticket-rush-web", user, password)
}

func (c *Client) passwordGrant(ctx context.Context, realm, clientID, user, password string) (Token, error) {
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {user},
		"password":   {password},
	}
	endpoint := c.BaseURL + "/realms/" + realm + "/protocol/openid-connect/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var body tokenResponse
	if err := c.do(req, &body); err != nil {
		return Token{}, err
	}
	return Token{
		Username:     user,
		AccessToken:  body.AccessToken,
		RefreshToken: body.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(body.ExpiresIn) * time.Second),
	}, nil
}

// User is the part of Keycloak's user representation the test accounts need.
type User struct {
	Username      string       `json:"username"`
	Email         string       `json:"email"`
	FirstName     string       `json:"firstName"`
	LastName      string       `json:"lastName"`
	Enabled       bool         `json:"enabled"`
	EmailVerified bool         `json:"emailVerified"`
	RealmRoles    []string     `json:"realmRoles"`
	Credentials   []Credential `json:"credentials"`
}

type Credential struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

// ImportResult counts what a partial import did; accounts that already exist are skipped, so a rerun is safe.
type ImportResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"`
}

// ImportUsers creates many accounts in one request (partial import), much faster than one POST per user.
func (a *Admin) ImportUsers(ctx context.Context, users []User) (ImportResult, error) {
	payload, err := json.Marshal(map[string]any{"ifResourceExists": "SKIP", "users": users})
	if err != nil {
		return ImportResult{}, err
	}
	req, err := a.request(ctx, http.MethodPost, "/admin/realms/"+Realm+"/partialImport", bytes.NewReader(payload))
	if err != nil {
		return ImportResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	var result ImportResult
	return result, a.client.do(req, &result)
}

type foundUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// DeleteUsersWithPrefix removes every account whose username starts with prefix and returns how many it removed.
func (a *Admin) DeleteUsersWithPrefix(ctx context.Context, prefix string) (int, error) {
	deleted := 0
	for {
		search := "/admin/realms/" + Realm + "/users?max=500&briefRepresentation=true&search=" + url.QueryEscape(prefix)
		req, err := a.request(ctx, http.MethodGet, search, nil)
		if err != nil {
			return deleted, err
		}
		var found []foundUser
		if err := a.client.do(req, &found); err != nil {
			return deleted, err
		}

		removedNow := 0
		for _, u := range found {
			// The search also matches the prefix in the middle of a name; only delete real prefix matches.
			if !strings.HasPrefix(u.Username, prefix) {
				continue
			}
			req, err := a.request(ctx, http.MethodDelete, "/admin/realms/"+Realm+"/users/"+u.ID, nil)
			if err != nil {
				return deleted, err
			}
			if err := a.client.do(req, nil); err != nil {
				return deleted, err
			}
			deleted++
			removedNow++
		}
		if removedNow == 0 {
			return deleted, nil
		}
	}
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d %s", req.Method, req.URL.Path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}
