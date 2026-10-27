package store_test

import (
	"errors"
	"testing"

	"github.com/VakZok/Cashflow/internal/store"
)

func rebate(t *testing.T, s *store.Store, date, desc string) store.NewTransaction {
	t.Helper()
	n := levy(t, s, date, desc)
	n.Type = store.TypeRebate
	return n
}

func TestRebateShowsInListWithItsLevyCategories(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(rebate(t, s, "2026-10-05", "Tax refund")); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	if r := rows[0]; r.Type != store.TypeRebate || r.Category != "LC" || r.Subcategory != "LS" || r.Description != "Tax refund" {
		t.Errorf("row = %+v", r)
	}
}

func TestRebateFieldRules(t *testing.T) {
	_, raw := open(t)
	for _, q := range []string{
		`INSERT INTO categories (name) VALUES ('C')`,
		`INSERT INTO subcategories (name) VALUES ('S')`,
		`INSERT INTO holdings (name) VALUES ('H')`,
		`INSERT INTO levy_categories (name) VALUES ('LC')`,
		`INSERT INTO levy_subcategories (name) VALUES ('LS')`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name                            string
		desc, cat, sub, hld, lcat, lsub any
		ok                              bool
	}{
		{"complete", "x", nil, nil, nil, 1, 1, true},
		{"without description", nil, nil, nil, nil, 1, 1, false},
		{"without levy category", "x", nil, nil, nil, nil, 1, false},
		{"without levy subcategory", "x", nil, nil, nil, 1, nil, false},
		{"with category", "x", 1, nil, nil, 1, 1, false},
		{"with subcategory", "x", nil, 1, nil, 1, 1, false},
		{"with holding", "x", nil, nil, 1, 1, 1, false},
	}
	for i, c := range cases {
		_, err := raw.Exec(`INSERT INTO transactions
			(date, amount, description, type, category_id, subcategory_id, holding_id, levy_category_id, levy_subcategory_id)
			VALUES (?, 5, ?, 'rebate', ?, ?, ?, ?, ?)`, "2026-10-0"+string(rune('1'+i)), c.desc, c.cat, c.sub, c.hld, c.lcat, c.lsub)
		if c.ok && err != nil {
			t.Errorf("%s: rejected: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
}

func TestDuplicateRebateIsRejectedButOtherTypesAreNot(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(rebate(t, s, "2026-10-05", "Tax refund")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransaction(rebate(t, s, "2026-10-05", "TAX REFUND")); !errors.Is(err, store.ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
	if _, err := s.AddTransaction(tx(t, s, "2026-10-05", 5, "Tax refund", store.TypeIncome)); err != nil {
		t.Errorf("income with a rebate's date and description rejected: %v", err)
	}
	if _, err := s.AddTransaction(levy(t, s, "2026-10-05", "Tax refund")); err != nil {
		t.Errorf("levy with a rebate's date and description rejected: %v", err)
	}
}

func TestRebateLargerThanAnyLevyIsAccepted(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(levy(t, s, "2026-10-01", "Income tax")); err != nil {
		t.Fatal(err)
	}
	r := rebate(t, s, "2026-10-05", "Tax refund")
	r.Amount = 5000
	if _, err := s.AddTransaction(r); err != nil {
		t.Errorf("large rebate rejected: %v", err)
	}
}
