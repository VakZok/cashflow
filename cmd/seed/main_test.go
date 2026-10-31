package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VakZok/Cashflow/internal/store"
)

// today is mid-month so the test sees whether days after it are filled too.
var today = time.Date(2026, time.October, 15, 9, 30, 0, 0, time.Local)

func seeded(t *testing.T, months int) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "demo.db")
	if _, err := run(path, months, today); err != nil {
		t.Fatalf("run: %v", err) // a Duplicate would fail here: the database rejects it
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

// TestSeedDefaultLength checks one seed of the default 12 months.
func TestSeedDefaultLength(t *testing.T) {
	t.Parallel()
	s, _ := seeded(t, 12)
	rows, err := s.ListTransactions()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("fills whole months ending with the current one", func(t *testing.T) {
		months := map[string]bool{}
		afterToday := false
		for _, r := range rows {
			if r.Date < "2025-11-01" || r.Date > "2026-10-31" {
				t.Errorf("Transaction dated %s, outside November 2025 to October 2026", r.Date)
			}
			months[r.Date[:7]] = true
			if r.Date > "2026-10-15" {
				afterToday = true
			}
		}
		if len(months) != 12 {
			t.Errorf("Transactions in %d months, want 12: %v", len(months), months)
		}
		if !afterToday {
			t.Error("no Transaction after today in the current month, want the month filled in full")
		}
	})

	t.Run("covers every Type and list", func(t *testing.T) {
		byType := map[string]int{}
		for _, r := range rows {
			byType[r.Type]++
		}
		for _, spec := range store.Types {
			if byType[spec.Type] == 0 {
				t.Errorf("no %s Transaction", spec.Label)
			}
		}
		for _, l := range []store.List{store.CategoryList, store.SubcategoryList,
			store.LevyCategoryList, store.LevySubcategoryList, store.HoldingList} {
			if n, err := s.CountNamed(l); err != nil || n == 0 {
				t.Errorf("%s list has %d entries (err %v), want some", l.Label, n, err)
			}
		}
		if holdings, err := s.CountNamed(store.HoldingList); err != nil || holdings != 3 {
			t.Errorf("%d Holdings (err %v), want 3", holdings, err)
		}
	})

	t.Run("saves a Repeating list without Duplicates", func(t *testing.T) {
		entries, err := s.ListRepeating()
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 5 {
			t.Fatalf("Repeating list has %d entries, want 5", len(entries))
		}
		seen := map[string]bool{}
		for _, e := range entries {
			key := e.Type + "|" + strings.ToLower(e.Description)
			if e.HoldingID != 0 {
				key = fmt.Sprintf("holding|%d", e.HoldingID)
			}
			if seen[key] {
				t.Errorf("Repeating entries collide on %q", key)
			}
			seen[key] = true
		}
	})

	t.Run("is the same when seeded again on the same day", func(t *testing.T) {
		again, _ := seeded(t, 12)
		rowsAgain, err := again.ListTransactions()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rows, rowsAgain) {
			t.Error("two seeds on the same day differ")
		}
	})
}

func TestSeedMonthsFlagOverridesTheLength(t *testing.T) {
	t.Parallel()
	s, _ := seeded(t, 2)
	rows, err := s.ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Date < "2026-09-01" {
			t.Fatalf("Transaction dated %s, want none before September 2026", r.Date)
		}
	}
}

func TestSeedRefusesANonEmptyDatabase(t *testing.T) {
	t.Parallel()
	s, path := seeded(t, 1)
	before, err := s.CountTransactions(store.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run(path, 1, today); err == nil {
		t.Fatal("second seed succeeded, want it refused")
	}
	if after, err := s.CountTransactions(store.Filter{}); err != nil || after != before {
		t.Errorf("refused seed changed the Transactions: %d before, %d after (err %v)", before, after, err)
	}
}
