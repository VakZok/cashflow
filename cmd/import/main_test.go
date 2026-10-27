package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/VakZok/Cashflow/internal/store"
)

func mock(t *testing.T, rows [][]any) string {
	path := filepath.Join(t.TempDir(), "finances.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE transactions (date TEXT, description TEXT, value INTEGER, main_category TEXT, subcategory TEXT, type TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO transactions VALUES (?,?,?,?,?,?)`, r...); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestImport(t *testing.T) {
	src := mock(t, [][]any{
		{"2024-03-01", "Gehalt", 3200, "Arbeit", "Lohn", "Einnahme"},
		{"2024-03-02", "Rewe", 45, "Lebensmittel", "Supermarkt", "Ausgabe"},
		{"2024-03-02", "Rückzahlung", 20, "Sonstiges", "Freunde", "Einnahme"},
		{"2024-03-02", "Rückzahlung", 20, "Sonstiges", "Freunde", "Ausgabe"},
	})
	rows, problems := load(src)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	out := filepath.Join(t.TempDir(), "new.db")
	if err := write(out, rows); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	income, expense, err := s.Totals("2024-03-01", "2024-03-31")
	if err != nil || income != 3220 || expense != 65 {
		t.Fatalf("totals %d %d %v", income, expense, err)
	}
}

func TestRejects(t *testing.T) {
	src := mock(t, [][]any{
		{"2024-03-02", "Rewe", 45, "Lebensmittel", "Supermarkt", "Ausgabe"},
		{"2024-03-02", "REWE", 10, "Lebensmittel", "Supermarkt", "Ausgabe"},
		{"2024-02-30", "Bad day", 1, "A", "B", "Ausgabe"},
		{"2024-03-03", "Typo", 1, "A", "B", "ausgabe"},
		{"2024-03-04", "Cents", 12.5, "A", "B", "Ausgabe"},
		{"2024-03-05", "", 1, "A", "B", "Ausgabe"},
	})
	_, problems := load(src)
	if len(problems) != 5 {
		t.Fatalf("got %d problems: %q", len(problems), problems)
	}
	for _, p := range problems {
		t.Log(p)
	}
}
