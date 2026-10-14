package store_test

import (
	"errors"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

func expense(t *testing.T, s *store.Store, date, desc string) store.NewTransaction {
	t.Helper()
	n := store.NewTransaction{Date: date, Amount: 7, Description: desc, Type: store.TypeExpense}
	var err error
	if n.CategoryID, err = s.EnsureCategory("C"); err != nil {
		t.Fatal(err)
	}
	if n.SubcategoryID, err = s.EnsureSubcategory("S"); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestUpdateChangingTypeReplacesTheFieldsTheTypeTakes(t *testing.T) {
	s, _ := open(t)
	id, err := s.AddTransaction(expense(t, s, "2026-10-05", "Lunch"))
	if err != nil {
		t.Fatal(err)
	}
	holding, err := s.EnsureHolding("ETF")
	if err != nil {
		t.Fatal(err)
	}
	err = s.UpdateTransaction(id, store.NewTransaction{Date: "2026-10-06", Amount: 90, Type: store.TypeInvestment, HoldingID: holding})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %+v, err = %v", rows, err)
	}
	want := store.TransactionRow{ID: id, Date: "2026-10-06", Amount: 90, Type: store.TypeInvestment, Holding: "ETF"}
	if rows[0] != want {
		t.Errorf("row = %+v, want %+v", rows[0], want)
	}
}

func TestUpdateMissingTransactionIsNotFound(t *testing.T) {
	s, _ := open(t)
	err := s.UpdateTransaction(99, store.NewTransaction{Date: "2026-10-06", Amount: 1, Type: store.TypeInvestment})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteRemovesTheTransaction(t *testing.T) {
	s, _ := open(t)
	id, err := s.AddTransaction(expense(t, s, "2026-10-05", "Lunch"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTransaction(id); err != nil {
		t.Fatal(err)
	}
	if rows, _ := s.ListTransactions(); len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
	if err := s.DeleteTransaction(id); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestUpdateIsNeverADuplicateOfItselfButOfOthers(t *testing.T) {
	s, _ := open(t)
	a, err := s.AddTransaction(expense(t, s, "2026-10-05", "Lunch"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransaction(expense(t, s, "2026-10-06", "Dinner")); err != nil {
		t.Fatal(err)
	}
	same := expense(t, s, "2026-10-05", "lunch")
	same.Amount = 9
	if err := s.UpdateTransaction(a, same); err != nil {
		t.Errorf("update to own date and description: %v", err)
	}
	clash := expense(t, s, "2026-10-06", "DINNER")
	if err := s.UpdateTransaction(a, clash); !errors.Is(err, store.ErrDuplicate) {
		t.Errorf("update onto another Transaction: err = %v, want ErrDuplicate", err)
	}
	taken, err := s.DescriptionTaken("2026-10-05", "Lunch", store.TypeExpense, a)
	if err != nil || taken {
		t.Errorf("DescriptionTaken excluding itself = %v, %v; want false", taken, err)
	}
	taken, err = s.DescriptionTaken("2026-10-05", "Lunch", store.TypeExpense, 0)
	if err != nil || !taken {
		t.Errorf("DescriptionTaken without exclusion = %v, %v; want true", taken, err)
	}
}

func TestGetTransactionReturnsTheStoredFields(t *testing.T) {
	s, _ := open(t)
	n := expense(t, s, "2026-10-05", "Lunch")
	id, err := s.AddTransaction(n)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTransaction(id)
	if err != nil || got != n {
		t.Errorf("got %+v, %v; want %+v", got, err, n)
	}
	if _, err := s.GetTransaction(id + 1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing: err = %v, want ErrNotFound", err)
	}
}
