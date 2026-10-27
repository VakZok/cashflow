package store_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/VakZok/Cashflow/internal/store"
)

func levy(t *testing.T, s *store.Store, date, desc string) store.NewTransaction {
	t.Helper()
	n := store.NewTransaction{Date: date, Amount: 5, Description: desc, Type: store.TypeLevy}
	var err error
	if n.LevyCategoryID, err = s.EnsureLevyCategory("LC"); err != nil {
		t.Fatal(err)
	}
	if n.LevySubcategoryID, err = s.EnsureLevySubcategory("LS"); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestLevyShowsInListWithItsLevyCategories(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(levy(t, s, "2026-10-05", "Income tax")); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	if r := rows[0]; r.Type != store.TypeLevy || r.Category != "LC" || r.Subcategory != "LS" || r.Description != "Income tax" {
		t.Errorf("row = %+v", r)
	}
}

func TestLevyFieldRules(t *testing.T) {
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
	calls := 0
	insert := func(typ string, desc, cat, sub, hold, lcat, lsub any) error {
		calls++
		_, err := raw.Exec(`INSERT INTO transactions
			(date, amount, description, type, category_id, subcategory_id, holding_id, levy_category_id, levy_subcategory_id)
			VALUES (?, 5, ?, ?, ?, ?, ?, ?, ?)`, fmt.Sprintf("2026-10-%02d", calls), desc, typ, cat, sub, hold, lcat, lsub)
		return err
	}
	cases := []struct {
		name                            string
		typ                             string
		desc, cat, sub, hld, lcat, lsub any
		ok                              bool
	}{
		{"levy complete", "levy", "x", nil, nil, nil, 1, 1, true},
		{"levy without description", "levy", nil, nil, nil, nil, 1, 1, false},
		{"levy without levy category", "levy", "x", nil, nil, nil, nil, 1, false},
		{"levy without levy subcategory", "levy", "x", nil, nil, nil, 1, nil, false},
		{"levy with category", "levy", "x", 1, nil, nil, 1, 1, false},
		{"levy with subcategory", "levy", "x", nil, 1, nil, 1, 1, false},
		{"levy with holding", "levy", "x", nil, nil, 1, 1, 1, false},
		{"levy with unknown levy category", "levy", "x", nil, nil, nil, 99, 1, false},
		{"income with levy category", "income", "x", 1, 1, nil, 1, nil, false},
		{"expense with levy subcategory", "expense", "x", 1, 1, nil, nil, 1, false},
		{"investment with levy category", "investment", nil, nil, nil, 1, 1, nil, false},
		{"divestment with levy subcategory", "divestment", nil, nil, nil, 1, nil, 1, false},
	}
	for _, c := range cases {
		err := insert(c.typ, c.desc, c.cat, c.sub, c.hld, c.lcat, c.lsub)
		if c.ok && err != nil {
			t.Errorf("%s: rejected: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
}

func TestLevyCategoriesAreSeparateFromCategories(t *testing.T) {
	s, _ := open(t)
	if _, err := s.EnsureCategory("Tax"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnsureLevyCategory("tax"); err != nil {
		t.Fatal(err)
	}
	cats, _ := s.ListNamed(store.CategoryList)
	levyCats, _ := s.ListNamed(store.LevyCategoryList)
	if len(cats) != 1 || cats[0].Name != "Tax" || len(levyCats) != 1 || levyCats[0].Name != "tax" {
		t.Errorf("categories = %+v, levy categories = %+v", cats, levyCats)
	}
}

func TestLevyCategoryWithTransactionsCannotBeDeleted(t *testing.T) {
	s, raw := open(t)
	if _, err := s.AddTransaction(levy(t, s, "2026-10-05", "Tax")); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`DELETE FROM levy_categories`, `DELETE FROM levy_subcategories`} {
		if _, err := raw.Exec(q); err == nil {
			t.Errorf("%s accepted while a Levy uses it", q)
		}
	}
}

func TestNoLevyCategoriesAreSeeded(t *testing.T) {
	s, _ := open(t)
	for _, l := range []store.List{store.LevyCategoryList, store.LevySubcategoryList} {
		if got, err := s.ListNamed(l); err != nil || len(got) != 0 {
			t.Errorf("%s seeded: %+v, err = %v", l.Label, got, err)
		}
	}
}

func TestDuplicateLevyIsRejected(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(levy(t, s, "2026-10-05", "Income tax")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransaction(levy(t, s, "2026-10-05", "INCOME TAX")); !errors.Is(err, store.ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
	// The same date and description under another Type is allowed.
	if _, err := s.AddTransaction(tx(t, s, "2026-10-05", 5, "Income tax", store.TypeExpense)); err != nil {
		t.Errorf("expense with a levy's date and description rejected: %v", err)
	}
}
