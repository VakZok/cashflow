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

func tx(date string, amount int64, desc, typ string) store.NewTransaction {
	return store.NewTransaction{Date: date, Amount: amount, Description: desc, Type: typ}
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
		if _, err := s.AddTransaction(tx(d, 5, "ok", store.TypeIncome)); err != nil {
			t.Errorf("date %q rejected: %v", d, err)
		}
	}
	for _, d := range bad {
		if _, err := s.AddTransaction(tx(d, 5, "bad", store.TypeIncome)); err == nil {
			t.Errorf("date %q accepted", d)
		}
	}
}

func TestAmountConstraint(t *testing.T) {
	s, raw := open(t)
	for _, a := range []int64{1, 100_000_000} {
		if _, err := s.AddTransaction(tx("2026-10-05", a, "ok", store.TypeIncome)); err != nil {
			t.Errorf("amount %d rejected: %v", a, err)
		}
	}
	for _, a := range []int64{0, -5, 100_000_001} {
		if _, err := s.AddTransaction(tx("2026-10-05", a, "bad", store.TypeIncome)); err == nil {
			t.Errorf("amount %d accepted", a)
		}
	}
	// SQLite would happily store these in an INTEGER column without typeof().
	for _, a := range []any{"12,5", "abc", "1.5", 1.5, "", "-5", "5"} {
		_, err := raw.Exec(`INSERT INTO transactions (date, amount, description, type) VALUES ('2026-10-05', ?, 'x', 'income')`, a)
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
	if _, err := s.AddTransaction(tx("2026-10-05", 5, strings.Repeat("d", 100), store.TypeIncome)); err != nil {
		t.Errorf("100-char description rejected: %v", err)
	}
	for _, d := range []string{strings.Repeat("d", 101), "", "   "} {
		if _, err := s.AddTransaction(tx("2026-10-05", 5, d, store.TypeIncome)); err == nil {
			t.Errorf("description %q accepted", d)
		}
	}
}

func TestTypeConstraint(t *testing.T) {
	s, _ := open(t)
	for _, typ := range []string{store.TypeOpeningBalance, store.TypeIncome, store.TypeExpense, store.TypeInvestment, store.TypeDivestment} {
		if _, err := s.AddTransaction(tx("2026-10-05", 5, "ok", typ)); err != nil {
			t.Errorf("type %q rejected: %v", typ, err)
		}
	}
	for _, typ := range []string{"taxes", "Income", ""} {
		if _, err := s.AddTransaction(tx("2026-10-05", 5, "bad", typ)); err == nil {
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
	if _, err := s.AddTransaction(store.NewTransaction{
		Date: "2026-10-05", Amount: 5, Description: evil, Type: store.TypeExpense, CategoryID: id,
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 1 || rows[0].Description != evil || rows[0].Category != evil {
		t.Errorf("rows = %+v, err = %v", rows, err)
	}
}
