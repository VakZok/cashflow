// Command seed fills a Money Manager database with dummy data for trying out the Report.
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/VakZok/money_manager_app/internal/store"
)

type row struct {
	day         int // day of month
	amount      int64
	desc        string
	cat, subcat string
}

var income = []row{
	{1, 3200, "Salary", "Salary", "Employer"},
	{15, 250, "Freelance gig", "Side income", "Freelance"},
	{20, 12, "Bank interest", "Other income", "Interest"},
}

var expenses = []row{
	{2, 950, "Rent", "Housing", "Rent"},
	{5, 85, "Electricity", "Housing", "Utilities"},
	{7, 120, "Groceries", "Food", "Supermarket"},
	{14, 110, "Groceries", "Food", "Supermarket"},
	{21, 95, "Groceries", "Food", "Supermarket"},
	{12, 60, "Dinner out", "Food", "Restaurants"},
	{18, 45, "Pizza night", "Food", "Restaurants"},
	{10, 50, "Monthly pass", "Transport", "Public transit"},
	{22, 70, "Fuel", "Transport", "Car"},
	{16, 15, "Streaming", "Leisure", "Subscriptions"},
	{25, 40, "Concert", "Leisure", "Events"},
	{27, 30, "Pharmacy", "Health", "Medicine"},
	{28, 4, "Coffee", "Food", "Cafes"},
}

func main() {
	dbPath := flag.String("db", "money_manager.db", "path to the SQLite database file")
	months := flag.Int("months", 3, "number of months back (including the current one) to fill")
	flag.Parse()

	s, err := store.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	existing, err := s.ListTransactions()
	if err != nil {
		log.Fatal(err)
	}
	if len(existing) > 0 {
		log.Fatalf("%s already has %d transactions; refusing to seed", *dbPath, len(existing))
	}

	holding, err := s.EnsureHolding("World ETF")
	if err != nil {
		log.Fatal(err)
	}

	first := time.Now().AddDate(0, -(*months - 1), 0)
	n := 0
	add := func(t store.NewTransaction) {
		if _, err := s.AddTransaction(t); err != nil {
			log.Fatal(err)
		}
		n++
	}
	for m := 0; m < *months; m++ {
		month := time.Date(first.Year(), first.Month()+time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		date := func(day int) string { return month.AddDate(0, 0, day-1).Format("2006-01-02") }
		for typ, rows := range map[string][]row{store.TypeIncome: income, store.TypeExpense: expenses} {
			for _, r := range rows {
				cat, err := s.EnsureCategory(r.cat)
				if err != nil {
					log.Fatal(err)
				}
				sub, err := s.EnsureSubcategory(r.subcat)
				if err != nil {
					log.Fatal(err)
				}
				add(store.NewTransaction{Date: date(r.day), Amount: r.amount, Description: r.desc,
					Type: typ, CategoryID: cat, SubcategoryID: sub})
			}
		}
		// Must not show up in the pie charts.
		add(store.NewTransaction{Date: date(3), Amount: 400, Type: store.TypeInvestment, HoldingID: holding})
		add(store.NewTransaction{Date: date(24), Amount: 100, Type: store.TypeDivestment, HoldingID: holding})
	}
	fmt.Printf("seeded %d transactions into %s\n", n, *dbPath)
}
