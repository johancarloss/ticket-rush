package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check /health on the local server and exit")
	flag.Parse()

	port := envOr("PORT", "8080")

	// The runtime image has no shell or curl, so the binary checks itself for the Docker healthcheck.
	if *healthcheck {
		os.Exit(runHealthcheck(port))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	shutdownTracing, err := setupTracing(context.Background())
	if err != nil {
		logger.Error("tracing setup failed", "error", err)
		os.Exit(1)
	}
	telemetry := newMetrics()

	limitPerSecond, err := strconv.ParseInt(envOr("RATE_LIMIT_PER_SECOND", "10"), 10, 64)
	if err != nil || limitPerSecond < 1 {
		logger.Error("invalid RATE_LIMIT_PER_SECOND", "value", os.Getenv("RATE_LIMIT_PER_SECOND"))
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: envOr("REDIS_ADDR", "redis:6379")})
	defer redisClient.Close()
	rateLimit := newFixedWindow(redisClient, limitPerSecond)

	validator := &tokenValidator{
		keys:     newKeySet(mustEnv("KEYCLOAK_JWKS_URL")),
		issuer:   mustEnv("TOKEN_ISSUER"),
		audience: mustEnv("TOKEN_AUDIENCE"),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("/verify", verifyHandler(logger, validator, rateLimit, telemetry))
	// Only reachable inside the Compose networks: no Traefik router points at the Guard.
	mux.Handle("GET /metrics", telemetry.handler())

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           traced(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("guard listening", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Finish in-flight requests before exiting, so a deploy does not cut requests in half.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
	if err := shutdownTracing(ctx); err != nil {
		logger.Error("tracing shutdown failed", "error", err)
	}
	logger.Info("guard stopped")
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// verifyHandler answers Traefik's forwardAuth: 2xx lets the request through, anything else is sent back to the client.
// On success it returns who the buyer is in X-User-Id and X-User-Roles, which Traefik forwards to the service.
func verifyHandler(logger *slog.Logger, validator *tokenValidator, rateLimit limiter, telemetry *metrics) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		method := r.Header.Get("X-Forwarded-Method")
		uri := r.Header.Get("X-Forwarded-Uri")

		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok || token == "" {
			logger.Info("request denied", "reason", "missing_token", "method", method, "uri", uri)
			telemetry.record("missing_token", started)
			writeJSONError(w, http.StatusUnauthorized, "missing or malformed bearer token")
			return
		}

		user, err := validator.validate(token)
		if err != nil {
			reason := denyReason(err)
			logger.Info("request denied", "reason", reason, "method", method, "uri", uri, "error", err.Error())
			telemetry.record(reason, started)
			writeJSONError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		// The limit is per account, not per IP, so it comes after the token says who the account is (ADR-002).
		allowed, err := rateLimit.allow(r.Context(), user.ID)
		if err != nil {
			// Fail-open: without Redis the token is still checked and the 4-seat limit still lives in the Checkout,
			// so blocking every purchase would cost more than letting a bot through for a while (docs/infra/GUARD.md).
			logger.Warn("rate limit unavailable, letting request through", "user_id", user.ID, "error", err.Error())
			telemetry.limiterErrors.Inc()
		} else if !allowed {
			logger.Info("request denied", "reason", "rate_limited", "user_id", user.ID, "method", method, "uri", uri)
			telemetry.record("rate_limited", started)
			w.Header().Set("Retry-After", "1")
			writeJSONError(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		logger.Info("request allowed", "user_id", user.ID, "method", method, "uri", uri)
		w.Header().Set("X-User-Id", user.ID)
		w.Header().Set("X-User-Roles", strings.Join(user.Roles, ","))
		w.WriteHeader(http.StatusOK)
		telemetry.record("allowed", started)
	}
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix)), true
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + message + `"}`))
}

func runHealthcheck(port string) int {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		fmt.Fprintf(os.Stderr, "missing required environment variable %s\n", key)
		os.Exit(1)
	}
	return value
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
