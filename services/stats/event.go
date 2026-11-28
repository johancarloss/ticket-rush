package main

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ticketIssued holds the fields of the TicketIssued event (docs/contracts/EVENTOS.md) that the counters need.
// Unknown fields are ignored, as the contract asks.
type ticketIssued struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
	OrderID string `json:"order_id"`
	Event   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"event"`
	Seats       []json.RawMessage `json:"seats"`
	AmountCents int64             `json:"amount_cents"`
}

// sale is what one confirmed order adds to the counters of its event.
type sale struct {
	OrderID     string
	EventID     string
	EventName   string
	Tickets     int64
	AmountCents int64
}

// parseSale reads the payload field of a stream message into the sale it represents.
func parseSale(payload string) (sale, error) {
	var event ticketIssued
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return sale{}, fmt.Errorf("invalid TicketIssued payload: %w", err)
	}
	switch {
	case event.Type != "TicketIssued":
		return sale{}, fmt.Errorf("unexpected event type %q", event.Type)
	case event.Version != 1:
		return sale{}, fmt.Errorf("unsupported TicketIssued version %d", event.Version)
	case event.OrderID == "":
		return sale{}, errors.New("TicketIssued without order_id")
	case event.Event.ID == "":
		return sale{}, errors.New("TicketIssued without event.id")
	case len(event.Seats) == 0:
		return sale{}, errors.New("TicketIssued without seats")
	case event.AmountCents < 0:
		return sale{}, fmt.Errorf("negative amount_cents %d", event.AmountCents)
	}
	return sale{
		OrderID:     event.OrderID,
		EventID:     event.Event.ID,
		EventName:   event.Event.Name,
		Tickets:     int64(len(event.Seats)),
		AmountCents: event.AmountCents,
	}, nil
}
