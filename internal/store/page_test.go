package store_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

func TestTransactionPageIsWhereTheTransactionSitsInTheOrdering(t *testing.T) {
	s, _ := open(t)
	// Seven dated Transactions; ids 5 and 6 share a date with 4 (ties break on the higher id first).
	dates := []string{"2026-09-10", "2026-09-09", "2026-09-08", "2026-09-07", "2026-09-06", "2026-09-06", "2026-09-06", "2026-09-05"}
	ids := make([]int64, len(dates))
	for i, date := range dates {
		id, err := s.AddTransaction(expense(t, s, date, fmt.Sprintf("T%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = id
	}
	// Order: T0..T3 (positions 0-3), then the tie newest id first: T6 (4), T5 (5), T4 (6), then T7 (7).
	want := []int{1, 1, 1, 1, 2, 2, 1, 2}
	for i, id := range ids {
		got, err := s.TransactionPage(id)
		if err != nil || got != want[i] {
			t.Errorf("T%d: page %d, err %v; want %d", i, got, err, want[i])
		}
	}
	if _, err := s.TransactionPage(999); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing id: err %v, want ErrNotFound", err)
	}
}

func TestListTransactionsPageOrdersByDateThenIDDescending(t *testing.T) {
	s, _ := open(t)
	for i, date := range []string{"2026-09-02", "2026-09-05", "2026-09-05", "2026-09-01", "2026-09-03", "2026-09-04", "2026-09-06"} {
		if _, err := s.AddTransaction(expense(t, s, date, fmt.Sprintf("T%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.CountTransactions()
	if err != nil || n != 7 {
		t.Fatalf("count = %d, err = %v", n, err)
	}
	page1, err := s.ListTransactionsPage(5, 0)
	if err != nil {
		t.Fatal(err)
	}
	page2, err := s.ListTransactionsPage(5, 5)
	if err != nil {
		t.Fatal(err)
	}
	got := func(rows []store.TransactionRow) (out []string) {
		for _, r := range rows {
			out = append(out, r.Description)
		}
		return out
	}
	if g := fmt.Sprint(got(page1)); g != "[T6 T2 T1 T5 T4]" {
		t.Errorf("page 1 = %s", g)
	}
	if g := fmt.Sprint(got(page2)); g != "[T0 T3]" {
		t.Errorf("page 2 = %s", g)
	}
}
