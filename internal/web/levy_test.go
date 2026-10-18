package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// levyForm builds a Levy submission. levyCategory/levySubcategory are an
// existing id or "new" together with the new name.
func levyForm(date, amount, desc, levyCategory, newLevyCategory, levySubcategory, newLevySubcategory string) url.Values {
	return url.Values{
		"type":                 {"levy"},
		"date":                 {date},
		"amount":               {amount},
		"description":          {desc},
		"levy_category":        {levyCategory},
		"new_levy_category":    {newLevyCategory},
		"levy_subcategory":     {levySubcategory},
		"new_levy_subcategory": {newLevySubcategory},
	}
}

func TestAddLevyWithInlineLevyCategoriesShowsInList(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", levyForm("2026-10-01", "250", "Income tax", "new", "Tax", "new", "Federal"))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status %d, want 303\n%s", r.Status, r.Body)
	}
	body := ta.follow(r).Body
	wantAll(t, body, "Income tax", ">250<", "Levy", "Tax", "Federal")
}

func TestLevyCategoriesAreSeparateFromCategories(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", levyForm("2026-10-01", "5", "Tax", "new", "Shared", "new", "LevySub"))
	ta.post("/transactions", expenseForm("2026-10-02", "5", "Food", "new", "Shared", "new", "Sub"))
	body := ta.get("/").Body

	sel := func(name string) string {
		return regexp.MustCompile(`(?s)<select[^>]*name="` + name + `".*?</select>`).FindString(body)
	}
	for name, wantIn := range map[string]string{"category": "Shared", "levy_category": "Shared", "subcategory": "Sub", "levy_subcategory": "LevySub"} {
		if !strings.Contains(sel(name), ">"+wantIn+"</option>") {
			t.Errorf("%s does not offer %q", name, wantIn)
		}
	}
	if strings.Contains(sel("category"), "LevySub") || strings.Contains(sel("subcategory"), "LevySub") ||
		strings.Contains(sel("levy_subcategory"), ">Sub</option>") {
		t.Error("a list leaks into the other")
	}
}

func TestLevyRejectsCategoryAndSubcategory(t *testing.T) {
	for field, value := range map[string]string{"category": "new", "new_category": "Food", "subcategory": "new", "new_subcategory": "Rest", "holding": "new", "new_holding": "Fund"} {
		ta := newTestApp(t)
		f := levyForm("2026-10-05", "5", "Tax", "new", "T", "new", "S")
		f.Set(field, value)
		if r := ta.post("/transactions", f); r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", field, r.Status)
		}
	}
}

func TestIncomeAndExpenseRejectLevyCategories(t *testing.T) {
	for _, typ := range []string{"income", "expense"} {
		for field, value := range map[string]string{"levy_category": "new", "new_levy_category": "Tax", "levy_subcategory": "new", "new_levy_subcategory": "Fed"} {
			ta := newTestApp(t)
			f := validForm(typ, "2026-10-05", "5", "Thing")
			f.Set(field, value)
			if r := ta.post("/transactions", f); r.Status != http.StatusUnprocessableEntity {
				t.Errorf("%s/%s: status %d, want 422", typ, field, r.Status)
			}
		}
	}
}

func TestLevyRequiresDescriptionAndLevyCategories(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", levyForm("2026-10-05", "5", "", "", "", "", ""))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	wantAll(t, r.Body, "Description is required.", "Levy Category is required.", "Levy Subcategory is required.")
}

func TestRejectedLevyCreatesNoLevyCategory(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", levyForm("2026-10-05", "0", "Tax", "new", "Ghost", "new", "Spirit"))
	if body := ta.get("/").Body; strings.Contains(body, ">Ghost</option>") || strings.Contains(body, ">Spirit</option>") {
		t.Error("rejected submission created a Levy Category")
	}
}

func TestDuplicateLevyIsRejectedWithMessage(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", levyForm("2026-10-01", "5", "Tax", "new", "T", "new", "S"))
	r := ta.post("/transactions", levyForm("2026-10-01", "9", "tax", "new", "U", "new", "V"))
	if r.Status != http.StatusUnprocessableEntity || !strings.Contains(r.Body, "already exists") {
		t.Errorf("status %d, want 422 with duplicate message", r.Status)
	}
}

func TestFormShowsLevyFieldsOnlyForLevy(t *testing.T) {
	ta := newTestApp(t)
	disabled := func(body, field string) bool {
		tag := regexp.MustCompile(`<(?:input|select)[^>]*name="` + field + `"[^>]*>`).FindString(body)
		if tag == "" {
			t.Fatalf("no input %q", field)
		}
		return regexp.MustCompile(`\sdisabled[\s>]`).MatchString(tag)
	}
	levy := ta.post("/transactions", levyForm("2026-10-05", "0", "x", "", "", "", "")).Body
	exp := ta.get("/").Body
	for _, f := range []string{"levy_category", "new_levy_category", "levy_subcategory", "new_levy_subcategory"} {
		if disabled(levy, f) || !disabled(exp, f) {
			t.Errorf("%s: wrong disabled state", f)
		}
	}
	for _, f := range []string{"category", "new_category", "subcategory", "new_subcategory"} {
		if !disabled(levy, f) || disabled(exp, f) {
			t.Errorf("%s: wrong disabled state", f)
		}
	}
}

func TestLevyDoesNotChangeReport(t *testing.T) {
	ta := newTestApp(t)
	ta.seedIn("expense", "2026-10-02", "300", "Food", "Restaurant")
	ta.seedIn("income", "2026-10-03", "900", "Salary", "Work")
	const path = "/report?period=month&date=2026-10"
	before := ta.mustReport(path)
	beforeExpense, beforeExpenseAmts := ta.chart(path)

	if r := ta.post("/transactions", levyForm("2026-10-04", "400", "Tax", "new", "Tax", "new", "Federal")); r.Status != http.StatusSeeOther {
		t.Fatalf("levy POST = %d\n%s", r.Status, r.Body)
	}
	if after := ta.mustReport(path); after != before {
		t.Errorf("report changed after a Levy:\n%s", after)
	}
	l, a := ta.chart(path)
	wantChart(t, l, a, beforeExpense, beforeExpenseAmts)
	// A Levy is not a chart Type.
	l, a = ta.chart(path + "&type=levy&group=category")
	wantChart(t, l, a, beforeExpense, beforeExpenseAmts)
}
