package store_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

func idOf(t *testing.T, s *store.Store, l store.List, name string) int64 {
	t.Helper()
	id, err := s.EnsureNamed(l, name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func names(t *testing.T, s *store.Store, l store.List) []string {
	t.Helper()
	all, err := s.ListNamed(l)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, n := range all {
		out = append(out, n.Name)
	}
	return out
}

func TestRenameNamedRenamesAndPropagatesToTransactions(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(expense(t, s, "2026-10-05", "Lunch")); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameNamed(store.CategoryList, idOf(t, s, store.CategoryList, "C"), "  Food "); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 1 || rows[0].Category != "Food" {
		t.Errorf("rows = %+v, err = %v", rows, err)
	}
}

func TestRenameNamedAllowsACaseOnlyRenameOfItself(t *testing.T) {
	s, _ := open(t)
	id := idOf(t, s, store.CategoryList, "food")
	if err := s.RenameNamed(store.CategoryList, id, "Food"); err != nil {
		t.Fatal(err)
	}
	if got := names(t, s, store.CategoryList); len(got) != 1 || got[0] != "Food" {
		t.Errorf("names = %v", got)
	}
}

func TestRenameNamedRefusesAnotherEntrysNameIgnoringCase(t *testing.T) {
	s, _ := open(t)
	a := idOf(t, s, store.CategoryList, "Food")
	idOf(t, s, store.CategoryList, "Rent")
	if err := s.RenameNamed(store.CategoryList, a, "rENT"); !errors.Is(err, store.ErrNameTaken) {
		t.Errorf("err = %v, want ErrNameTaken", err)
	}
	if got := names(t, s, store.CategoryList); fmt.Sprint(got) != "[Food Rent]" {
		t.Errorf("names = %v", got)
	}
}

func TestRenameNamedAllowsANameUsedInAnotherList(t *testing.T) {
	s, _ := open(t)
	idOf(t, s, store.SubcategoryList, "Rent")
	if err := s.RenameNamed(store.CategoryList, idOf(t, s, store.CategoryList, "Food"), "Rent"); err != nil {
		t.Fatal(err)
	}
}

func TestRenameNamedOfAMissingEntryIsNotFound(t *testing.T) {
	s, _ := open(t)
	if err := s.RenameNamed(store.CategoryList, 99, "Food"); !errors.Is(err, store.ErrEntryNotFound) {
		t.Errorf("err = %v, want ErrEntryNotFound", err)
	}
}

func TestNamedPageIsThePageTheEntrySortsOn(t *testing.T) {
	s, _ := open(t)
	var last int64
	for i := 1; i <= 7; i++ {
		last = idOf(t, s, store.CategoryList, fmt.Sprintf("Entry %d", i))
	}
	if page, err := s.NamedPage(store.CategoryList, last); err != nil || page != 2 {
		t.Errorf("page = %d, err = %v, want 2", page, err)
	}
	if _, err := s.NamedPage(store.CategoryList, 99); !errors.Is(err, store.ErrEntryNotFound) {
		t.Errorf("err = %v, want ErrEntryNotFound", err)
	}
}

func TestDeleteNamedRemovesUnusedRefusesUsedAndReportsMissing(t *testing.T) {
	s, _ := open(t)
	unused := idOf(t, s, store.CategoryList, "Unused")
	if err := s.DeleteNamed(store.CategoryList, unused); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNamed(store.CategoryList, unused); !errors.Is(err, store.ErrEntryNotFound) {
		t.Errorf("err = %v, want ErrEntryNotFound", err)
	}
	if _, err := s.AddTransaction(expense(t, s, "2026-10-05", "Lunch")); err != nil {
		t.Fatal(err)
	}
	used := idOf(t, s, store.CategoryList, "C")
	if err := s.DeleteNamed(store.CategoryList, used); !errors.Is(err, store.ErrEntryInUse) {
		t.Errorf("err = %v, want ErrEntryInUse", err)
	}
	if got := names(t, s, store.CategoryList); len(got) != 1 {
		t.Errorf("names = %v, want the used entry to stay", got)
	}
}
