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

func TestFormDefaultsDateToToday(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/").Body
	if !strings.Contains(body, `name="date" value="2026-10-05"`) {
		t.Errorf("date input not defaulted to today:\n%s", body)
	}
}

func TestPastAndFutureDatesAreAccepted(t *testing.T) {
	ta := newTestApp(t)
	for _, date := range []string{"2020-01-15", "2031-12-31"} {
		r := ta.post("/transactions", expenseForm(date, "5", "Thing", "new", "Misc", "new", "Other"))
		if r.Status != http.StatusSeeOther {
			t.Fatalf("date %s: status %d, want 303", date, r.Status)
		}
		if body := ta.follow(r).Body; !strings.Contains(body, date) {
			t.Errorf("list missing %s", date)
		}
	}
}

func TestInvalidAmountsAreRejectedWithMessage(t *testing.T) {
	for _, amount := range []string{"0", "-5", "12.50", "12,50", "abc", ""} {
		t.Run("amount="+amount, func(t *testing.T) {
			ta := newTestApp(t)
			r := ta.post("/transactions", expenseForm("2026-10-05", amount, "Lunch", "new", "Food", "new", "Restaurant"))
			if r.Status != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", r.Status)
			}
			if !strings.Contains(r.Body, "Amount must be a whole number of euros greater than zero") {
				t.Errorf("missing amount message in:\n%s", r.Body)
			}
			if strings.Contains(ta.get("/").Body, "Lunch") {
				t.Error("rejected expense was stored")
			}
		})
	}
}

func TestInvalidSubmissionKeepsEnteredValues(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("2026-09-01", "0", "Lunch", "new", "Food", "new", "Restaurant"))
	for _, want := range []string{`value="2026-09-01"`, `value="Lunch"`, `value="Food"`, `value="Restaurant"`} {
		if !strings.Contains(r.Body, want) {
			t.Errorf("form lost %s", want)
		}
	}
	// Nothing was created as a side effect of the failed submission.
	if body := ta.get("/").Body; strings.Contains(body, ">Food<") || strings.Contains(body, ">Restaurant<") {
		t.Errorf("failed submission created a Category/Subcategory:\n%s", body)
	}
}

func TestInvalidDateAndMissingFieldsAreRejected(t *testing.T) {
	cases := map[string]struct {
		form url.Values
		msg  string
	}{
		"bad date":        {expenseForm("05/10/2026", "5", "x", "new", "A", "new", "B"), "Date must be in the format YYYY-MM-DD"},
		"impossible date": {expenseForm("2026-02-30", "5", "x", "new", "A", "new", "B"), "Date must be in the format YYYY-MM-DD"},
		"no description":  {expenseForm("2026-10-05", "5", "  ", "new", "A", "new", "B"), "Description is required"},
		"no category":     {expenseForm("2026-10-05", "5", "x", "", "", "new", "B"), "Category is required"},
		"empty new cat":   {expenseForm("2026-10-05", "5", "x", "new", " ", "new", "B"), "Category is required"},
		"no subcategory":  {expenseForm("2026-10-05", "5", "x", "new", "A", "", ""), "Subcategory is required"},
		"unknown cat id":  {expenseForm("2026-10-05", "5", "x", "999", "", "new", "B"), "Category is required"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ta := newTestApp(t)
			r := ta.post("/transactions", c.form)
			if r.Status != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", r.Status)
			}
			if !strings.Contains(r.Body, c.msg) {
				t.Errorf("missing %q in:\n%s", c.msg, r.Body)
			}
		})
	}
}

func TestInlineCategoriesAreUsableImmediatelyAndInAnyCombination(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "40", "Dinner", "new", "Food", "new", "Restaurant"))
	ta.post("/transactions", expenseForm("2026-10-02", "30", "Party", "new", "Social", "new", "Cocktails"))

	body := ta.get("/").Body
	// New entries are offered in the form's selects.
	for _, name := range []string{"Food", "Social", "Restaurant", "Cocktails"} {
		if !strings.Contains(body, `>`+name+`</option>`) {
			t.Errorf("form does not offer %q", name)
		}
	}

	// Reuse existing ids in a combination not entered before.
	catID := optionValue(t, body, "category", "Food")
	subID := optionValue(t, body, "subcategory", "Cocktails")
	r := ta.post("/transactions", expenseForm("2026-10-03", "12", "Drinks", catID, "", subID, ""))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("existing ids: status %d, want 303", r.Status)
	}
	if !strings.Contains(ta.follow(r).Body, "Drinks") {
		t.Error("expense with existing Category/Subcategory not listed")
	}
}

func TestNewCategoryNamesReuseExistingCaseInsensitively(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "1", "a", "new", "Food", "new", "Restaurant"))
	ta.post("/transactions", expenseForm("2026-10-01", "1", "b", "new", "food", "new", "RESTAURANT"))
	body := strings.ToLower(ta.get("/").Body)
	if n := strings.Count(body, `>food</option>`); n != 1 {
		t.Errorf("Category offered %d times, want 1", n)
	}
	if n := strings.Count(body, `>restaurant</option>`); n != 1 {
		t.Errorf("Subcategory offered %d times, want 1", n)
	}
}

func TestAmountFormatting(t *testing.T) {
	ta := newTestApp(t)
	amounts := map[string]string{"7": "€7", "999": "€999", "1000": "€1.000", "1234": "€1.234", "1234567": "€1.234.567"}
	for in, want := range amounts {
		ta.post("/transactions", expenseForm("2026-10-05", in, "amt"+in, "new", "C", "new", "S"))
		if body := ta.get("/").Body; !strings.Contains(body, ">"+want+"<") {
			t.Errorf("amount %s: want %s in its own element in list", in, want)
		}
	}
}

func TestListIsNewestFirst(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-01-01", "1", "entry-older", "new", "C", "new", "S"))
	ta.post("/transactions", expenseForm("2026-03-01", "1", "entry-newer", "new", "C", "new", "S"))
	body := ta.get("/").Body
	if strings.Index(body, "entry-newer") > strings.Index(body, "entry-older") {
		t.Error("list is not ordered newest first")
	}
}

func TestDataPersistsAcrossRestart(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "50", "Persisted", "new", "C", "new", "S"))
	ta.restart()
	if !strings.Contains(ta.get("/").Body, "Persisted") {
		t.Error("Transaction lost after restart")
	}
}

func TestPageIsPhoneFriendly(t *testing.T) {
	ta := newTestApp(t)
	if !strings.Contains(ta.get("/").Body, `<meta name="viewport" content="width=device-width, initial-scale=1">`) {
		t.Error("missing responsive viewport meta tag")
	}
}

// optionValue finds the value of the <option> labelled label inside the
// <select name="selectName"> of an HTML page.
func optionValue(t *testing.T, html, selectName, label string) string {
	t.Helper()
	loc := regexp.MustCompile(`<select[^>]*name="` + regexp.QuoteMeta(selectName) + `"`).FindStringIndex(html)
	if loc == nil {
		t.Fatalf("no select %q", selectName)
	}
	sel := html[loc[0] : loc[0]+strings.Index(html[loc[0]:], "</select>")]
	m := regexp.MustCompile(`<option value="([^"]*)"[^>]*>` + regexp.QuoteMeta(label) + `</option>`).FindStringSubmatch(sel)
	if m == nil {
		t.Fatalf("no option %q in select %q:\n%s", label, selectName, sel)
	}
	return m[1]
}
