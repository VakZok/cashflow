package store_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

// open returns a Store on a fresh database file plus a raw connection to the
// same file, for inserting values the Go API would never produce.
func open(t *testing.T) (*store.Store, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	raw, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return s, raw
}

// tx builds a Transaction that satisfies the per-Type field rules: Income and
// Expense get a Category and Subcategory plus the description, Investment and
// Divestment get a Holding and no description, Opening balance gets only the
// description.
func tx(t *testing.T, s *store.Store, date string, amount int64, desc, typ string) store.NewTransaction {
	t.Helper()
	n := store.NewTransaction{Date: date, Amount: amount, Description: desc, Type: typ}
	switch typ {
	case store.TypeIncome, store.TypeExpense:
		var err error
		if n.CategoryID, err = s.EnsureCategory("C"); err != nil {
			t.Fatal(err)
		}
		if n.SubcategoryID, err = s.EnsureSubcategory("S"); err != nil {
			t.Fatal(err)
		}
	case store.TypeInvestment, store.TypeDivestment:
		var err error
		n.Description = ""
		if n.HoldingID, err = s.EnsureHolding("H"); err != nil {
			t.Fatal(err)
		}
	}
	return n
}

func TestDateConstraint(t *testing.T) {
	s, _ := open(t)
	good := []string{"2000-01-01", "2200-12-31", "2024-02-29", "2026-10-05"}
	bad := []string{
		"1999-12-31", "2201-01-01", "0001-01-01", "9999-12-31", // range
		"2026-00-10", "2026-13-10", "2026-19-10", // month
		"2026-10-00", "2026-10-32", "2026-10-39", // day
		"2026-02-30", "2025-02-29", "2026-04-31", // not a real day
		"2026-1-05", "2026/10/05", "05-10-2026", "2026-10-05 ", "2026-10-05T10:00", "", // format
	}
	for _, d := range good {
		if _, err := s.AddTransaction(tx(t, s, d, 5, "ok", store.TypeIncome)); err != nil {
			t.Errorf("date %q rejected: %v", d, err)
		}
	}
	for _, d := range bad {
		if _, err := s.AddTransaction(tx(t, s, d, 5, "bad", store.TypeIncome)); err == nil {
			t.Errorf("date %q accepted", d)
		}
	}
}

func TestAmountConstraint(t *testing.T) {
	s, raw := open(t)
	for _, a := range []int64{1, 100_000_000} {
		if _, err := s.AddTransaction(tx(t, s, "2026-10-05", a, "ok", store.TypeIncome)); err != nil {
			t.Errorf("amount %d rejected: %v", a, err)
		}
	}
	for _, a := range []int64{0, -5, 100_000_001} {
		if _, err := s.AddTransaction(tx(t, s, "2026-10-05", a, "bad", store.TypeIncome)); err == nil {
			t.Errorf("amount %d accepted", a)
		}
	}
	// SQLite would happily store these in an INTEGER column without typeof().
	for _, a := range []any{"12,5", "abc", "1.5", 1.5, "", "-5", "5"} {
		_, err := raw.Exec(`INSERT INTO transactions (date, amount, description, type) VALUES ('2026-10-05', ?, 'x', 'opening_balance')`, a)
		if a == "5" {
			// Numeric text is converted to an integer by column affinity: fine.
			if err != nil {
				t.Errorf("amount %#v rejected: %v", a, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("amount %#v accepted", a)
		}
	}
}

func TestTextLengthConstraints(t *testing.T) {
	s, _ := open(t)
	ok := strings.Repeat("ä", 50)
	if _, err := s.EnsureCategory(ok); err != nil {
		t.Errorf("50-char category rejected: %v", err)
	}
	if _, err := s.EnsureSubcategory(ok); err != nil {
		t.Errorf("50-char subcategory rejected: %v", err)
	}
	if _, err := s.EnsureCategory(strings.Repeat("a", 51)); err == nil {
		t.Error("51-char category accepted")
	}
	if _, err := s.EnsureSubcategory(strings.Repeat("a", 51)); err == nil {
		t.Error("51-char subcategory accepted")
	}
	if _, err := s.EnsureCategory("   "); err == nil {
		t.Error("blank category accepted")
	}
	if _, err := s.AddTransaction(tx(t, s, "2026-10-05", 5, strings.Repeat("d", 100), store.TypeIncome)); err != nil {
		t.Errorf("100-char description rejected: %v", err)
	}
	for _, d := range []string{strings.Repeat("d", 101), "", "   "} {
		if _, err := s.AddTransaction(tx(t, s, "2026-10-05", 5, d, store.TypeIncome)); err == nil {
			t.Errorf("description %q accepted", d)
		}
	}
}

func TestTypeConstraint(t *testing.T) {
	s, _ := open(t)
	for _, typ := range []string{store.TypeOpeningBalance, store.TypeIncome, store.TypeExpense, store.TypeInvestment, store.TypeDivestment} {
		if _, err := s.AddTransaction(tx(t, s, "2026-10-05", 5, "ok", typ)); err != nil {
			t.Errorf("type %q rejected: %v", typ, err)
		}
	}
	for _, typ := range []string{"taxes", "Income", ""} {
		if _, err := s.AddTransaction(tx(t, s, "2026-10-05", 5, "bad", typ)); err == nil {
			t.Errorf("type %q accepted", typ)
		}
	}
}

func TestInjectionAttemptsAreStoredAsPlainText(t *testing.T) {
	s, _ := open(t)
	evil := "x'); DROP TABLE transactions;--"
	id, err := s.EnsureCategory(evil)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := s.EnsureSubcategory("S")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransaction(store.NewTransaction{
		Date: "2026-10-05", Amount: 5, Description: evil, Type: store.TypeExpense, CategoryID: id, SubcategoryID: sub,
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 1 || rows[0].Description != evil || rows[0].Category != evil {
		t.Errorf("rows = %+v, err = %v", rows, err)
	}
}

func TestHoldingNameConstraints(t *testing.T) {
	s, _ := open(t)
	if _, err := s.EnsureHolding(strings.Repeat("ä", 50)); err != nil {
		t.Errorf("50-char holding rejected: %v", err)
	}
	for _, name := range []string{strings.Repeat("a", 51), "   ", ""} {
		if _, err := s.EnsureHolding(name); err == nil {
			t.Errorf("holding %q accepted", name)
		}
	}
	a, _ := s.EnsureHolding("ETF World")
	b, err := s.EnsureHolding("etf world")
	if err != nil || a != b {
		t.Errorf("case-insensitive reuse: %d vs %d (%v)", a, b, err)
	}
	// The database itself enforces uniqueness, not only EnsureHolding.
	_, raw := open(t)
	_, _ = raw.Exec(`INSERT INTO holdings (name) VALUES ('Gold')`)
	if _, err := raw.Exec(`INSERT INTO holdings (name) VALUES ('GOLD')`); err == nil {
		t.Error("duplicate holding differing in case accepted")
	}
}

// TestTypeFieldRules inserts directly via SQL, bypassing the Go API, to prove
// the per-Type CHECK ties each Type to its allowed fields.
func TestTypeFieldRules(t *testing.T) {
	_, raw := open(t)
	for _, q := range []string{
		`INSERT INTO categories (name) VALUES ('C')`,
		`INSERT INTO subcategories (name) VALUES ('S')`,
		`INSERT INTO holdings (name) VALUES ('H')`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	insert := func(typ string, desc, cat, sub, hold any) error {
		_, err := raw.Exec(`INSERT INTO transactions (date, amount, description, type, category_id, subcategory_id, holding_id)
			VALUES ('2026-10-05', 5, ?, ?, ?, ?, ?)`, desc, typ, cat, sub, hold)
		return err
	}
	cases := []struct {
		name                string
		typ                 string
		desc, cat, sub, hld any
		ok                  bool
	}{
		{"income complete", "income", "x", 1, 1, nil, true},
		{"expense complete", "expense", "x", 1, 1, nil, true},
		{"income without description", "income", nil, 1, 1, nil, false},
		{"income without category", "income", "x", nil, 1, nil, false},
		{"expense without subcategory", "expense", "x", 1, nil, nil, false},
		{"expense with holding", "expense", "x", 1, 1, 1, false},
		{"income with holding", "income", "x", 1, 1, 1, false},
		{"investment complete", "investment", nil, nil, nil, 1, true},
		{"divestment complete", "divestment", nil, nil, nil, 1, true},
		{"investment without holding", "investment", nil, nil, nil, nil, false},
		{"investment with description", "investment", "x", nil, nil, 1, false},
		{"divestment with category", "divestment", nil, 1, nil, 1, false},
		{"divestment with subcategory", "divestment", nil, nil, 1, 1, false},
		{"investment unknown holding", "investment", nil, nil, nil, 99, false},
		{"opening balance unconstrained", "opening_balance", "x", nil, nil, nil, true},
	}
	for _, c := range cases {
		err := insert(c.typ, c.desc, c.cat, c.sub, c.hld)
		if c.ok && err != nil {
			t.Errorf("%s: rejected: %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
}

func TestListShowsHoldingForInvestments(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(tx(t, s, "2026-10-05", 100, "", store.TypeInvestment)); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 1 || rows[0].Holding != "H" || rows[0].Description != "" {
		t.Errorf("rows = %+v, err = %v", rows, err)
	}
}
