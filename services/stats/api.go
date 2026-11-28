package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
)

// salesReader is the part of the store the API needs, so the handler can be tested without Postgres.
type salesReader interface {
	eventSales(ctx context.Context, eventID string) (eventSales, bool, error)
}

// eventSalesHandler answers GET /api/stats/events/{event_id} (docs/contracts/stats.openapi.yaml). The route goes
// through buyer-auth@file, so the Guard already checked the token and set X-User-Id and X-User-Roles; the service
// only checks that the user is an organizer (docs/guides/NOVO-SERVICO.md).
func eventSalesHandler(logger *slog.Logger, sales salesReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-Id")
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "missing X-User-Id; the route must go through buyer-auth")
			return
		}
		if !slices.Contains(strings.Split(r.Header.Get("X-User-Roles"), ","), "organizer") {
			writeError(w, http.StatusForbidden, "forbidden", "only organizers can see sales")
			return
		}

		eventID := r.PathValue("event_id")
		result, found, err := sales.eventSales(r.Context(), eventID)
		if err != nil {
			logger.Error("reading sales failed", "event_id", eventID, "error", err.Error())
			writeError(w, http.StatusInternalServerError, "internal_error", "could not read the sales")
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "no_sales", "no sale recorded for event "+eventID)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
