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
	income, expense, _ := s.Totals("0001-01-01", "9999-01-01")
	if len(rows) != 0 || income != 0 || expense != 0 {
		t.Errorf("rows %v income %d expense %d", rows, income, expense)
	}
}

func saveRepeating(t *testing.T, s *store.Store, in ...store.RepeatingEntry) {
	t.Helper()
	if err := s.ReplaceRepeating(in); err != nil {
		t.Fatal(err)
	}
}

func TestBookRepeatingCreatesOneIndependentTransactionPerRow(t *testing.T) {
	s, _ := open(t)
	saveRepeating(t, s,
		repeatingEntry(t, s, store.TypeExpense, "Rent", 800),
		repeatingEntry(t, s, store.TypeIncome, "Salary", 2000))
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 2 || skipped != 0 {
		t.Fatalf("booked %d skipped %d err %v", booked, skipped, err)
	}
	rows, _ := s.ListTransactions()
	if len(rows) != 2 {
		t.Fatalf("got %+v", rows)
	}
	for _, r := range rows {
		if r.Date != "2026-10-01" || r.Category != "C" || r.Subcategory != "S" {
			t.Errorf("row %+v", r)
		}
	}
	income, expense, _ := s.Totals("2026-10-01", "2026-11-01")
	if income != 2000 || expense != 800 {
		t.Errorf("income %d expense %d", income, expense)
	}

	// The list is unchanged, and editing it leaves the booked copies alone.
	if got, _ := s.ListRepeating(); len(got) != 2 {
		t.Errorf("list changed: %+v", got)
	}
	saveRepeating(t, s)
	if rows, _ := s.ListTransactions(); len(rows) != 2 {
		t.Errorf("booked Transactions changed: %+v", rows)
	}
}

func TestBookRepeatingSkipsDuplicatesAndBooksTheRest(t *testing.T) {
	s, _ := open(t)
	e := repeatingEntry(t, s, store.TypeExpense, "Rent", 800)
	saveRepeating(t, s, e,
		repeatingEntry(t, s, store.TypeIncome, "Salary", 2000),
		repeatingEntry(t, s, store.TypeExpense, "Gym", 30))
	if _, err := s.AddTransaction(store.NewTransaction{Date: "2026-10-01", Amount: 1, Description: "rent", // ignoring ASCII case
		Type: store.TypeExpense, CategoryID: e.CategoryID, SubcategoryID: e.SubcategoryID}); err != nil {
		t.Fatal(err)
	}
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 2 || skipped != 1 {
		t.Fatalf("booked %d skipped %d err %v", booked, skipped, err)
	}
	rows, _ := s.ListTransactions()
	got := map[string]int64{}
	for _, r := range rows {
		got[r.Description] = r.Amount
	}
	if len(rows) != 3 || got["rent"] != 1 || got["Salary"] != 2000 || got["Gym"] != 30 {
		t.Errorf("rows %+v", rows)
	}
}

func TestBookRepeatingSkipsARowCollidingWithAnEarlierOne(t *testing.T) {
	s, _ := open(t)
	// Save List rejects such a list; the store still books it without failing.
	saveRepeating(t, s,
		repeatingEntry(t, s, store.TypeIncome, "Salary", 2000),
		repeatingEntry(t, s, store.TypeExpense, "Rent", 800),
		repeatingEntry(t, s, store.TypeExpense, "rent", 5))
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 2 || skipped != 1 {
		t.Fatalf("booked %d skipped %d err %v", booked, skipped, err)
	}
}

func TestBookRepeatingTwiceForOneDateSkipsEverything(t *testing.T) {
	s, _ := open(t)
	saveRepeating(t, s, repeatingEntry(t, s, store.TypeExpense, "Rent", 800))
	if _, _, err := s.BookRepeating("2026-10-01"); err != nil {
		t.Fatal(err)
	}
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 0 || skipped != 1 {
		t.Fatalf("booked %d skipped %d err %v", booked, skipped, err)
	}
	if rows, _ := s.ListTransactions(); len(rows) != 1 {
		t.Errorf("rows %+v", rows)
	}
	if booked, _, err := s.BookRepeating("2026-11-01"); err != nil || booked != 1 {
		t.Fatalf("next month: booked %d err %v", booked, err)
	}
}

func levyRepeatingEntry(t *testing.T, s *store.Store, typ, desc string, amount int64) store.RepeatingEntry {
	t.Helper()
	c, err := s.EnsureLevyCategory("LC")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := s.EnsureLevySubcategory("LS")
	if err != nil {
		t.Fatal(err)
	}
	return store.RepeatingEntry{Type: typ, Amount: amount, Description: desc, LevyCategoryID: c, LevySubcategoryID: sc}
}

func TestRepeatingListsLevyAndRebateAfterTheOtherTypesInSavedOrder(t *testing.T) {
	s, _ := open(t)
	in := []store.RepeatingEntry{
		levyRepeatingEntry(t, s, store.TypeRebate, "R1", 1),
		levyRepeatingEntry(t, s, store.TypeLevy, "L1", 2),
		repeatingEntry(t, s, store.TypeExpense, "E1", 3),
		levyRepeatingEntry(t, s, store.TypeRebate, "R2", 4),
		repeatingEntry(t, s, store.TypeIncome, "I1", 5),
		levyRepeatingEntry(t, s, store.TypeLevy, "L2", 6),
	}
	if err := s.ReplaceRepeating(in); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListRepeating()
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, e := range got {
		order = append(order, e.Description)
	}
	want := []string{"I1", "E1", "L1", "L2", "R1", "R2"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
	if got[2].LevyCategoryID == 0 || got[2].LevySubcategoryID == 0 || got[2].CategoryID != 0 {
		t.Errorf("Levy lists not stored on the Levy row: %+v", got[2])
	}
}

func TestRepeatingBlocksDeletingItsLevyCategoryAndLevySubcategory(t *testing.T) {
	s, raw := open(t)
	if err := s.ReplaceRepeating([]store.RepeatingEntry{levyRepeatingEntry(t, s, store.TypeLevy, "Tax", 5)}); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DELETE FROM levy_categories`); err == nil {
		t.Error("deleting a Levy Category used by a Repeating row succeeded")
	}
	if _, err := raw.Exec(`DELETE FROM levy_subcategories`); err == nil {
		t.Error("deleting a Levy Subcategory used by a Repeating row succeeded")
	}
	if err := s.ReplaceRepeating(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`DELETE FROM levy_categories`); err != nil {
		t.Errorf("unused Levy Category not deletable: %v", err)
	}
	if _, err := raw.Exec(`DELETE FROM levy_subcategories`); err != nil {
		t.Errorf("unused Levy Subcategory not deletable: %v", err)
	}
}

func TestBookRepeatingBooksLevyAndRebateWithTheirLevyLists(t *testing.T) {
	s, _ := open(t)
	saveRepeating(t, s, levyRepeatingEntry(t, s, store.TypeLevy, "Tax", 250), levyRepeatingEntry(t, s, store.TypeRebate, "Tax", 40))
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 2 || skipped != 0 {
		t.Fatalf("booked %d, skipped %d, err %v", booked, skipped, err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %v, err %v", rows, err)
	}
	for _, r := range rows {
		if r.Category != "LC" || r.Subcategory != "LS" {
			t.Errorf("%s booked with %q/%q, want LC/LS", r.Type, r.Category, r.Subcategory)
		}
	}
}

func TestBookRepeatingRebateIsNotADuplicateOfIncomeButIsOfARebate(t *testing.T) {
	s, _ := open(t)
	if _, err := s.AddTransaction(store.NewTransaction{Date: "2026-10-01", Amount: 40, Description: "Refund", Type: store.TypeIncome,
		CategoryID: repeatingEntry(t, s, store.TypeIncome, "x", 1).CategoryID, SubcategoryID: repeatingEntry(t, s, store.TypeIncome, "x", 1).SubcategoryID}); err != nil {
		t.Fatal(err)
	}
	saveRepeating(t, s, levyRepeatingEntry(t, s, store.TypeRebate, "refund", 40))
	if booked, skipped, err := s.BookRepeating("2026-10-01"); err != nil || booked != 1 || skipped != 0 {
		t.Fatalf("booked %d, skipped %d, err %v", booked, skipped, err)
	}
	// Booking again collides with the Rebate just booked.
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 0 || skipped != 1 {
		t.Fatalf("booked %d, skipped %d, err %v", booked, skipped, err)
	}
}

func holdingRepeatingEntry(t *testing.T, s *store.Store, typ, holding string, amount int64) store.RepeatingEntry {
	t.Helper()
	id, err := s.EnsureNamed(store.HoldingList, holding)
	if err != nil {
		t.Fatal(err)
	}
	return store.RepeatingEntry{Type: typ, Amount: amount, HoldingID: id}
}

func TestRepeatingListsInvestmentThenDivestmentBetweenExpenseAndLevyInSavedOrder(t *testing.T) {
	s, _ := open(t)
	in := []store.RepeatingEntry{
		levyRepeatingEntry(t, s, store.TypeLevy, "L1", 1),
		holdingRepeatingEntry(t, s, store.TypeDivestment, "D1", 2),
		holdingRepeatingEntry(t, s, store.TypeInvestment, "I1", 3),
		repeatingEntry(t, s, store.TypeExpense, "E1", 4),
		holdingRepeatingEntry(t, s, store.TypeInvestment, "I2", 5),
	}
	saveRepeating(t, s, in...)
	got, err := s.ListRepeating()
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{4, 3, 5, 2, 1}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i, e := range got {
		if e.Amount != want[i] {
			t.Errorf("row %d has amount %d, want %d (%+v)", i, e.Amount, want[i], got)
		}
	}
	if got[1].HoldingID == 0 || got[1].Description != "" || got[1].CategoryID != 0 {
		t.Errorf("Holding row not stored as Holding only: %+v", got[1])
	}
}

func TestRepeatingBlocksDeletingItsHolding(t *testing.T) {
	s, raw := open(t)
	saveRepeating(t, s, holdingRepeatingEntry(t, s, store.TypeInvestment, "ETF", 5))
	if _, err := raw.Exec(`DELETE FROM holdings`); err == nil {
		t.Error("deleting a Holding used by a Repeating row succeeded")
	}
	saveRepeating(t, s)
	if _, err := raw.Exec(`DELETE FROM holdings`); err != nil {
		t.Errorf("unused Holding not deletable: %v", err)
	}
}

func TestBookRepeatingBooksInvestmentAndDivestmentAgainstTheirHolding(t *testing.T) {
	s, _ := open(t)
	saveRepeating(t, s,
		holdingRepeatingEntry(t, s, store.TypeInvestment, "ETF", 300), holdingRepeatingEntry(t, s, store.TypeDivestment, "Bonds", 100))
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 2 || skipped != 0 {
		t.Fatalf("booked %d, skipped %d, err %v", booked, skipped, err)
	}
	rows, err := s.ListTransactions()
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %v, err %v", rows, err)
	}
	got := map[string]string{}
	for _, r := range rows {
		got[r.Holding] = r.Type
	}
	if got["ETF"] != store.TypeInvestment || got["Bonds"] != store.TypeDivestment {
		t.Errorf("booked %v", got)
	}
}

func TestBookRepeatingHoldingCollidesWithAnyTypeOnTheSameDate(t *testing.T) {
	s, _ := open(t)
	etf := holdingRepeatingEntry(t, s, store.TypeDivestment, "ETF", 40)
	if _, err := s.AddTransaction(store.NewTransaction{Date: "2026-10-01", Amount: 40, Type: store.TypeDivestment, HoldingID: etf.HoldingID}); err != nil {
		t.Fatal(err)
	}
	etf.Type = store.TypeInvestment
	saveRepeating(t, s, repeatingEntry(t, s, store.TypeIncome, "Salary", 5), etf)
	booked, skipped, err := s.BookRepeating("2026-10-01")
	if err != nil || booked != 1 || skipped != 1 {
		t.Fatalf("booked %d, skipped %d, err %v", booked, skipped, err)
	}
	if rows, _ := s.ListTransactions(); len(rows) != 2 {
		t.Errorf("rows %v, want the Divestment and Salary", rows)
	}
	if booked, _, err := s.BookRepeating("2026-11-01"); err != nil || booked != 2 {
		t.Fatalf("next month: booked %d err %v", booked, err)
	}
}
