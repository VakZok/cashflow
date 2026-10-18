package store_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

// descriptions are the rows' descriptions, in order.
func descriptions(rows []store.TransactionRow) (out []string) {
	for _, r := range rows {
		out = append(out, r.Description)
	}
	return out
}

func TestAFilterNarrowsListAndCountToItsDateRange(t *testing.T) {
	s, _ := open(t)
	for _, date := range []string{"2025-12-31", "2026-01-01", "2026-02-28", "2026-03-01", "2026-12-31", "2027-01-01"} {
		if _, err := s.AddTransaction(expense(t, s, date, date)); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name string
		f    store.Filter
		want string
	}{
		{"none", store.Filter{}, "[2027-01-01 2026-12-31 2026-03-01 2026-02-28 2026-01-01 2025-12-31]"},
		{"year", store.Filter{From: "2026-01-01", To: "2027-01-01"}, "[2026-12-31 2026-03-01 2026-02-28 2026-01-01]"},
		{"month", store.Filter{From: "2026-02-01", To: "2026-03-01"}, "[2026-02-28]"},
	} {
		rows, err := s.ListTransactionsPage(c.f, -1, 0)
		if err != nil {
			t.Fatal(err)
		}
		if g := fmt.Sprint(descriptions(rows)); g != c.want {
			t.Errorf("%s: list = %s, want %s", c.name, g, c.want)
		}
		if n, err := s.CountTransactions(c.f); err != nil || n != len(rows) {
			t.Errorf("%s: count = %d, err %v; want %d", c.name, n, err, len(rows))
		}
	}
}

func TestTransactionPageCountsOnlyTransactionsMatchingTheFilter(t *testing.T) {
	s, _ := open(t)
	var ids []int64
	// Six in 2027 push the 2026 ones to page 2 unfiltered.
	for i, date := range []string{"2026-05-01", "2026-05-02", "2027-01-01", "2027-01-02", "2027-01-03", "2027-01-04", "2027-01-05", "2027-01-06"} {
		id, err := s.AddTransaction(expense(t, s, date, fmt.Sprintf("T%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	in2026 := store.Filter{From: "2026-01-01", To: "2027-01-01"}
	if p, err := s.TransactionPage(ids[0], store.Filter{}); err != nil || p != 2 {
		t.Errorf("unfiltered: page %d, err %v; want 2", p, err)
	}
	if p, err := s.TransactionPage(ids[0], in2026); err != nil || p != 1 {
		t.Errorf("2026: page %d, err %v; want 1", p, err)
	}
	if _, err := s.TransactionPage(ids[2], in2026); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("outside the filter: err %v, want ErrNotFound", err)
	}
}

func TestTransactionYearsAreTheYearsWithTransactionsNewestFirst(t *testing.T) {
	s, _ := open(t)
	if ys, err := s.TransactionYears(); err != nil || len(ys) != 0 {
		t.Fatalf("empty: years %v, err %v", ys, err)
	}
	for _, date := range []string{"2024-03-01", "2026-01-01", "2026-12-31", "2024-07-07"} {
		if _, err := s.AddTransaction(expense(t, s, date, date)); err != nil {
			t.Fatal(err)
		}
	}
	ys, err := s.TransactionYears()
	if err != nil || fmt.Sprint(ys) != "[2026 2024]" {
		t.Errorf("years %v, err %v; want [2026 2024]", ys, err)
	}
}
