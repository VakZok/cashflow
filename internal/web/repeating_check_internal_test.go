package web

import (
	"net/url"
	"path/filepath"
	"testing"

	"github.com/VakZok/Cashflow/internal/store"
)

func checkApp(t *testing.T) *App {
	t.Helper()
	a, err := New(Config{DBPath: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// checkList runs the read and check steps on form values.
func checkList(t *testing.T, v url.Values) ([]repeatingRow, checkedList) {
	t.Helper()
	a := checkApp(t)
	rows, _ := a.readRepeating(v)
	c, err := a.checkRepeating(rows)
	if err != nil {
		t.Fatal(err)
	}
	return rows, c
}

func listValues(rows ...map[string]string) url.Values {
	v := url.Values{}
	for i, r := range rows {
		n := string(rune('0' + i))
		v.Add("row", n)
		for k, val := range r {
			v.Set(k+"_"+n, val)
		}
	}
	return v
}

func rent(extra map[string]string) map[string]string {
	r := map[string]string{"type": "expense", "amount": "5", "description": "Rent", "category": "new", "new_category": "Home", "subcategory": "new", "new_subcategory": "Flat"}
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func TestCheckRepeatingCleanListHasNoProblems(t *testing.T) {
	_, c := checkList(t, listValues(rent(nil)))
	if c.problems.failed() || len(c.entries) != 1 {
		t.Fatalf("got %+v", c)
	}
}

func TestCheckRepeatingMissingField(t *testing.T) {
	rows, c := checkList(t, listValues(rent(map[string]string{"description": ""})))
	if !c.problems.missing || c.problems.button().Label != "Missing fields" || !rows[0].Bad("description") {
		t.Fatalf("got %+v, row %+v", c.problems, rows[0])
	}
}

func TestCheckRepeatingBadAmount(t *testing.T) {
	rows, c := checkList(t, listValues(rent(map[string]string{"amount": "abc"})))
	if !c.problems.amount || !rows[0].AmountBad || c.problems.button().Label != "" {
		t.Fatalf("got %+v, row %+v", c.problems, rows[0])
	}
}

func TestCheckRepeatingDuplicateRowIsMarked(t *testing.T) {
	rows, c := checkList(t, listValues(rent(nil), rent(nil)))
	if !c.problems.duplicate || rows[0].Duplicate || !rows[1].Duplicate || c.problems.button().Label != "Duplicate rows" {
		t.Fatalf("got %+v, rows %+v", c.problems, rows)
	}
}

func TestCheckRepeatingSavesAndCreatesNothing(t *testing.T) {
	a := checkApp(t)
	rows, _ := a.readRepeating(listValues(rent(nil)))
	if _, err := a.checkRepeating(rows); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.store.ListRepeating(); len(got) != 0 {
		t.Fatalf("saved %v", got)
	}
	if got, _ := a.store.ListNamed(store.DefaultListSet.Category); len(got) != 0 {
		t.Fatalf("created %v", got)
	}
}
