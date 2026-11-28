package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// Stream and group from docs/contracts/EVENTOS.md.
const (
	ticketsStream     = "tickets.issued"
	notificationGroup = "notification"
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

	redisClient := redis.NewClient(&redis.Options{Addr: envOr("REDIS_ADDR", "redis:6379")})
	defer redisClient.Close()

	// Each replica needs its own consumer name inside the group; the container hostname is unique per replica.
	consumerName, err := os.Hostname()
	if err != nil {
		logger.Error("hostname unavailable", "error", err)
		os.Exit(1)
	}
	tickets := newConsumer(redisClient, ticketsStream, notificationGroup, consumerName, logTicketIssued(logger), logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	// Only reachable inside the Compose networks: the service has no Traefik router.
	mux.Handle("GET /metrics", newMetrics().handler())

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("notification listening", "port", port)
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
	// The consumer finishes the message in hand before the process exits.
	<-consumed

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "error", err)
	}
	logger.Info("notification stopped")
}

// logTicketIssued is the handler of the skeleton: it validates the event and logs it. Sending the e-mail, with
// the check for orders already notified, comes with issue #41.
func logTicketIssued(logger *slog.Logger) handler {
	return func(_ context.Context, message redis.XMessage) error {
		payload, _ := message.Values["payload"].(string)
		event, err := parseTicketIssued(payload)
		if err != nil {
			return err
		}
		// The buyer's e-mail stays out of the log; the order ID is enough to find the purchase.
		logger.Info("ticket issued received",
			"message_id", message.ID,
			"event_message_id", event.MessageID,
			"order_id", event.OrderID,
			"event_id", event.Event.ID,
			"seats", len(event.Seats),
			"trace_id", event.TraceID,
		)
		return nil
	}
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

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
