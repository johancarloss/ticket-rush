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
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Stream and group from docs/contracts/EVENTOS.md.
const (
	ticketsStream = "tickets.issued"
	statsGroup    = "stats"
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

	pool, err := pgxpool.New(context.Background(), mustEnv("DATABASE_URL"))
	if err != nil {
		logger.Error("invalid DATABASE_URL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	sales := &store{pool: pool}
	migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), 30*time.Second)
	err = sales.migrate(migrateCtx)
	cancelMigrate()
	if err != nil {
		logger.Error("creating tables failed", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: envOr("REDIS_ADDR", "redis:6379")})
	defer redisClient.Close()

	// Each replica needs its own consumer name inside the group; the container hostname is unique per replica.
	consumerName, err := os.Hostname()
	if err != nil {
		logger.Error("hostname unavailable", "error", err)
		os.Exit(1)
	}
	tickets := newConsumer(redisClient, ticketsStream, statsGroup, consumerName, recordSaleHandler(logger, sales), sales.ping, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/stats/events/{event_id}", eventSalesHandler(logger, sales))
	// No Traefik router matches /metrics, so only the Compose networks reach it.
	mux.Handle("GET /metrics", newMetrics().handler())

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("stats listening", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	consumed := make(chan struct{})
	go func() {
		defer close(consumed)
		tickets.run(ctx)
	}()

	<-ctx.Done()

	// Stop taking requests and finish the messages already read, before the pool and Redis close.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
	<-consumed
	logger.Info("stats stopped")
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
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
