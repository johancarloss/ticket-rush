package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// handler processes one stream message. Returning nil acknowledges the message; an error leaves it pending.
type handler func(ctx context.Context, message redis.XMessage) error

// consumer reads a Redis stream through a consumer group (ADR-006): every group receives every message, and a
// message is acknowledged with XACK only after the handler succeeds, so a crash in the middle never loses it.
type consumer struct {
	client   *redis.Client
	stream   string
	group    string
	name     string
	handle   handler
	logger   *slog.Logger
	block    time.Duration
	batch    int64
	retryMin time.Duration
	retryMax time.Duration
}

func newConsumer(client *redis.Client, stream, group, name string, handle handler, logger *slog.Logger) *consumer {
	return &consumer{
		client:   client,
		stream:   stream,
		group:    group,
		name:     name,
		handle:   handle,
		logger:   logger.With("stream", stream, "group", group, "consumer", name),
		block:    2 * time.Second,
		batch:    10,
		retryMin: 500 * time.Millisecond,
		retryMax: 10 * time.Second,
	}
}

// run consumes until ctx is cancelled. A message already being handled is finished before it returns, so a
// SIGTERM never leaves an e-mail half sent. While Redis is unreachable it keeps retrying with backoff.
func (c *consumer) run(ctx context.Context) {
	wait := c.retryMin
	for ctx.Err() == nil {
		err := c.consume(ctx)
		if err == nil || ctx.Err() != nil {
			return
		}
		c.logger.Warn("redis unavailable, retrying", "error", err.Error(), "retry_in", wait.String())
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
	// Messages this consumer read before a crash and never acknowledged come back first.
	if err := c.drainPending(ctx); err != nil {
		return err
	}
	c.logger.Info("consumer started")
	for ctx.Err() == nil {
		messages, err := c.read(ctx, ">")
		if err != nil {
			return err
		}
		c.processAll(ctx, messages)
	}
	return nil
}

// ensureGroup creates the group at the start of the stream, so events published before the service ever ran are
// still processed. The group already existing is the normal case after the first start.
func (c *consumer) ensureGroup(ctx context.Context) error {
	err := c.client.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err()
	if err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// drainPending walks this consumer's pending list from the start. The cursor moves past every message read, so a
// message that keeps failing is tried once per start instead of blocking the ones after it.
func (c *consumer) drainPending(ctx context.Context) error {
	cursor := "0"
	for ctx.Err() == nil {
		messages, err := c.read(ctx, cursor)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			return nil
		}
		c.processAll(ctx, messages)
		cursor = messages[len(messages)-1].ID
	}
	return nil
}

// read returns new messages (">") or this consumer's pending messages after the given ID. An empty result after
// the block time is not an error.
func (c *consumer) read(ctx context.Context, id string) ([]redis.XMessage, error) {
	args := &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.name,
		Streams:  []string{c.stream, id},
		Count:    c.batch,
	}
	// Pending messages are already there; only new ones are worth waiting for.
	if id == ">" {
		args.Block = c.block
	}
	streams, err := c.client.XReadGroup(ctx, args).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return streams[0].Messages, nil
}

func (c *consumer) processAll(ctx context.Context, messages []redis.XMessage) {
	// Detached from ctx: on shutdown the batch already read is finished instead of cut in half.
	work := context.WithoutCancel(ctx)
	for _, message := range messages {
		c.process(work, message)
	}
}

func (c *consumer) process(ctx context.Context, message redis.XMessage) {
	if err := c.handle(ctx, message); err != nil {
		// Left pending: it is read again on the next start. Retry and the DLQ come with issue #41.
		c.logger.Error("message failed, left pending", "message_id", message.ID, "error", err.Error())
		return
	}
	if err := c.client.XAck(ctx, c.stream, c.group, message.ID).Err(); err != nil {
		// The message stays pending and is handled again; consumers ignore repeated orders (ADR-006).
		c.logger.Error("ack failed", "message_id", message.ID, "error", err.Error())
	}
}
