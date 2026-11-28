package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testStream = "tickets.issued"
	testDead   = "tickets.issued.dead"
	testGroup  = "stats"
)

// recorder is a handler that counts how many times it saw each payload and fails each one the number of times
// listed in failures (-1 fails forever).
type recorder struct {
	mu       sync.Mutex
	seen     map[string]int
	failures map[string]int
}

func newRecorder(failures map[string]int) *recorder {
	return &recorder{seen: map[string]int{}, failures: failures}
}

func (r *recorder) handle(_ context.Context, message redis.XMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	payload, _ := message.Values["payload"].(string)
	r.seen[payload]++
	if left := r.failures[payload]; left != 0 {
		r.failures[payload] = left - 1
		return errors.New("simulated failure")
	}
	return nil
}

func (r *recorder) times(payload string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seen[payload]
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })
	return client
}

func alwaysReady(context.Context) error { return nil }

// fastConsumer returns a consumer with short waits, so retries happen in milliseconds.
func fastConsumer(client *redis.Client, name string, handle handler, ready func(context.Context) error) *consumer {
	c := newConsumer(client, testStream, testGroup, name, handle, ready, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c.minIdle = 20 * time.Millisecond
	c.block = 10 * time.Millisecond
	c.retryMin = 10 * time.Millisecond
	c.retryMax = 20 * time.Millisecond
	return c
}

// start runs the consumer in the background and returns a function that stops it and waits for it to return.
func start(t *testing.T, c *consumer) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.run(ctx)
	}()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("consumer did not stop after cancel")
		}
	}
	t.Cleanup(stop)
	return stop
}

func publish(t *testing.T, client *redis.Client, payload string) string {
	t.Helper()
	id, err := client.XAdd(context.Background(), &redis.XAddArgs{Stream: testStream, Values: map[string]any{"payload": payload}}).Result()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func pendingCount(t *testing.T, client *redis.Client) int64 {
	t.Helper()
	pending, err := client.XPending(context.Background(), testStream, testGroup).Result()
	if err != nil {
		t.Fatal(err)
	}
	return pending.Count
}

func deadMessages(t *testing.T, client *redis.Client) []redis.XMessage {
	t.Helper()
	messages, err := client.XRange(context.Background(), testDead, "-", "+").Result()
	if err != nil {
		t.Fatal(err)
	}
	return messages
}

func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestProcessedMessageIsAcknowledged(t *testing.T) {
	client := newTestRedis(t)
	publish(t, client, "published before the group existed")
	r := newRecorder(nil)
	start(t, fastConsumer(client, "replica-1", r.handle, alwaysReady))

	publish(t, client, "published after")

	eventually(t, func() bool {
		return r.times("published before the group existed") == 1 && r.times("published after") == 1
	})
	eventually(t, func() bool { return pendingCount(t, client) == 0 })
}

func TestMessageThatFailsOnceIsRetried(t *testing.T) {
	client := newTestRedis(t)
	r := newRecorder(map[string]int{"flaky": 1})
	start(t, fastConsumer(client, "replica-1", r.handle, alwaysReady))

	publish(t, client, "flaky")

	eventually(t, func() bool { return r.times("flaky") == 2 })
	eventually(t, func() bool { return pendingCount(t, client) == 0 })
	if dead := deadMessages(t, client); len(dead) != 0 {
		t.Fatalf("dead-letter stream has %d messages, want 0", len(dead))
	}
}

func TestMessageThatAlwaysFailsGoesToDeadLetterAfterThreeAttempts(t *testing.T) {
	client := newTestRedis(t)
	r := newRecorder(map[string]int{"broken": -1})
	start(t, fastConsumer(client, "replica-1", r.handle, alwaysReady))

	brokenID := publish(t, client, "broken")
	publish(t, client, "fine")

	eventually(t, func() bool { return len(deadMessages(t, client)) == 1 })
	eventually(t, func() bool { return pendingCount(t, client) == 0 })
	if got := r.times("broken"); got != 3 {
		t.Fatalf("broken message tried %d times, want 3", got)
	}
	if got := r.times("fine"); got != 1 {
		t.Fatalf("fine message processed %d times, want 1", got)
	}
	dead := deadMessages(t, client)[0].Values
	want := map[string]any{"payload": "broken", "source_id": brokenID, "group": testGroup, "attempts": "3", "error": "simulated failure"}
	for key, value := range want {
		if dead[key] != value {
			t.Errorf("dead-letter %s = %v, want %v", key, dead[key], value)
		}
	}
}

func TestPendingMessageOfADeadReplicaIsClaimed(t *testing.T) {
	client := newTestRedis(t)
	ctx := context.Background()
	if err := client.XGroupCreateMkStream(ctx, testStream, testGroup, "0").Err(); err != nil {
		t.Fatal(err)
	}
	publish(t, client, "read by the replica that died")
	// Another replica read the message and died before XACK.
	if err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: testGroup, Consumer: "replica-dead", Streams: []string{testStream, ">"},
	}).Err(); err != nil {
		t.Fatal(err)
	}

	r := newRecorder(nil)
	start(t, fastConsumer(client, "replica-1", r.handle, alwaysReady))

	eventually(t, func() bool { return r.times("read by the replica that died") == 1 })
	eventually(t, func() bool { return pendingCount(t, client) == 0 })
}

func TestMessageThatCrashedTheProcessGoesToDeadLetterWithoutBeingHandled(t *testing.T) {
	client := newTestRedis(t)
	ctx := context.Background()
	if err := client.XGroupCreateMkStream(ctx, testStream, testGroup, "0").Err(); err != nil {
		t.Fatal(err)
	}
	id := publish(t, client, "crashes the process")
	if err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: testGroup, Consumer: "replica-dead", Streams: []string{testStream, ">"},
	}).Err(); err != nil {
		t.Fatal(err)
	}
	// Two more deliveries that never finished, three in all, as if the process died each time it handled the message.
	for range 2 {
		if err := client.XClaim(ctx, &redis.XClaimArgs{
			Stream: testStream, Group: testGroup, Consumer: "replica-dead", Messages: []string{id},
		}).Err(); err != nil {
			t.Fatal(err)
		}
	}

	r := newRecorder(nil)
	start(t, fastConsumer(client, "replica-1", r.handle, alwaysReady))

	eventually(t, func() bool { return len(deadMessages(t, client)) == 1 })
	if got := r.times("crashes the process"); got != 0 {
		t.Fatalf("handled %d times, want 0", got)
	}
	if got := pendingCount(t, client); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}
}

func TestNothingIsReadWhileTheDependencyIsDown(t *testing.T) {
	client := newTestRedis(t)
	var databaseUp atomic.Bool
	ready := func(context.Context) error {
		if !databaseUp.Load() {
			return errors.New("postgres down")
		}
		return nil
	}
	r := newRecorder(nil)
	start(t, fastConsumer(client, "replica-1", r.handle, ready))

	publish(t, client, "during the outage")
	time.Sleep(100 * time.Millisecond)
	if got := r.times("during the outage"); got != 0 {
		t.Fatalf("handled %d times while the dependency was down, want 0", got)
	}
	if dead := deadMessages(t, client); len(dead) != 0 {
		t.Fatalf("dead-letter stream has %d messages, want 0", len(dead))
	}

	databaseUp.Store(true)
	eventually(t, func() bool { return r.times("during the outage") == 1 })
	eventually(t, func() bool { return pendingCount(t, client) == 0 })
}

func TestMessagesAlreadyReadAreFinishedOnShutdown(t *testing.T) {
	client := newTestRedis(t)
	release := make(chan struct{})
	var handled atomic.Int32
	slow := func(ctx context.Context, message redis.XMessage) error {
		<-release
		if ctx.Err() != nil {
			return ctx.Err()
		}
		handled.Add(1)
		return nil
	}
	stop := start(t, fastConsumer(client, "replica-1", slow, alwaysReady))
	publish(t, client, "in progress during shutdown")
	eventually(t, func() bool { return pendingCount(t, client) == 1 })

	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()
	time.Sleep(20 * time.Millisecond)
	close(release)
	<-stopped

	if handled.Load() != 1 {
		t.Fatal("message read before shutdown was not finished")
	}
	if got := pendingCount(t, client); got != 0 {
		t.Fatalf("pending = %d, want 0", got)
	}
}
