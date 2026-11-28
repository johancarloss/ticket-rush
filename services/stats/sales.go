package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// saleRecorder is the part of the store the consumer needs, so the handler can be tested without Postgres.
type saleRecorder interface {
	recordSale(ctx context.Context, sale sale) (bool, error)
}

func recordSaleHandler(logger *slog.Logger, sales saleRecorder) handler {
	return func(ctx context.Context, message redis.XMessage) error {
		payload, _ := message.Values["payload"].(string)
		sale, err := parseSale(payload)
		if err != nil {
			return err
		}
		recorded, err := sales.recordSale(ctx, sale)
		if err != nil {
			return fmt.Errorf("recording sale: %w", err)
		}
		if !recorded {
			logger.Info("repeated order ignored", "message_id", message.ID, "order_id", sale.OrderID, "event_id", sale.EventID)
			return nil
		}
		logger.Info("sale recorded", "message_id", message.ID, "order_id", sale.OrderID, "event_id", sale.EventID,
			"tickets", sale.Tickets, "amount_cents", sale.AmountCents)
		return nil
	}
}
