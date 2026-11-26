package main

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// handler processes one stream message. Returning nil acknowledges the message; an error leaves it pending, to be
// tried again until maxAttempts.
type handler func(ctx context.Context, message redis.XMessage) error

// consumer reads a Redis stream through a consumer group with retry and a dead-letter stream (ADR-006):
//
//   - a message is acknowledged with XACK only after the handler succeeds, so a crash in the middle never loses it;
//   - a message left pending for minIdle is claimed again (XCLAIM), by this replica or by any other, which also
//     picks up the messages of a replica that died;
//   - the delivery count kept by Redis is the attempt number; when the attempt that fails is the last one, the
//     message goes to the dead-letter stream and the original is acknowledged, so it is not tried again.
type consumer struct {
	client      *redis.Client
	stream      string
	deadStream  string
	group       string
	name        string
	handle      handler
	ready       func(context.Context) error
	logger      *slog.Logger
	maxAttempts int64
	minIdle     time.Duration
	block       time.Duration
	batch       int64
	retryMin    time.Duration
	retryMax    time.Duration
}

func newConsumer(client *redis.Client, stream, group, name string, handle handler, ready func(context.Context) error,
	logger *slog.Logger) *consumer {
	return &consumer{
		client:      client,
		stream:      stream,
		deadStream:  stream + ".dead",
		group:       group,
		name:        name,
		handle:      handle,
		ready:       ready,
		logger:      logger.With("stream", stream, "group", group, "consumer", name),
		maxAttempts: 3,
		minIdle:     5 * time.Second,
		block:       2 * time.Second,
		batch:       10,
		retryMin:    500 * time.Millisecond,
		retryMax:    10 * time.Second,
	}
}

// run consumes until ctx is cancelled. Messages already read are finished before it returns, so a SIGTERM never
// leaves one half processed. While Redis or the handler's own dependency is down it waits and tries again.
func (c *consumer) run(ctx context.Context) {
	wait := c.retryMin
	for ctx.Err() == nil {
		started := time.Now()
		err := c.consume(ctx)
		if err == nil || ctx.Err() != nil {
			return
		}
		// A consumer that ran for a while recovered from the last outage; the next one starts with a short wait.
		if time.Since(started) > c.retryMax {
			wait = c.retryMin
		}
		c.logger.Warn("consumer paused", "error", err.Error(), "retry_in", wait.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, c.retryMax)
	}
}

func (c *consumer) consume(ctx context.Context) error {
	if err := c.ensureGroup(ctx); err != nil {
		return err
	}
	c.logger.Info("consumer started")
	for ctx.Err() == nil {
		// Without the dependency every attempt would fail and the messages would reach the dead-letter stream for
		// nothing, so nothing is read until it is back.
		if err := c.ready(ctx); err != nil {
			return err
		}
		if err := c.retryPending(ctx); err != nil {
			return err
		}
		messages, err := c.readNew(ctx)
		if err != nil {
			return err
		}
		c.processAll(ctx, messages, func(redis.XMessage) int64 { return 1 })
	}
	return nil
}

// ensureGroup creates the group at the start of the stream, so events published before the service ever ran are
// still counted. The group already existing is the normal case after the first start.
func (c *consumer) ensureGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err()
	if err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

func (c *consumer) readNew(ctx context.Context) ([]redis.XMessage, error) {
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.name,
		Streams:  []string{c.stream, ">"},
		Count:    c.batch,
		Block:    c.block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return streams[0].Messages, nil
}

// retryPending claims the messages of the whole group that have waited at least minIdle since their last delivery.
// XPENDING gives the delivery count, which XAUTOCLAIM does not return, and XCLAIM with the same minIdle makes sure
// two replicas never take the same message.
func (c *consumer) retryPending(ctx context.Context) error {
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.stream,
		Group:  c.group,
		Idle:   c.minIdle,
		Start:  "-",
		End:    "+",
		Count:  c.batch,
	}).Result()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	deliveries := make(map[string]int64, len(pending))
	ids := make([]string, 0, len(pending))
	for _, entry := range pending {
		deliveries[entry.ID] = entry.RetryCount
		ids = append(ids, entry.ID)
	}
	claimed, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   c.stream,
		Group:    c.group,
		Consumer: c.name,
		MinIdle:  c.minIdle,
		Messages: ids,
	}).Result()
	if err != nil {
		return err
	}
	// XCLAIM counts as one more delivery, so this attempt is the previous count plus one.
	c.processAll(ctx, claimed, func(message redis.XMessage) int64 { return deliveries[message.ID] + 1 })
	return nil
}

func (c *consumer) processAll(ctx context.Context, messages []redis.XMessage, attemptOf func(redis.XMessage) int64) {
	// Detached from ctx: on shutdown the batch already read is finished instead of cut in half.
	work := context.WithoutCancel(ctx)
	for _, message := range messages {
		c.process(work, message, attemptOf(message))
	}
}

func (c *consumer) process(ctx context.Context, message redis.XMessage, attempt int64) {
	// A message delivered more times than allowed without ever failing cleanly crashed the process while being
	// handled; trying it once more would crash it again.
	if attempt > c.maxAttempts {
		c.deadLetter(ctx, message, attempt-1, errors.New("delivered too many times without finishing"))
		return
	}
	err := c.handle(ctx, message)
	if err == nil {
		if err := c.client.XAck(ctx, c.stream, c.group, message.ID).Err(); err != nil {
			// The message stays pending and is handled again; the handler ignores repeated orders (ADR-006).
			c.logger.Error("ack failed", "message_id", message.ID, "error", err.Error())
		}
		return
	}
	if attempt >= c.maxAttempts {
		c.deadLetter(ctx, message, attempt, err)
		return
	}
	c.logger.Warn("message failed, will retry", "message_id", message.ID, "attempt", attempt, "error", err.Error())
}

// deadLetter copies the message to the dead-letter stream and acknowledges the original in one MULTI/EXEC, so the
// message is never in both places nor in neither.
func (c *consumer) deadLetter(ctx context.Context, message redis.XMessage, attempts int64, cause error) {
	payload, _ := message.Values["payload"].(string)
	_, err := c.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.XAdd(ctx, &redis.XAddArgs{Stream: c.deadStream, Values: map[string]any{
			"payload":   payload,
			"source_id": message.ID,
			"group":     c.group,
			"attempts":  strconv.FormatInt(attempts, 10),
			"error":     cause.Error(),
		}})
		pipe.XAck(ctx, c.stream, c.group, message.ID)
		return nil
	})
	if err != nil {
		// Still pending: the next claim tries the dead-letter step again.
		c.logger.Error("dead-letter failed", "message_id", message.ID, "error", err.Error())
		return
	}
	c.logger.Error("message sent to dead-letter stream", "message_id", message.ID, "dead_stream", c.deadStream,
		"attempts", attempts, "error", cause.Error())
}
