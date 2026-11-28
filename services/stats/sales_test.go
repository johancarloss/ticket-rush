package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

// validPayload is the example from docs/contracts/EVENTOS.md, with a field the contract may add later.
const validPayload = `{
  "message_id": "0b7c6f0e-5d1a-4c3e-9f2b-8a1d2e3f4a5b",
  "type": "TicketIssued",
  "version": 1,
  "occurred_at": "2026-11-20T14:03:12.481Z",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
  "order_id": "7",
  "buyer_id": "42",
  "buyer_email": "comprador@exemplo.com",
  "event": {"id": "show-2026-12-01", "name": "Show de Abertura"},
  "seats": [
    {"seat_id": "A-12", "sector": "Pista", "row": "A", "number": 12},
    {"seat_id": "A-13", "sector": "Pista", "row": "A", "number": 13}
  ],
  "amount_cents": 40000,
  "currency": "BRL",
  "field_added_later": true
}`

func TestParseSaleReadsTheContractExample(t *testing.T) {
	got, err := parseSale(validPayload)
	if err != nil {
		t.Fatal(err)
	}
	want := sale{OrderID: "7", EventID: "show-2026-12-01", EventName: "Show de Abertura", Tickets: 2, AmountCents: 40000}
	if got != want {
		t.Fatalf("sale = %+v, want %+v", got, want)
	}
}

func TestParseSaleRejectsBrokenEvents(t *testing.T) {
	cases := map[string]string{
		"not json":        `{"type":`,
		"wrong type":      strings.Replace(validPayload, `"TicketIssued"`, `"OrderCancelled"`, 1),
		"newer version":   strings.Replace(validPayload, `"version": 1`, `"version": 2`, 1),
		"no order":        strings.Replace(validPayload, `"order_id": "7"`, `"order_id": ""`, 1),
		"no event id":     strings.Replace(validPayload, `"id": "show-2026-12-01"`, `"id": ""`, 1),
		"negative amount": strings.Replace(validPayload, `"amount_cents": 40000`, `"amount_cents": -1`, 1),
		"no seats":        `{"type":"TicketIssued","version":1,"order_id":"7","event":{"id":"e"},"seats":[],"amount_cents":1}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseSale(payload); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

type fakeRecorder struct {
	recorded map[string]bool
	err      error
}

func (f *fakeRecorder) recordSale(_ context.Context, s sale) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.recorded[s.OrderID] {
		return false, nil
	}
	f.recorded[s.OrderID] = true
	return true, nil
}

func TestRepeatedOrderIsAcknowledgedWithoutCounting(t *testing.T) {
	sales := &fakeRecorder{recorded: map[string]bool{}}
	handle := recordSaleHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), sales)
	message := redis.XMessage{ID: "1-0", Values: map[string]any{"payload": validPayload}}

	for range 2 {
		if err := handle(context.Background(), message); err != nil {
			t.Fatal(err)
		}
	}
	if len(sales.recorded) != 1 {
		t.Fatalf("recorded %d orders, want 1", len(sales.recorded))
	}
}

func TestStoreErrorLeavesTheMessageForRetry(t *testing.T) {
	sales := &fakeRecorder{err: errors.New("connection refused")}
	handle := recordSaleHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), sales)

	err := handle(context.Background(), redis.XMessage{ID: "1-0", Values: map[string]any{"payload": validPayload}})
	if err == nil {
		t.Fatal("expected an error, so the consumer retries the message")
	}
}
