package main

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VakZok/Cashflow/internal/store"
)

// old builds an old-format finances.db holding rows of
// (date, description, value, main_category, subcategory, type).
func old(t *testing.T, rows ...[]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "finances.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE transactions (date TEXT, description TEXT, value INTEGER,
		main_category TEXT, subcategory TEXT, type TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO transactions VALUES (?, ?, ?, ?, ?, ?)`, r...); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func newPath(t *testing.T) string { return filepath.Join(t.TempDir(), "new_finances.db") }

func openNew(t *testing.T, path string) *store.Store {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestEinnahmeAndAusgabeBecomeIncomeAndExpense(t *testing.T) {
	from := old(t,
		[]any{"2024-03-01", "Gehalt", 3200, "Arbeit", "Lohn", "Einnahme"},
		[]any{"2024-03-02", "Rewe", 45, "Lebensmittel", "Supermarkt", "Ausgabe"},
	)
	to := newPath(t)

	problems, err := run(from, to)
	if err != nil || len(problems) > 0 {
		t.Fatalf("run: %v %q", err, problems)
	}

	rows, err := openNew(t, to).ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d transactions, want 2", len(rows))
	}
	// Newest first.
	want := []struct {
		date, desc, typ, cat, sub string
		amount                    int64
	}{
		{"2024-03-02", "Rewe", store.TypeExpense, "Lebensmittel", "Supermarkt", 45},
		{"2024-03-01", "Gehalt", store.TypeIncome, "Arbeit", "Lohn", 3200},
	}
	for i, w := range want {
		r := rows[i]
		if r.Date != w.date || r.Description != w.desc || r.Type != w.typ ||
			r.Category != w.cat || r.Subcategory != w.sub || r.Amount != w.amount {
			t.Errorf("row %d = %+v, want %+v", i, r, w)
		}
	}
}

func TestIncomeAndExpenseShareCategories(t *testing.T) {
	from := old(t,
		[]any{"2024-03-01", "Flohmarkt", 30, "Sonstiges", "Diverses", "Einnahme"},
		[]any{"2024-03-02", "Geschenk", 25, "Sonstiges", "Diverses", "Ausgabe"},
	)
	to := newPath(t)
	if problems, err := run(from, to); err != nil || len(problems) > 0 {
		t.Fatalf("run: %v %q", err, problems)
	}

	s := openNew(t, to)
	for _, l := range []store.List{store.CategoryList, store.SubcategoryList} {
		named, err := s.ListNamed(l)
		if err != nil {
			t.Fatal(err)
		}
		if len(named) != 1 {
			t.Errorf("list %v has %d entries, want 1: %+v", l, len(named), named)
		}
	}
}

func TestRejectedRowsAreListedAndNothingIsWritten(t *testing.T) {
	long := func(n int) string { return strings.Repeat("ä", n) }
	tests := []struct {
		name string
		row  []any
		want string
	}{
		{"impossible date", []any{"2024-02-30", "Miete", 900, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (2024-02-30 "Miete"): date "2024-02-30" is not a real YYYY-MM-DD day`},
		{"other date format", []any{"05.03.2024", "Miete", 900, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (05.03.2024 "Miete"): date "05.03.2024" is not a real YYYY-MM-DD day`},
		{"lowercase type", []any{"2024-03-05", "Miete", 900, "Wohnen", "Miete", "ausgabe"},
			`row 2 (2024-03-05 "Miete"): type "ausgabe" must be Einnahme or Ausgabe`},
		{"decimal value", []any{"2024-03-05", "Miete", 12.5, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (2024-03-05 "Miete"): value 12.5 must be a whole number 1..100000000`},
		{"zero value", []any{"2024-03-05", "Miete", 0, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (2024-03-05 "Miete"): value 0 must be a whole number 1..100000000`},
		{"value too large", []any{"2024-03-05", "Miete", 100_000_001, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (2024-03-05 "Miete"): value 100000001 must be a whole number 1..100000000`},
		{"blank description", []any{"2024-03-05", "  ", 900, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (2024-03-05 ""): description must be 1..100 characters, has 0`},
		{"missing description", []any{"2024-03-05", nil, 900, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (2024-03-05 ""): description must be 1..100 characters, has 0`},
		{"description too long", []any{"2024-03-05", long(101), 900, "Wohnen", "Miete", "Ausgabe"},
			`row 2 (2024-03-05 "` + long(101) + `"): description must be 1..100 characters, has 101`},
		{"main_category too long", []any{"2024-03-05", "Miete", 900, long(51), "Miete", "Ausgabe"},
			`row 2 (2024-03-05 "Miete"): main_category must be 1..50 characters, has 51`},
		{"empty subcategory", []any{"2024-03-05", "Miete", 900, "Wohnen", "", "Ausgabe"},
			`row 2 (2024-03-05 "Miete"): subcategory must be 1..50 characters, has 0`},
		{"duplicate ignoring case", []any{"2024-03-01", "GEHALT", 100, "Arbeit", "Bonus", "Einnahme"},
			`row 2 (2024-03-01 "GEHALT"): duplicate of row 1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from := old(t, []any{"2024-03-01", "Gehalt", 3200, "Arbeit", "Lohn", "Einnahme"}, tt.row)
			to := newPath(t)

			problems, err := run(from, to)
			if err != nil {
				t.Fatal(err)
			}
			if len(problems) != 1 || problems[0] != tt.want {
				t.Errorf("problems = %q, want [%q]", problems, tt.want)
			}
			if _, err := os.Stat(to); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("%s was written: %v", to, err)
			}
		})
	}
}

func TestSameDescriptionUnderBothTypesIsNotADuplicate(t *testing.T) {
	from := old(t,
		[]any{"2024-03-02", "Rückzahlung", 20, "Sonstiges", "Freunde", "Einnahme"},
		[]any{"2024-03-02", "Rückzahlung", 20, "Sonstiges", "Freunde", "Ausgabe"},
	)
	if problems, err := run(from, newPath(t)); err != nil || len(problems) > 0 {
		t.Fatalf("run: %v %q", err, problems)
	}
}

func TestEveryProblemIsListedInOneRun(t *testing.T) {
	from := old(t,
		[]any{"2024-02-30", "", 0, "Wohnen", "Miete", "Ausgabe"},
		[]any{"2024-03-01", "Gehalt", 3200, "Arbeit", "Lohn", "Gehalt"},
	)
	problems, err := run(from, newPath(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`row 1 (2024-02-30 ""): date "2024-02-30" is not a real YYYY-MM-DD day`,
		`row 1 (2024-02-30 ""): description must be 1..100 characters, has 0`,
		`row 1 (2024-02-30 ""): value 0 must be a whole number 1..100000000`,
		`row 2 (2024-03-01 "Gehalt"): type "Gehalt" must be Einnahme or Ausgabe`,
	}
	if strings.Join(problems, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems =\n%s\nwant\n%s", strings.Join(problems, "\n"), strings.Join(want, "\n"))
	}
}

// The driver turns values of a column declared DATE into time.Time; the
// import must still see the stored YYYY-MM-DD text.
func TestDateColumnDeclaredAsDate(t *testing.T) {
	from := filepath.Join(t.TempDir(), "finances.db")
	db, err := sql.Open("sqlite", from)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE transactions (date DATE, description TEXT, value INTEGER,
		main_category TEXT, subcategory TEXT, type TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO transactions VALUES ('2024-03-01', 'Gehalt', 3200, 'Arbeit', 'Lohn', 'Einnahme')`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	to := newPath(t)

	if problems, err := run(from, to); err != nil || len(problems) > 0 {
		t.Fatalf("run: %v %q", err, problems)
	}
	rows, err := openNew(t, to).ListTransactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Date != "2024-03-01" {
		t.Errorf("rows = %+v, want one dated 2024-03-01", rows)
	}
}

func TestExistingTargetIsRefused(t *testing.T) {
	from := old(t, []any{"2024-03-01", "Gehalt", 3200, "Arbeit", "Lohn", "Einnahme"})
	to := newPath(t)
	if err := os.WriteFile(to, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := run(from, to); err == nil {
		t.Fatal("run succeeded, want an error")
	}
	if got, _ := os.ReadFile(to); string(got) != "keep me" {
		t.Errorf("target was changed to %q", got)
	}
}

func TestOldFileIsLeftUnchanged(t *testing.T) {
	from := old(t, []any{"2024-03-01", "Gehalt", 3200, "Arbeit", "Lohn", "Einnahme"})
	before, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if problems, err := run(from, newPath(t)); err != nil || len(problems) > 0 {
		t.Fatalf("run: %v %q", err, problems)
	}
	after, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("old file was modified")
	}
}

func TestMissingOldFileIsAnError(t *testing.T) {
	to := newPath(t)
	if _, err := run(filepath.Join(t.TempDir(), "nope.db"), to); err == nil {
		t.Fatal("run succeeded, want an error")
	}
	if _, err := os.Stat(to); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s was written: %v", to, err)
	}
}
