package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const testStream = "tickets.issued"

// recorder is a handler that remembers which messages it saw and fails the ones listed in failing.
type recorder struct {
	mu      sync.Mutex
	seen    []string
	failing map[string]bool
}

func (r *recorder) handle(_ context.Context, message redis.XMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	payload, _ := message.Values["payload"].(string)
	r.seen = append(r.seen, payload)
	if r.failing[payload] {
		return errors.New("simulated failure")
	}
	return nil
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.seen)
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	return client
}

// startConsumer runs a consumer in the background and returns a function that stops it and waits for it to return.
func startConsumer(t *testing.T, client *redis.Client, name string, r *recorder) func() {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := newConsumer(client, testStream, "notification", name, r.handle, logger)
	c.block = 50 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.run(ctx)
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("consumer did not stop after cancel")
		}
	}
}

func publish(t *testing.T, client *redis.Client, payload string) {
	t.Helper()
	if err := client.XAdd(context.Background(), &redis.XAddArgs{Stream: testStream, Values: map[string]any{"payload": payload}}).Err(); err != nil {
		t.Fatal(err)
	}
}

func pendingCount(t *testing.T, client *redis.Client) int64 {
	t.Helper()
	pending, err := client.XPending(context.Background(), testStream, "notification").Result()
	if err != nil {
		t.Fatal(err)
	}
	return pending.Count
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestProcessedMessageIsAcknowledged(t *testing.T) {
	client := newTestRedis(t)
	r := &recorder{}
	stop := startConsumer(t, client, "replica-1", r)
	defer stop()

	publish(t, client, "first")
	publish(t, client, "second")

	eventually(t, func() bool { return r.count() == 2 })
	eventually(t, func() bool { return pendingCount(t, client) == 0 })
}

func TestEventsPublishedBeforeTheGroupExistedAreProcessed(t *testing.T) {
	client := newTestRedis(t)
	publish(t, client, "published while the service was not running")

	r := &recorder{}
	stop := startConsumer(t, client, "replica-1", r)
	defer stop()

	eventually(t, func() bool { return r.count() == 1 })
}

func TestFailedMessageStaysPendingAndOthersContinue(t *testing.T) {
	client := newTestRedis(t)
	r := &recorder{failing: map[string]bool{"broken": true}}
	stop := startConsumer(t, client, "replica-1", r)
	defer stop()

	publish(t, client, "broken")
	publish(t, client, "fine")

	eventually(t, func() bool { return r.count() == 2 })
	eventually(t, func() bool { return pendingCount(t, client) == 1 })
}

func TestPendingMessagesAreReprocessedOnRestart(t *testing.T) {
	client := newTestRedis(t)
	ctx := context.Background()
	if err := client.XGroupCreateMkStream(ctx, testStream, "notification", "0").Err(); err != nil {
		t.Fatal(err)
	}
	publish(t, client, "read before the crash")
	// The consumer read the message and died before XACK.
	if err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "notification", Consumer: "replica-1", Streams: []string{testStream, ">"},
	}).Err(); err != nil {
		t.Fatal(err)
	}

	r := &recorder{}
	stop := startConsumer(t, client, "replica-1", r)
	defer stop()

	eventually(t, func() bool { return r.count() == 1 })
	eventually(t, func() bool { return pendingCount(t, client) == 0 })
}

func TestMessageThatKeepsFailingDoesNotBlockTheRestart(t *testing.T) {
	client := newTestRedis(t)
	r := &recorder{failing: map[string]bool{"broken": true}}
	stop := startConsumer(t, client, "replica-1", r)
	publish(t, client, "broken")
	eventually(t, func() bool { return r.count() == 1 })
	stop()

	stop = startConsumer(t, client, "replica-1", r)
	defer stop()
	publish(t, client, "after restart")

	// The broken message is tried again once, then the new one is processed.
	eventually(t, func() bool { return r.count() == 3 })
	if got := pendingCount(t, client); got != 1 {
		t.Fatalf("pending = %d, want only the broken message", got)
	}
}

func TestConsumerRetriesWhileRedisIsDown(t *testing.T) {
	// Reserve a free address, then leave nothing listening on it until the test brings Redis up.
	reserved := miniredis.RunT(t)
	addr := reserved.Addr()
	reserved.Close()
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { client.Close() })

	r := &recorder{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c := newConsumer(client, testStream, "notification", "replica-1", r.handle, logger)
	c.block = 50 * time.Millisecond
	c.retryMin = 10 * time.Millisecond
	c.retryMax = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.run(ctx)
	}()
	defer func() {
		cancel()
		<-done
	}()

	time.Sleep(50 * time.Millisecond)
	server := miniredis.NewMiniRedis()
	if err := server.StartAddr(addr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	publish(t, client, "after redis came back")
	eventually(t, func() bool { return r.count() == 1 })
}
