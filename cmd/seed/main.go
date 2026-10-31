// Command seed fills an empty Cashflow database with demo data ready for
// screenshots: one person paid in euros in Germany, over whole months ending
// with the current one.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/VakZok/Cashflow/internal/store"
)

// row is a Transaction to book on a day of a month. cat and sub name a
// Category and Subcategory for Income and Expense, a Levy Category and Levy
// Subcategory for Levy and Rebate; holding names the Holding of an Investment
// or Divestment.
type row struct {
	typ      string
	day      int   // day of the month, at most 28 so every month has it
	amount   int64 // whole euros
	step     int64 // how much the amount varies from month to month; 0 keeps it fixed
	desc     string
	cat, sub string
	holding  string
}

func income(day int, amount, step int64, desc, cat, sub string) row {
	return row{typ: store.TypeIncome, day: day, amount: amount, step: step, desc: desc, cat: cat, sub: sub}
}

func expense(day int, amount, step int64, desc, cat, sub string) row {
	return row{typ: store.TypeExpense, day: day, amount: amount, step: step, desc: desc, cat: cat, sub: sub}
}

func levy(day int, amount int64, desc, cat, sub string) row {
	return row{typ: store.TypeLevy, day: day, amount: amount, desc: desc, cat: cat, sub: sub}
}

func rebate(day int, amount int64, desc, cat, sub string) row {
	return row{typ: store.TypeRebate, day: day, amount: amount, desc: desc, cat: cat, sub: sub}
}

func invest(typ string, day int, amount int64, holding string) row {
	return row{typ: typ, day: day, amount: amount, holding: holding}
}

// Booked every month.
var (
	salary   = income(27, 3050, 0, "Salary", "Work", "Salary")
	rent     = expense(1, 980, 0, "Rent", "Housing", "Rent")
	netflix  = expense(15, 14, 0, "Netflix", "Leisure", "Subscriptions")
	worldETF = invest(store.TypeInvestment, 3, 400, "World ETF")
	wageTax  = levy(27, 560, "Wage tax", "Income tax", "Payroll")
)

var monthly = []row{
	salary,
	income(28, 6, 2, "Savings interest", "Other", "Interest"),

	rent,
	expense(3, 68, 6, "Electricity", "Housing", "Utilities"),
	expense(5, 40, 0, "Internet", "Housing", "Utilities"),
	expense(2, 58, 0, "Deutschlandticket", "Transport", "Public transport"),
	expense(4, 64, 9, "Groceries", "Food", "Supermarket"),
	expense(11, 72, 8, "Groceries", "Food", "Supermarket"),
	expense(18, 58, 11, "Groceries", "Food", "Supermarket"),
	expense(25, 81, 7, "Groceries", "Food", "Supermarket"),
	expense(9, 38, 12, "Dinner with friends", "Food", "Restaurants"),
	expense(22, 14, 3, "Lunch out", "Food", "Restaurants"),
	expense(13, 9, 2, "Coffee", "Food", "Cafés"),
	netflix,
	expense(15, 11, 0, "Spotify", "Leisure", "Subscriptions"),
	expense(6, 30, 0, "Gym", "Health", "Sports"),
	expense(19, 12, 6, "Pharmacy", "Health", "Pharmacy"),
	expense(20, 15, 10, "Bike repair", "Transport", "Bike"),

	wageTax,
	levy(27, 440, "Pension insurance", "Social security", "Payroll"),
	levy(27, 395, "Health insurance", "Social security", "Payroll"),
	levy(27, 62, "Unemployment insurance", "Social security", "Payroll"),
	levy(27, 82, "Long-term care insurance", "Social security", "Payroll"),

	worldETF,
	invest(store.TypeInvestment, 3, 100, "Emerging Markets ETF"),
	invest(store.TypeInvestment, 16, 50, "Gold ETC"),
}

// repeating is the saved Repeating list.
var repeating = []row{salary, rent, netflix, worldETF, wageTax}

// seasonal returns the one-offs of a month.
func seasonal(year int, month time.Month) []row {
	switch month {
	case time.March:
		return []row{expense(8, 120, 0, "Spring jacket", "Shopping", "Clothing")}
	case time.June:
		// A Divestment with its profit booked as Income (ADR 0002) and the
		// Levies taken off that profit before it arrived.
		return []row{
			invest(store.TypeDivestment, 20, 600, "World ETF"),
			income(20, 140, 0, "World ETF profit", "Investments", "Capital gains"),
			levy(20, 45, "Capital gains tax", "Income tax", "Capital gains"),
			levy(20, 2, "Solidarity surcharge", "Solidarity surcharge", "Capital gains"),
		}
	case time.July:
		// A Rebate and the Income that carries its money.
		desc := fmt.Sprintf("Tax refund %d", year-1)
		return []row{
			rebate(14, 640, desc, "Income tax", "Tax return"),
			income(14, 640, 0, desc, "Other", "Tax refund"),
		}
	case time.August:
		return []row{
			expense(1, 320, 0, "Flights to Lisbon", "Travel", "Holiday"),
			expense(10, 1150, 0, "Summer holiday", "Travel", "Holiday"),
		}
	case time.November:
		return []row{expense(24, 180, 0, "Winter boots", "Shopping", "Clothing")}
	case time.December:
		return []row{
			income(27, 1200, 0, "Christmas bonus", "Work", "Bonus"),
			levy(27, 520, "Wage tax on bonus", "Income tax", "Payroll"),
			expense(12, 340, 0, "Christmas gifts", "Gifts", "Christmas"),
			expense(23, 95, 0, "Christmas dinner", "Food", "Restaurants"),
		}
	}
	return nil
}

func main() {
	dbPath := flag.String("db", "test.db", "path to the SQLite database file")
	months := flag.Int("months", 12, "number of whole months, ending with the current one, to fill")
	flag.Parse()

	n, err := run(*dbPath, *months, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("seeded %d transactions into %s\n", n, *dbPath)
}

// run seeds the database at dbPath with months whole months ending with the
// month of today, days after today included, and saves the Repeating list. It
// refuses a database that already has Transactions. The data depends only on
// the month, so seeding twice on the same day gives the same result.
func run(dbPath string, months int, today time.Time) (int, error) {
	if months < 1 {
		return 0, errors.New("-months must be at least 1")
	}
	s, err := store.Open(dbPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = s.Close() }()

	existing, err := s.CountTransactions(store.Filter{})
	if err != nil {
		return 0, err
	}
	if existing > 0 {
		return 0, fmt.Errorf("%s already has %d transactions; refusing to seed", dbPath, existing)
	}

	n := 0
	for m := months - 1; m >= 0; m-- {
		month := time.Date(today.Year(), today.Month()-time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		k := int64(month.Year()*12 + int(month.Month()))
		rows := append(append([]row(nil), monthly...), seasonal(month.Year(), month.Month())...)
		for _, e := range rows {
			t, err := resolve(s, e)
			if err != nil {
				return n, err
			}
			t.Date = month.AddDate(0, 0, e.day-1).Format("2006-01-02")
			// Add 0 to 4 steps so months differ; k*7 walks through all
			// remainders of 5, and the day shifts the pattern per row.
			t.Amount = e.amount + e.step*((k*7+int64(e.day))%5)
			if _, err := s.AddTransaction(t); err != nil {
				return n, fmt.Errorf("%s %s %q: %w", t.Date, e.typ, e.desc+e.holding, err)
			}
			n++
		}
	}

	var list []store.RepeatingEntry
	for _, e := range repeating {
		t, err := resolve(s, e)
		if err != nil {
			return n, err
		}
		list = append(list, store.RepeatingEntry{Type: t.Type, Amount: e.amount, Description: t.Description,
			CategoryID: t.CategoryID, SubcategoryID: t.SubcategoryID, HoldingID: t.HoldingID,
			LevyCategoryID: t.LevyCategoryID, LevySubcategoryID: t.LevySubcategoryID})
	}
	return n, s.ReplaceRepeating(list)
}

// resolve turns e into a Transaction without date and amount, creating the
// list entries it names.
func resolve(s *store.Store, e row) (store.NewTransaction, error) {
	t := store.NewTransaction{Type: e.typ, Description: e.desc}
	spec, ok := store.SpecFor(e.typ)
	if !ok {
		return t, fmt.Errorf("unknown type %q", e.typ)
	}
	var err error
	switch {
	case spec.Holding:
		t.HoldingID, err = s.EnsureHolding(e.holding)
	case spec.TakesLevyLists():
		if t.LevyCategoryID, err = s.EnsureLevyCategory(e.cat); err == nil {
			t.LevySubcategoryID, err = s.EnsureLevySubcategory(e.sub)
		}
	default:
		if t.CategoryID, err = s.EnsureCategory(e.cat); err == nil {
			t.SubcategoryID, err = s.EnsureSubcategory(e.sub)
		}
	}
	return t, err
}
