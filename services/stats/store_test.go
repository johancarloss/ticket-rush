package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests run against the real Postgres of the Compose (docs/TESTES.md, integration level), because what they
// check, the primary key stopping a repeated order, only exists in the database. Without STATS_TEST_DATABASE_URL
// they are skipped; docs/services/ESTATISTICAS.md shows how to run them. Every test uses IDs of its own, so they
// can run against a database that already has data.
func testStore(t *testing.T) *store {
	t.Helper()
	url := os.Getenv("STATS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("STATS_TEST_DATABASE_URL not set; integration test skipped")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s := &store{pool: pool}
	if err := s.migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func uniqueID(t *testing.T, prefix string) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return prefix + "-" + hex.EncodeToString(b)
}

func TestRepeatedEventIsNotCountedTwice(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	eventID := uniqueID(t, "show")
	order := sale{OrderID: uniqueID(t, "order"), EventID: eventID, EventName: "Show de Abertura", Tickets: 2, AmountCents: 40000}

	first, err := s.recordSale(ctx, order)
	if err != nil || !first {
		t.Fatalf("first recordSale = %v, %v; want true, nil", first, err)
	}
	second, err := s.recordSale(ctx, order)
	if err != nil || second {
		t.Fatalf("second recordSale = %v, %v; want false, nil", second, err)
	}

	got, found, err := s.eventSales(ctx, eventID)
	if err != nil || !found {
		t.Fatalf("eventSales = %v, %v", found, err)
	}
	if got.Orders != 1 || got.TicketsSold != 2 || got.RevenueCents != 40000 {
		t.Fatalf("sales = %+v, want 1 order, 2 tickets, 40000 cents", got)
	}
}

func TestConcurrentCopiesOfTheSameEventCountOnce(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	eventID := uniqueID(t, "show")
	order := sale{OrderID: uniqueID(t, "order"), EventID: eventID, EventName: "Show de Abertura", Tickets: 4, AmountCents: 80000}

	var wg sync.WaitGroup
	var mu sync.Mutex
	recorded := 0
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := s.recordSale(ctx, order)
			if err != nil {
				t.Error(err)
				return
			}
			if ok {
				mu.Lock()
				recorded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if recorded != 1 {
		t.Fatalf("%d copies recorded, want 1", recorded)
	}
	got, _, err := s.eventSales(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Orders != 1 || got.TicketsSold != 4 || got.RevenueCents != 80000 {
		t.Fatalf("sales = %+v, want 1 order, 4 tickets, 80000 cents", got)
	}
}

func TestDifferentOrdersAddUp(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	eventID := uniqueID(t, "show")
	for _, tickets := range []int64{1, 3} {
		order := sale{OrderID: uniqueID(t, "order"), EventID: eventID, EventName: "Show de Abertura", Tickets: tickets, AmountCents: tickets * 20000}
		if _, err := s.recordSale(ctx, order); err != nil {
			t.Fatal(err)
		}
	}

	got, _, err := s.eventSales(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Orders != 2 || got.TicketsSold != 4 || got.RevenueCents != 80000 || got.Currency != "BRL" {
		t.Fatalf("sales = %+v, want 2 orders, 4 tickets, 80000 cents in BRL", got)
	}
}

func TestEventWithoutSalesIsReportedAsMissing(t *testing.T) {
	s := testStore(t)

	_, found, err := s.eventSales(context.Background(), uniqueID(t, "show"))
	if err != nil || found {
		t.Fatalf("eventSales = %v, %v; want false, nil", found, err)
	}
}
