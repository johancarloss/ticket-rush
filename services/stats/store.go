package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// eventSales are the counters of one event (the show), as the organizer sees them.
type eventSales struct {
	EventID      string    `json:"event_id"`
	EventName    string    `json:"event_name"`
	Orders       int64     `json:"orders"`
	TicketsSold  int64     `json:"tickets_sold"`
	RevenueCents int64     `json:"revenue_cents"`
	Currency     string    `json:"currency"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// store keeps the counters in the stats schema (ADR-005). The search_path of stats_svc already points at it, so the
// statements use the table names alone.
type store struct {
	pool *pgxpool.Pool
}

// The advisory lock lets two replicas start at the same time without both creating the tables.
const schema = `
SELECT pg_advisory_xact_lock(hashtext('stats.schema'));

CREATE TABLE IF NOT EXISTS processed_orders (
	order_id     text PRIMARY KEY,
	event_id     text NOT NULL,
	processed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS event_sales (
	event_id      text PRIMARY KEY,
	event_name    text NOT NULL,
	orders        bigint NOT NULL,
	tickets_sold  bigint NOT NULL,
	revenue_cents bigint NOT NULL,
	updated_at    timestamptz NOT NULL
);
`

func (s *store) migrate(ctx context.Context) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, schema)
		return err
	})
}

func (s *store) ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.pool.Ping(ctx)
}

// recordSale adds the order to the counters of its event, once. The order ID goes into processed_orders in the
// same transaction as the increment: a repeated event (the Checkout outbox may publish twice, ADR-001) finds the ID
// there, adds nothing, and recordSale returns false. Two copies arriving at the same time are serialized by the
// primary key, so only one of them increments.
func (s *store) recordSale(ctx context.Context, sale sale) (bool, error) {
	recorded := false
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO processed_orders (order_id, event_id) VALUES ($1, $2) ON CONFLICT (order_id) DO NOTHING`,
			sale.OrderID, sale.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO event_sales (event_id, event_name, orders, tickets_sold, revenue_cents, updated_at)
			VALUES ($1, $2, 1, $3, $4, now())
			ON CONFLICT (event_id) DO UPDATE SET
				event_name    = EXCLUDED.event_name,
				orders        = event_sales.orders + 1,
				tickets_sold  = event_sales.tickets_sold + EXCLUDED.tickets_sold,
				revenue_cents = event_sales.revenue_cents + EXCLUDED.revenue_cents,
				updated_at    = now()`,
			sale.EventID, sale.EventName, sale.Tickets, sale.AmountCents)
		recorded = err == nil
		return err
	})
	return recorded, err
}

// eventSales returns the counters of the event, and false when no sale of it was recorded.
func (s *store) eventSales(ctx context.Context, eventID string) (eventSales, bool, error) {
	sales := eventSales{Currency: "BRL"}
	err := s.pool.QueryRow(ctx,
		`SELECT event_id, event_name, orders, tickets_sold, revenue_cents, updated_at FROM event_sales WHERE event_id = $1`,
		eventID).Scan(&sales.EventID, &sales.EventName, &sales.Orders, &sales.TicketsSold, &sales.RevenueCents, &sales.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return eventSales{}, false, nil
	}
	if err != nil {
		return eventSales{}, false, err
	}
	return sales, true, nil
}
