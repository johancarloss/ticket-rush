package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
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

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("/verify", verifyHandler(logger))

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
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
	logger.Info("guard stopped")
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// verifyHandler answers Traefik's forwardAuth: 2xx lets the request through, anything else is sent back to the client.
// For now it only requires a Bearer token to be present; signature validation against Keycloak comes next.
func verifyHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		method := r.Header.Get("X-Forwarded-Method")
		uri := r.Header.Get("X-Forwarded-Uri")

		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok || token == "" {
			logger.Info("request denied", "reason", "missing_token", "method", method, "uri", uri)
			writeJSONError(w, http.StatusUnauthorized, "missing or malformed bearer token")
			return
		}

		logger.Info("request allowed", "method", method, "uri", uri)
		w.WriteHeader(http.StatusOK)
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

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
