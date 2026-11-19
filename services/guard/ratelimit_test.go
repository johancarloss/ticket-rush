package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestWindow(t *testing.T, limit int64) (*fixedWindow, *miniredis.Miniredis, *time.Time) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { client.Close() })

	now := time.Unix(1_800_000_000, 0)
	window := newFixedWindow(client, limit)
	window.now = func() time.Time { return now }
	return window, server, &now
}

func TestFixedWindowAllowsUpToTheLimit(t *testing.T) {
	window, _, _ := newTestWindow(t, 10)
	ctx := context.Background()

	for i := 1; i <= 10; i++ {
		if ok, err := window.allow(ctx, "buyer-1"); err != nil || !ok {
			t.Fatalf("request %d: ok=%v err=%v, want allowed", i, ok, err)
		}
	}
	if ok, _ := window.allow(ctx, "buyer-1"); ok {
		t.Fatal("request 11 in the same second was allowed")
	}
	if ok, _ := window.allow(ctx, "buyer-2"); !ok {
		t.Fatal("another account was limited by buyer-1's count")
	}
}

func TestFixedWindowResetsNextSecond(t *testing.T) {
	window, _, now := newTestWindow(t, 1)
	ctx := context.Background()

	window.allow(ctx, "buyer-1")
	if ok, _ := window.allow(ctx, "buyer-1"); ok {
		t.Fatal("second request in the same second was allowed with limit 1")
	}
	*now = now.Add(time.Second)
	if ok, _ := window.allow(ctx, "buyer-1"); !ok {
		t.Fatal("request in the next second was still limited")
	}
}

func TestFixedWindowKeysExpire(t *testing.T) {
	window, server, _ := newTestWindow(t, 10)
	window.allow(context.Background(), "buyer-1")

	if len(server.Keys()) != 1 {
		t.Fatalf("keys = %v, want one counter", server.Keys())
	}
	server.FastForward(3 * time.Second)
	if len(server.Keys()) != 0 {
		t.Fatalf("keys = %v after 3 s, want none: counters must not pile up in Redis", server.Keys())
	}
}

func TestFixedWindowReturnsErrorWhenRedisIsDown(t *testing.T) {
	window, server, _ := newTestWindow(t, 10)
	server.Close()

	if _, err := window.allow(context.Background(), "buyer-1"); err == nil {
		t.Fatal("no error with Redis down; the handler would not know to fail open")
	}
}
