package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ticketIssued is the TicketIssued event published by the Checkout (docs/contracts/EVENTOS.md).
// Fields the service does not use yet are kept so the log shows what arrived; unknown fields are ignored.
type ticketIssued struct {
	MessageID   string    `json:"message_id"`
	Type        string    `json:"type"`
	Version     int       `json:"version"`
	OccurredAt  time.Time `json:"occurred_at"`
	TraceID     string    `json:"trace_id"`
	Traceparent string    `json:"traceparent"`
	OrderID     string    `json:"order_id"`
	BuyerID     string    `json:"buyer_id"`
	BuyerEmail  string    `json:"buyer_email"`
	Event       struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"event"`
	Seats []struct {
		SeatID string `json:"seat_id"`
		Sector string `json:"sector"`
		Row    string `json:"row"`
		Number int    `json:"number"`
	} `json:"seats"`
	AmountCents int64  `json:"amount_cents"`
	Currency    string `json:"currency"`
}

// parseTicketIssued reads the payload field of a stream message and checks the fields an e-mail cannot go without.
func parseTicketIssued(payload string) (ticketIssued, error) {
	var event ticketIssued
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return ticketIssued{}, fmt.Errorf("invalid TicketIssued payload: %w", err)
	}
	switch {
	case event.Type != "TicketIssued":
		return ticketIssued{}, fmt.Errorf("unexpected event type %q", event.Type)
	case event.Version != 1:
		return ticketIssued{}, fmt.Errorf("unsupported TicketIssued version %d", event.Version)
	case event.OrderID == "":
		return ticketIssued{}, errors.New("TicketIssued without order_id")
	case event.BuyerEmail == "":
		return ticketIssued{}, errors.New("TicketIssued without buyer_email")
	case event.Event.ID == "":
		return ticketIssued{}, errors.New("TicketIssued without event.id")
	case len(event.Seats) == 0:
		return ticketIssued{}, errors.New("TicketIssued without seats")
	}
	return event, nil
}
