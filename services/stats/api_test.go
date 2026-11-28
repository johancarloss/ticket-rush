package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeSales struct {
	sales map[string]eventSales
	err   error
}

func (f fakeSales) eventSales(_ context.Context, eventID string) (eventSales, bool, error) {
	if f.err != nil {
		return eventSales{}, false, f.err
	}
	sales, found := f.sales[eventID]
	return sales, found, nil
}

func serve(t *testing.T, reader salesReader, eventID, userID, roles string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/stats/events/{event_id}", eventSalesHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), reader))
	request := httptest.NewRequest(http.MethodGet, "/api/stats/events/"+eventID, nil)
	if userID != "" {
		request.Header.Set("X-User-Id", userID)
	}
	if roles != "" {
		request.Header.Set("X-User-Roles", roles)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	return response
}

func errorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body.Error
}

var showSales = fakeSales{sales: map[string]eventSales{"show-2026-12-01": {
	EventID:      "show-2026-12-01",
	EventName:    "Show de Abertura",
	Orders:       2,
	TicketsSold:  3,
	RevenueCents: 60000,
	Currency:     "BRL",
	UpdatedAt:    time.Date(2026, 11, 20, 14, 3, 12, 0, time.UTC),
}}}

func TestOrganizerSeesTheSales(t *testing.T) {
	response := serve(t, showSales, "show-2026-12-01", "organizer-1", "offline_access,organizer")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body eventSales
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body != showSales.sales["show-2026-12-01"] {
		t.Fatalf("body = %+v", body)
	}
}

func TestBuyerIsForbidden(t *testing.T) {
	response := serve(t, showSales, "show-2026-12-01", "buyer-1", "buyer")

	if response.Code != http.StatusForbidden || errorCode(t, response) != "forbidden" {
		t.Fatalf("status = %d, want 403 forbidden", response.Code)
	}
}

func TestRoleMustMatchExactly(t *testing.T) {
	response := serve(t, showSales, "show-2026-12-01", "buyer-1", "buyer,not-organizer")

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
}

func TestRequestThatSkippedTheGuardIsUnauthenticated(t *testing.T) {
	response := serve(t, showSales, "show-2026-12-01", "", "organizer")

	if response.Code != http.StatusUnauthorized || errorCode(t, response) != "unauthenticated" {
		t.Fatalf("status = %d, want 401 unauthenticated", response.Code)
	}
}

func TestEventWithoutSalesIsNotFound(t *testing.T) {
	response := serve(t, showSales, "show-without-sales", "organizer-1", "organizer")

	if response.Code != http.StatusNotFound || errorCode(t, response) != "no_sales" {
		t.Fatalf("status = %d, want 404 no_sales", response.Code)
	}
}

func TestStoreErrorIsInternalError(t *testing.T) {
	response := serve(t, fakeSales{err: errors.New("connection refused")}, "show-2026-12-01", "organizer-1", "organizer")

	if response.Code != http.StatusInternalServerError || errorCode(t, response) != "internal_error" {
		t.Fatalf("status = %d, want 500 internal_error", response.Code)
	}
}
