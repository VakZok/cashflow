package store_test

import (
	"errors"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

func repeatingEntry(t *testing.T, s *store.Store, typ, desc string, amount int64) store.RepeatingEntry {
	t.Helper()
	c, err := s.EnsureCategory("C")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := s.EnsureSubcategory("S")
	if err != nil {
		t.Fatal(err)
	}
	return store.RepeatingEntry{Type: typ, Amount: amount, Description: desc, CategoryID: c, SubcategoryID: sc}
}

func TestRepeatingListIsEmptyAtFirst(t *testing.T) {
	s, _ := open(t)
	got, err := s.ListRepeating()
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestRepeatingIsListedByTypeThenSavedOrder(t *testing.T) {
	s, _ := open(t)
	in := []store.RepeatingEntry{
		repeatingEntry(t, s, store.TypeExpense, "Rent", 800),
		repeatingEntry(t, s, store.TypeIncome, "Salary", 2000),
		repeatingEntry(t, s, store.TypeExpense, "Gym", 30),
		repeatingEntry(t, s, store.TypeIncome, "Pension", 100),
	}
	if err := s.ReplaceRepeating(in); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListRepeating()
	if err != nil {
		t.Fatal(err)
	}
	var descs []string
	for _, e := range got {
		descs = append(descs, e.Description)
	}
	want := []string{"Salary", "Pension", "Rent", "Gym"}
	if len(descs) != len(want) {
		t.Fatalf("got %v, want %v", descs, want)
	}
	for i := range want {
		if descs[i] != want[i] {
			t.Fatalf("got %v, want %v", descs, want)
		}
	}
	if got[0].Amount != 2000 || got[0].CategoryID == 0 || got[0].SubcategoryID == 0 {
		t.Errorf("first entry = %+v", got[0])
	}
}

func TestReplaceRepeatingReplacesTheWholeList(t *testing.T) {
	s, _ := open(t)
	if err := s.ReplaceRepeating([]store.RepeatingEntry{repeatingEntry(t, s, store.TypeIncome, "Salary", 1)}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceRepeating([]store.RepeatingEntry{repeatingEntry(t, s, store.TypeExpense, "Rent", 2)}); err != nil {
		t.Fatal(err)
	}
	got, _ := s.ListRepeating()
	if len(got) != 1 || got[0].Description != "Rent" {
		t.Fatalf("got %+v", got)
	}
	if err := s.ReplaceRepeating(nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ListRepeating(); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestReplaceRepeatingIsAtomic(t *testing.T) {
	s, _ := open(t)
	good := repeatingEntry(t, s, store.TypeIncome, "Salary", 5)
	if err := s.ReplaceRepeating([]store.RepeatingEntry{good}); err != nil {
		t.Fatal(err)
	}
	bad := repeatingEntry(t, s, store.TypeExpense, "Rent", 0) // amount violates the CHECK
	if err := s.ReplaceRepeating([]store.RepeatingEntry{good, bad}); err == nil {
		t.Fatal("want an error")
	}
	got, _ := s.ListRepeating()
	if len(got) != 1 || got[0].Description != "Salary" {
		t.Fatalf("old list not kept: %+v", got)
	}
}

func TestReplaceRepeatingRejectsMoreThanFifty(t *testing.T) {
	s, _ := open(t)
	var in []store.RepeatingEntry
	for i := 0; i < store.MaxRepeating+1; i++ {
		in = append(in, repeatingEntry(t, s, store.TypeIncome, "x", 1))
	}
	if err := s.ReplaceRepeating(in); !errors.Is(err, store.ErrTooManyRepeating) {
		t.Fatalf("err = %v", err)
	}
	if err := s.ReplaceRepeating(in[:store.MaxRepeating]); err != nil {
		t.Fatalf("50 rows: %v", err)
	}
}

func TestRepeatingBlocksDeletingItsCategoryAndSubcategory(t *testing.T) {
	s, raw := open(t)
	if err := s.ReplaceRepeating([]store.RepeatingEntry{repeatingEntry(t, s, store.TypeIncome, "Salary", 5)}); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DELETE FROM categories`); err == nil {
		t.Error("deleting a Category used by a Repeating row succeeded")
	}
	if _, err := raw.Exec(`DELETE FROM subcategories`); err == nil {
		t.Error("deleting a Subcategory used by a Repeating row succeeded")
	}
	if err := s.ReplaceRepeating(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DELETE FROM categories`); err != nil {
		t.Errorf("unused Category not deletable: %v", err)
	}
}

func TestRepeatingIsNotATransaction(t *testing.T) {
	s, _ := open(t)
	if err := s.ReplaceRepeating([]store.RepeatingEntry{repeatingEntry(t, s, store.TypeIncome, "Salary", 5)}); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListTransactions()
	income, expense, _ := s.Totals("2000-01-01", "2200-01-01")
	if len(rows) != 0 || income != 0 || expense != 0 {
		t.Errorf("rows %v income %d expense %d", rows, income, expense)
	}
}
