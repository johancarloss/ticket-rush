package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoginReturnsTokensAndExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/realms/ticket-rush/protocol/openid-connect/token" {
			t.Errorf("path = %s", r.URL.Path)
		}
		r.ParseForm()
		if r.Form.Get("client_id") != "ticket-rush-web" || r.Form.Get("grant_type") != "password" || r.Form.Get("username") != "loadbuyer0001" {
			t.Errorf("form = %v", r.Form)
		}
		w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":300}`))
	}))
	defer server.Close()

	before := time.Now()
	tok, err := New(server.URL+"/auth").Login(context.Background(), "loadbuyer0001", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "access" || tok.RefreshToken != "refresh" || tok.Username != "loadbuyer0001" {
		t.Fatalf("token = %+v", tok)
	}
	if tok.ExpiresAt.Before(before.Add(299*time.Second)) || tok.ExpiresAt.After(time.Now().Add(300*time.Second)) {
		t.Errorf("expires_at = %s, want about 5 minutes from now", tok.ExpiresAt)
	}
}

func TestLoginReportsKeycloakErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	_, err := New(server.URL).Login(context.Background(), "loadbuyer0001", "wrong")
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("err = %v, want the status and Keycloak's message", err)
	}
}

// adminLogin answers the master realm token endpoint the way Keycloak does, with a one-minute token.
func adminLogin(w http.ResponseWriter, r *http.Request) bool {
	if !strings.HasSuffix(r.URL.Path, "/realms/master/protocol/openid-connect/token") {
		return false
	}
	w.Write([]byte(`{"access_token":"admin-token","expires_in":60}`))
	return true
}

func TestImportUsersSkipsExisting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if adminLogin(w, r) {
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/auth/admin/realms/ticket-rush/partialImport" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer admin-token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body struct {
			IfResourceExists string `json:"ifResourceExists"`
			Users            []User `json:"users"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.IfResourceExists != "SKIP" || len(body.Users) != 2 {
			t.Errorf("body = %+v", body)
		}
		w.Write([]byte(`{"added":1,"skipped":1}`))
	}))
	defer server.Close()

	result, err := New(server.URL+"/auth").Admin("admin", "pw").ImportUsers(context.Background(), []User{{Username: "a"}, {Username: "b"}})
	if err != nil || result.Added != 1 || result.Skipped != 1 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func TestDeleteUsersWithPrefixKeepsOtherAccounts(t *testing.T) {
	var mu sync.Mutex
	users := map[string]string{"1": "loadbuyer0001", "2": "loadbuyer0002", "3": "buyer1", "4": "xloadbuyer"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if adminLogin(w, r) {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			// Like Keycloak's search, match anywhere in the name.
			var found []foundUser
			for id, name := range users {
				if strings.Contains(name, r.URL.Query().Get("search")) {
					found = append(found, foundUser{ID: id, Username: name})
				}
			}
			json.NewEncoder(w).Encode(found)
		case http.MethodDelete:
			delete(users, strings.TrimPrefix(r.URL.Path, "/admin/realms/ticket-rush/users/"))
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	n, err := New(server.URL).Admin("admin", "pw").DeleteUsersWithPrefix(context.Background(), "loadbuyer")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(users) != 2 || users["3"] != "buyer1" || users["4"] != "xloadbuyer" {
		t.Fatalf("deleted %d, left %v; want only the two loadbuyer accounts removed", n, users)
	}
}

func TestAdminRenewsTokenBeforeItExpires(t *testing.T) {
	var mu sync.Mutex
	logins := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/realms/master/protocol/openid-connect/token") {
			mu.Lock()
			logins++
			mu.Unlock()
			// Already inside the renewal margin, as if the import had taken almost the whole minute.
			w.Write([]byte(`{"access_token":"admin-token","expires_in":10}`))
			return
		}
		w.Write([]byte(`{"added":0,"skipped":0}`))
	}))
	defer server.Close()

	admin := New(server.URL).Admin("admin", "pw")
	for range 3 {
		if _, err := admin.ImportUsers(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
	}
	if logins != 3 {
		t.Fatalf("logins = %d, want 3: a token about to expire must be renewed before each call", logins)
	}
}
