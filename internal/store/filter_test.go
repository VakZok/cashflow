package store_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/VakZok/money_manager_app/internal/store"
)

// descriptions are the rows' descriptions, in order.
func descriptions(rows []store.TransactionRow) (out []string) {
	for _, r := range rows {
		out = append(out, r.Description)
	}
	return out
}

func TestAFilterNarrowsListAndCountToItsDateRange(t *testing.T) {
	s, _ := open(t)
	for _, date := range []string{"2025-12-31", "2026-01-01", "2026-02-28", "2026-03-01", "2026-12-31", "2027-01-01"} {
		if _, err := s.AddTransaction(expense(t, s, date, date)); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name string
		f    store.Filter
		want string
	}{
		{"none", store.Filter{}, "[2027-01-01 2026-12-31 2026-03-01 2026-02-28 2026-01-01 2025-12-31]"},
		{"year", store.Filter{From: "2026-01-01", To: "2027-01-01"}, "[2026-12-31 2026-03-01 2026-02-28 2026-01-01]"},
		{"month", store.Filter{From: "2026-02-01", To: "2026-03-01"}, "[2026-02-28]"},
	} {
		rows, err := s.ListTransactionsPage(c.f, -1, 0)
		if err != nil {
			t.Fatal(err)
		}
		if g := fmt.Sprint(descriptions(rows)); g != c.want {
			t.Errorf("%s: list = %s, want %s", c.name, g, c.want)
		}
		if n, err := s.CountTransactions(c.f); err != nil || n != len(rows) {
			t.Errorf("%s: count = %d, err %v; want %d", c.name, n, err, len(rows))
		}
	}
}

func TestTransactionPageCountsOnlyTransactionsMatchingTheFilter(t *testing.T) {
	s, _ := open(t)
	var ids []int64
	// Six in 2027 push the 2026 ones to page 2 unfiltered.
	for i, date := range []string{"2026-05-01", "2026-05-02", "2027-01-01", "2027-01-02", "2027-01-03", "2027-01-04", "2027-01-05", "2027-01-06"} {
		id, err := s.AddTransaction(expense(t, s, date, fmt.Sprintf("T%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	in2026 := store.Filter{From: "2026-01-01", To: "2027-01-01"}
	if p, err := s.TransactionPage(ids[0], store.Filter{}); err != nil || p != 2 {
		t.Errorf("unfiltered: page %d, err %v; want 2", p, err)
	}
	if p, err := s.TransactionPage(ids[0], in2026); err != nil || p != 1 {
		t.Errorf("2026: page %d, err %v; want 1", p, err)
	}
	if _, err := s.TransactionPage(ids[2], in2026); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("outside the filter: err %v, want ErrNotFound", err)
	}
}

func TestTransactionYearsAreTheYearsWithTransactionsNewestFirst(t *testing.T) {
	s, _ := open(t)
	if ys, err := s.TransactionYears(); err != nil || len(ys) != 0 {
		t.Fatalf("empty: years %v, err %v", ys, err)
	}
	for _, date := range []string{"2024-03-01", "2026-01-01", "2026-12-31", "2024-07-07"} {
		if _, err := s.AddTransaction(expense(t, s, date, date)); err != nil {
			t.Fatal(err)
		}
	}
	ys, err := s.TransactionYears()
	if err != nil || fmt.Sprint(ys) != "[2026 2024]" {
		t.Errorf("years %v, err %v; want [2026 2024]", ys, err)
	}
}

// typed records a Transaction of the Type described by its own Type and fields.
func typed(t *testing.T, s *store.Store, day int, n store.NewTransaction) {
	t.Helper()
	n.Date, n.Amount = fmt.Sprintf("2026-01-%02d", day), 5
	if _, err := s.AddTransaction(n); err != nil {
		t.Fatal(err)
	}
}

func TestAFilterNarrowsToATypeAndItsGroupings(t *testing.T) {
	s, _ := open(t)
	must := func(id int64, err error) int64 {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	food, rent := must(s.EnsureCategory("Food")), must(s.EnsureCategory("Rent"))
	cafe, flat := must(s.EnsureSubcategory("Cafe")), must(s.EnsureSubcategory("Flat"))
	tax, fee := must(s.EnsureLevyCategory("Tax")), must(s.EnsureLevyCategory("Fee"))
	federal := must(s.EnsureLevySubcategory("Federal"))
	etf := must(s.EnsureHolding("ETF"))
	typed(t, s, 1, store.NewTransaction{Type: store.TypeExpense, Description: "E food cafe", CategoryID: food, SubcategoryID: cafe})
	typed(t, s, 2, store.NewTransaction{Type: store.TypeExpense, Description: "E food flat", CategoryID: food, SubcategoryID: flat})
	typed(t, s, 3, store.NewTransaction{Type: store.TypeExpense, Description: "E rent flat", CategoryID: rent, SubcategoryID: flat})
	typed(t, s, 4, store.NewTransaction{Type: store.TypeIncome, Description: "I food cafe", CategoryID: food, SubcategoryID: cafe})
	typed(t, s, 5, store.NewTransaction{Type: store.TypeLevy, Description: "L tax", LevyCategoryID: tax, LevySubcategoryID: federal})
	typed(t, s, 6, store.NewTransaction{Type: store.TypeRebate, Description: "R fee", LevyCategoryID: fee, LevySubcategoryID: federal})
	typed(t, s, 7, store.NewTransaction{Type: store.TypeInvestment, HoldingID: etf})
	typed(t, s, 8, store.NewTransaction{Type: store.TypeDivestment, HoldingID: etf})

	for _, c := range []struct {
		name string
		f    store.Filter
		want string
	}{
		{"all", store.Filter{}, "8"},
		{"expense", store.Filter{Type: store.TypeExpense}, "3"},
		{"income", store.Filter{Type: store.TypeIncome}, "1"},
		{"levy", store.Filter{Type: store.TypeLevy}, "1"},
		{"rebate", store.Filter{Type: store.TypeRebate}, "1"},
		{"investment", store.Filter{Type: store.TypeInvestment}, "1"},
		{"divestment", store.Filter{Type: store.TypeDivestment}, "1"},
		{"category", store.Filter{Type: store.TypeExpense, CategoryID: food}, "2"},
		{"subcategory", store.Filter{Type: store.TypeExpense, SubcategoryID: flat}, "2"},
		{"both", store.Filter{Type: store.TypeExpense, CategoryID: food, SubcategoryID: flat}, "1"},
		{"unused combination", store.Filter{Type: store.TypeExpense, CategoryID: rent, SubcategoryID: cafe}, "0"},
		{"levy category", store.Filter{Type: store.TypeLevy, CategoryID: tax}, "1"},
		{"levy category of the other Type", store.Filter{Type: store.TypeLevy, CategoryID: fee}, "0"},
		{"levy subcategory shared by both Types", store.Filter{Type: store.TypeRebate, SubcategoryID: federal}, "1"},
		{"groupings are ignored for a Type without them", store.Filter{Type: store.TypeInvestment, CategoryID: food}, "1"},
		{"groupings are ignored without a Type", store.Filter{CategoryID: food}, "8"},
	} {
		rows, err := s.ListTransactionsPage(c.f, -1, 0)
		if err != nil {
			t.Fatal(err)
		}
		if g := fmt.Sprint(len(rows)); g != c.want {
			t.Errorf("%s: list has %s, want %s", c.name, g, c.want)
		}
		if n, err := s.CountTransactions(c.f); err != nil || fmt.Sprint(n) != c.want {
			t.Errorf("%s: count = %d, err %v; want %s", c.name, n, err, c.want)
		}
	}
}

func TestTransactionPageHonoursTheTypeFilter(t *testing.T) {
	s, _ := open(t)
	var income int64
	for i := range 7 {
		n := expense(t, s, fmt.Sprintf("2026-05-0%d", i+1), fmt.Sprintf("T%d", i))
		if i == 0 {
			n.Type = store.TypeIncome
		}
		id, err := s.AddTransaction(n)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			income = id
		}
	}
	if p, err := s.TransactionPage(income, store.Filter{}); err != nil || p != 2 {
		t.Errorf("unfiltered: page %d, err %v; want 2", p, err)
	}
	if p, err := s.TransactionPage(income, store.Filter{Type: store.TypeIncome}); err != nil || p != 1 {
		t.Errorf("income: page %d, err %v; want 1", p, err)
	}
	if _, err := s.TransactionPage(income, store.Filter{Type: store.TypeExpense}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("other Type: err %v, want ErrNotFound", err)
	}
}

func TestAHoldingFilterNarrowsInvestmentsAndDivestmentsOfThatHolding(t *testing.T) {
	s, _ := open(t)
	etf, _ := s.EnsureHolding("ETF")
	gold, _ := s.EnsureHolding("Gold")
	for i, c := range []struct {
		typ     string
		holding int64
	}{
		{store.TypeInvestment, etf}, {store.TypeDivestment, etf}, {store.TypeInvestment, gold},
	} {
		n := store.NewTransaction{Date: fmt.Sprintf("2026-01-0%d", i+1), Amount: 3, Type: c.typ, HoldingID: c.holding}
		if _, err := s.AddTransaction(n); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddTransaction(expense(t, s, "2026-01-09", "Lunch")); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		f    store.Filter
		want int
	}{
		{"Type only", store.Filter{Type: store.TypeInvestment}, 2},
		{"Investments of a Holding", store.Filter{Type: store.TypeInvestment, HoldingID: etf}, 1},
		{"Divestments of a Holding", store.Filter{Type: store.TypeDivestment, HoldingID: etf}, 1},
		{"Divestments of another Holding", store.Filter{Type: store.TypeDivestment, HoldingID: gold}, 0},
		{"Holding is ignored for a Type without one", store.Filter{Type: store.TypeExpense, HoldingID: etf}, 1},
		{"Holding is ignored without a Type", store.Filter{HoldingID: etf}, 4},
		{"Holding combines with the date range", store.Filter{Type: store.TypeInvestment, HoldingID: etf, From: "2026-01-02", To: "2026-02-01"}, 0},
	} {
		rows, err := s.ListTransactionsPage(c.f, -1, 0)
		if err != nil || len(rows) != c.want {
			t.Errorf("%s: list has %d, err %v; want %d", c.name, len(rows), err, c.want)
		}
		if n, err := s.CountTransactions(c.f); err != nil || n != c.want {
			t.Errorf("%s: count = %d, err %v; want %d", c.name, n, err, c.want)
		}
	}
}

func TestASearchMatchesDescriptionOrHoldingAsALiteralASCIICaseInsensitivePhrase(t *testing.T) {
	s, _ := open(t)
	etf, err := s.EnsureHolding("ETF World")
	if err != nil {
		t.Fatal(err)
	}
	typed(t, s, 1, store.NewTransaction{Type: store.TypeInvestment, HoldingID: etf})
	for i, d := range []string{"Etf fees", "Food rest", "100% sure", "a_b", "axb", "Ärger", "Rent"} {
		typed(t, s, i+2, expense(t, s, "", d))
	}
	for _, c := range []struct {
		search string
		want   int
	}{
		{"etf", 2}, // the Investment through its Holding, the Expense through its description
		{"  ETF  ", 2},
		{"food rest", 1},
		{"food  rest", 0},
		{"%", 1},
		{"_", 1},
		{"ä", 0}, // case is ignored for ASCII only
		{"Ä", 1},
		{"", 8},
		{"   ", 8},
	} {
		f := store.Filter{Search: c.search}
		rows, err := s.ListTransactionsPage(f, -1, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != c.want {
			t.Errorf("%q: list = %v, want %d rows", c.search, descriptions(rows), c.want)
		}
		if n, err := s.CountTransactions(f); err != nil || n != c.want {
			t.Errorf("%q: count = %d, err %v; want %d", c.search, n, err, c.want)
		}
	}
}
