package main

import (
	"strings"
	"testing"
)

// validPayload is the example from docs/contracts/EVENTOS.md.
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

func TestParseTicketIssuedAcceptsTheContractExample(t *testing.T) {
	event, err := parseTicketIssued(validPayload)
	if err != nil {
		t.Fatal(err)
	}
	if event.OrderID != "7" || event.Event.ID != "show-2026-12-01" || len(event.Seats) != 2 || event.AmountCents != 40000 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestParseTicketIssuedRejectsBrokenEvents(t *testing.T) {
	cases := map[string]string{
		"not json":      `{"type":`,
		"wrong type":    strings.Replace(validPayload, `"TicketIssued"`, `"OrderCancelled"`, 1),
		"newer version": strings.Replace(validPayload, `"version": 1`, `"version": 2`, 1),
		"no order":      strings.Replace(validPayload, `"order_id": "7"`, `"order_id": ""`, 1),
		"no e-mail":     strings.Replace(validPayload, `"buyer_email": "comprador@exemplo.com"`, `"buyer_email": ""`, 1),
		"no event id":   strings.Replace(validPayload, `"id": "show-2026-12-01"`, `"id": ""`, 1),
		"no seats":      `{"type":"TicketIssued","version":1,"order_id":"7","buyer_email":"a@b.c","event":{"id":"e"},"seats":[]}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseTicketIssued(payload); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
