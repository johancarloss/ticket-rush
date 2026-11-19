// testusers creates the load-test accounts in Keycloak, logs them in to a tokens file, and removes them afterwards.
//
//	go run ./cmd/testusers create -buyers 2000 -scalpers 5
//	go run ./cmd/testusers tokens -out tokens.json
//	go run ./cmd/testusers delete
//
// Details in bots/README.md.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/johancarloss/ticket-rush/bots/internal/accounts"
	"github.com/johancarloss/ticket-rush/bots/internal/keycloak"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "create":
		err = runCreate(os.Args[2:])
	case "tokens":
		err = runTokens(os.Args[2:])
	case "delete":
		err = runDelete(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: testusers create|tokens|delete [flags]  (see bots/README.md)")
	os.Exit(2)
}

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	buyers := fs.Int("buyers", 2000, "number of fan accounts")
	scalpers := fs.Int("scalpers", 5, "number of scalper accounts")
	batch := fs.Int("batch", 500, "accounts per import request")
	fs.Parse(args)

	password, err := requireEnv("LOAD_USERS_PASSWORD")
	if err != nil {
		return err
	}
	ctx := context.Background()
	admin, err := adminClient()
	if err != nil {
		return err
	}

	users := accounts.Build(*buyers, *scalpers, password)
	started := time.Now()
	var added, skipped int
	for start := 0; start < len(users); start += *batch {
		end := min(start+*batch, len(users))
		result, err := admin.ImportUsers(ctx, users[start:end])
		if err != nil {
			return fmt.Errorf("importing accounts %d to %d: %w", start, end-1, err)
		}
		added += result.Added
		skipped += result.Skipped
		fmt.Printf("accounts %d-%d: %d created, %d already existed\n", start, end-1, result.Added, result.Skipped)
	}
	fmt.Printf("done in %s: %d created, %d already existed\n", time.Since(started).Round(time.Millisecond), added, skipped)
	return nil
}

func runTokens(args []string) error {
	fs := flag.NewFlagSet("tokens", flag.ExitOnError)
	buyers := fs.Int("buyers", 2000, "number of fan accounts to log in")
	scalpers := fs.Int("scalpers", 5, "number of scalper accounts to log in")
	out := fs.String("out", "tokens.json", "file to write the tokens to (kept out of Git)")
	concurrency := fs.Int("concurrency", 20, "logins in parallel")
	fs.Parse(args)

	password, err := requireEnv("LOAD_USERS_PASSWORD")
	if err != nil {
		return err
	}
	// Logs in through the public address, the same path a buyer's browser takes.
	public := keycloak.New(envOr("KEYCLOAK_URL", "http://localhost:8180/auth"))
	names := accounts.Usernames(*buyers, *scalpers)

	ctx := context.Background()
	started := time.Now()
	tokens := make([]keycloak.Token, len(names))
	errs := make([]error, len(names))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range *concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				tokens[i], errs[i] = public.Login(ctx, names[i], password)
			}
		}()
	}
	for i := range names {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("some logins failed: %w", err)
	}
	data, err := json.MarshalIndent(tokens, "", " ")
	if err != nil {
		return err
	}
	// Tokens are credentials: readable only by the owner.
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		return err
	}
	fmt.Printf("%d tokens written to %s in %s; access tokens expire at %s, the bots renew them with the refresh token\n",
		len(tokens), *out, time.Since(started).Round(time.Millisecond), tokens[0].ExpiresAt.Format(time.TimeOnly))
	return nil
}

func runDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	fs.Parse(args)

	ctx := context.Background()
	admin, err := adminClient()
	if err != nil {
		return err
	}
	total := 0
	for _, prefix := range []string{accounts.BuyerPrefix, accounts.ScalperPrefix} {
		n, err := admin.DeleteUsersWithPrefix(ctx, prefix)
		if err != nil {
			return err
		}
		total += n
	}
	fmt.Printf("%d load-test accounts removed\n", total)
	return nil
}

// adminClient uses the admin entrypoint, published only on 127.0.0.1 (docs/infra/KEYCLOAK.md).
func adminClient() (*keycloak.Admin, error) {
	user, err := requireEnv("KEYCLOAK_ADMIN_USER")
	if err != nil {
		return nil, err
	}
	password, err := requireEnv("KEYCLOAK_ADMIN_PASSWORD")
	if err != nil {
		return nil, err
	}
	return keycloak.New(envOr("KEYCLOAK_ADMIN_URL", "http://127.0.0.1:8185/auth")).Admin(user, password), nil
}

func requireEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is not set (load the .env first: set -a; . ./.env; set +a)", key)
	}
	return value, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
