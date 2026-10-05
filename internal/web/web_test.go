package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// expenseForm builds the entry form submission. category/subcategory are
// either an existing id or "new" together with the new name.
func expenseForm(date, amount, desc, category, newCategory, subcategory, newSubcategory string) url.Values {
	return url.Values{
		"date":            {date},
		"amount":          {amount},
		"description":     {desc},
		"category":        {category},
		"new_category":    {newCategory},
		"subcategory":     {subcategory},
		"new_subcategory": {newSubcategory},
	}
}

func TestAppServesPagesAndSurvivesRestart(t *testing.T) {
	ta := newTestApp(t)
	if r := ta.get("/"); r.Status != 200 {
		t.Fatalf("GET / = %d, want 200", r.Status)
	}
	// Restarting on the same database re-runs migrations without error.
	ta.restart()
	if r := ta.get("/"); r.Status != 200 {
		t.Fatalf("GET / after restart = %d, want 200", r.Status)
	}
}

func TestAddExpenseShowsInList(t *testing.T) {
	ta := newTestApp(t)

	r := ta.post("/transactions", expenseForm("2026-10-01", "1234", "Rent", "new", "Housing", "new", "Monthly"))
	if r.Status != http.StatusSeeOther || r.Location != "/" {
		t.Fatalf("POST = %d -> %q, want 303 -> /", r.Status, r.Location)
	}
	body := ta.follow(r).Body
	for _, want := range []string{"2026-10-01", "€1.234", "Rent", "Housing", "Monthly"} {
		if !strings.Contains(body, want) {
			t.Errorf("list missing %q in:\n%s", want, body)
		}
	}
}
