package main

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// limiter answers whether an account still has room in the current second (ADR-002).
type limiter interface {
	allow(ctx context.Context, userID string) (bool, error)
}

// fixedWindow counts requests per account in one-second windows, in Redis so the two Guard replicas share the count.
// A fixed window lets an account reach twice the limit across the edge of two seconds; for stopping a bot that
// hammers the API all the time this is enough, and it costs a single round trip.
type fixedWindow struct {
	client  *redis.Client
	limit   int64
	timeout time.Duration
	now     func() time.Time
}

// INCR and EXPIRE run together, so a key never stays without expiry if the Guard dies between them.
var incrWithExpiry = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
  redis.call("EXPIRE", KEYS[1], ARGV[1])
end
return count
`)

func newFixedWindow(client *redis.Client, limit int64) *fixedWindow {
	return &fixedWindow{client: client, limit: limit, timeout: 50 * time.Millisecond, now: time.Now}
}

func (f *fixedWindow) allow(ctx context.Context, userID string) (bool, error) {
	// A slow Redis must not hold every purchase; past the timeout the caller decides what to do with the error.
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	key := "guard:rl:" + userID + ":" + strconv.FormatInt(f.now().Unix(), 10)
	count, err := incrWithExpiry.Run(ctx, f.client, []string{key}, 2).Int64()
	if err != nil {
		return false, err
	}
	return count <= f.limit, nil
}
