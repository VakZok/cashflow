package store_test

import (
	"fmt"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

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
