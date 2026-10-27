// Command import converts an old finances.db into a new Cashflow database.
//
// It is a one-time migration the creator needed to bring their existing
// records over, not part of the app. The old file is only read. Every row is
// checked against the app's rules first; if any row is rejected, the problems
// are listed and nothing is written.
//
//	go run ./cmd/import -from finances.db -db new_finances.db
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VakZok/Cashflow/internal/store"
	_ "modernc.org/sqlite"
)

// types maps the old type column to the app's Types.
var types = map[string]string{"Einnahme": store.TypeIncome, "Ausgabe": store.TypeExpense}

type row struct {
	rowid                     int64
	date, desc, cat, sub, typ string
	amount                    int64
}

func main() {
	from := flag.String("from", "finances.db", "old database to read")
	to := flag.String("db", "new_finances.db", "new database to create; must not exist")
	flag.Parse()

	problems, err := run(*from, *to)
	for _, p := range problems {
		fmt.Println(p)
	}
	if err != nil {
		log.Fatal(err)
	}
	if len(problems) > 0 {
		log.Fatalf("%d problem(s); nothing written", len(problems))
	}
	fmt.Printf("imported into %s\n", *to)
}

// run converts the old database at from into a new database at to. If any
// row breaks a rule, it returns one problem per broken rule and writes
// nothing.
func run(from, to string) (problems []string, err error) {
	if _, err := os.Stat(to); err == nil {
		return nil, fmt.Errorf("%s already exists; refusing to overwrite", to)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	rows, problems, err := load(from)
	if err != nil || len(problems) > 0 {
		return problems, err
	}

	// Build next to the target and rename at the end, so a failure halfway
	// never leaves a partial database at to.
	tmp := to + ".tmp"
	_ = os.Remove(tmp)
	if err := write(tmp, rows); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	return nil, os.Rename(tmp, to)
}

func load(path string) (out []row, problems []string, err error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = db.Close() }()

	rs, err := db.Query(`SELECT rowid, date, description, value, main_category, subcategory, type
		FROM transactions ORDER BY rowid`)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rs.Close() }()

	seen := map[string]int64{}
	for rs.Next() {
		var id int64
		var date, desc, cat, sub, typ sql.NullString
		var value any
		if err := rs.Scan(&id, &date, &desc, &value, &cat, &sub, &typ); err != nil {
			return nil, nil, err
		}
		r := row{rowid: id, date: strings.TrimSpace(date.String), desc: strings.TrimSpace(desc.String),
			cat: strings.TrimSpace(cat.String), sub: strings.TrimSpace(sub.String)}
		bad := func(format string, a ...any) {
			problems = append(problems, fmt.Sprintf("row %d (%s %q): %s", id, r.date, r.desc, fmt.Sprintf(format, a...)))
		}

		if d, err := time.Parse("2006-01-02", r.date); err != nil || d.Format("2006-01-02") != r.date {
			bad("date %q is not a real YYYY-MM-DD day", date.String)
		}
		if n := utf8.RuneCountInString(r.desc); n < 1 || n > 100 {
			bad("description must be 1..100 characters, has %d", n)
		}
		if n := utf8.RuneCountInString(r.cat); n < 1 || n > 50 {
			bad("main_category must be 1..50 characters, has %d", n)
		}
		if n := utf8.RuneCountInString(r.sub); n < 1 || n > 50 {
			bad("subcategory must be 1..50 characters, has %d", n)
		}
		if v, ok := value.(int64); !ok || v < 1 || v > 100_000_000 {
			bad("value %v must be a whole number 1..100000000", value)
		} else {
			r.amount = v
		}
		if t, ok := types[typ.String]; !ok {
			bad("type %q must be Einnahme or Ausgabe", typ.String)
		} else {
			r.typ = t
		}

		// The app's Duplicate rule: date, description ignoring ASCII case, Type (ADR 0006).
		if r.typ != "" {
			key := r.date + "\x00" + asciiLower(r.desc) + "\x00" + r.typ
			if first, ok := seen[key]; ok {
				bad("duplicate of row %d", first)
			} else {
				seen[key] = id
			}
		}
		out = append(out, r)
	}
	return out, problems, rs.Err()
}

func write(path string, rows []row) error {
	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()
	for _, r := range rows {
		cat, err := s.EnsureCategory(r.cat)
		if err != nil {
			return fmt.Errorf("row %d: %w", r.rowid, err)
		}
		sub, err := s.EnsureSubcategory(r.sub)
		if err != nil {
			return fmt.Errorf("row %d: %w", r.rowid, err)
		}
		if _, err := s.AddTransaction(store.NewTransaction{Date: r.date, Amount: r.amount, Description: r.desc,
			Type: r.typ, CategoryID: cat, SubcategoryID: sub}); err != nil {
			return fmt.Errorf("row %d: %w", r.rowid, err)
		}
	}
	return nil
}

// asciiLower lowercases A-Z only, matching SQLite's NOCASE.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
