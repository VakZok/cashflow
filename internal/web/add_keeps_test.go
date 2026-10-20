package web_test

import (
	"regexp"
	"strings"
	"testing"
)

// selectedLabel is the label of the selected option of the select with that id on the Add page ("" if none is).
func selectedLabel(t *testing.T, body, id string) string {
	t.Helper()
	sel := regexp.MustCompile(`(?s)<select id="` + id + `"[^>]*>(.*?)</select>`).FindStringSubmatch(body)
	if sel == nil {
		t.Fatalf("no select %q in:\n%s", id, body)
	}
	m := regexp.MustCompile(`<option value="[^"]*" selected>([^<]*)</option>`).FindStringSubmatch(sel[1])
	if m == nil {
		return ""
	}
	return m[1]
}

// inputValue is the value attribute of the input with that id on the Add page.
func inputValue(t *testing.T, body, id string) string {
	t.Helper()
	m := regexp.MustCompile(`<input id="` + id + `"[^>]*? value="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no input %q in:\n%s", id, body)
	}
	return m[1]
}

// inputHidden reports whether the input with that id is rendered hidden.
func inputHidden(t *testing.T, body, id string) bool {
	t.Helper()
	m := regexp.MustCompile(`<input id="` + id + `"[^>]*>`).FindString(body)
	if m == "" {
		t.Fatalf("no input %q in:\n%s", id, body)
	}
	return strings.Contains(m, " hidden")
}

func TestAnAddedExpenseKeepsDateTypeAndItsChoices(t *testing.T) {
	ta := newTestApp(t)
	page := ta.follow(ta.post("/transactions", expenseForm("2026-09-01", "5", "Lunch", "new", "Food", "new", "Cafe")))
	if v := inputValue(t, page.Body, "date"); v != "2026-09-01" {
		t.Errorf("date %q", v)
	}
	if got := selectedLabel(t, page.Body, "type"); got != "Expense" {
		t.Errorf("type %q", got)
	}
	if got := selectedLabel(t, page.Body, "category"); got != "Food" {
		t.Errorf("category %q", got)
	}
	if got := selectedLabel(t, page.Body, "subcategory"); got != "Cafe" {
		t.Errorf("subcategory %q", got)
	}
	if inputValue(t, page.Body, "amount") != "" || inputValue(t, page.Body, "description") != "" {
		t.Error("amount or description was kept")
	}
	if !inputHidden(t, page.Body, "new_category") || inputValue(t, page.Body, "new_category") != "" {
		t.Error("the new… name input is not hidden and empty")
	}
}

func TestAnAddedLevyKeepsItsLevyChoices(t *testing.T) {
	ta := newTestApp(t)
	page := ta.follow(ta.post("/transactions", levyForm("2026-09-01", "5", "Tax", "new", "Income tax", "new", "Q3")))
	if got := selectedLabel(t, page.Body, "type"); got != "Levy" {
		t.Errorf("type %q", got)
	}
	if got := selectedLabel(t, page.Body, "levy_category"); got != "Income tax" {
		t.Errorf("levy category %q", got)
	}
	if got := selectedLabel(t, page.Body, "levy_subcategory"); got != "Q3" {
		t.Errorf("levy subcategory %q", got)
	}
	if got := selectedLabel(t, page.Body, "category"); got != "" {
		t.Errorf("category %q, want empty", got)
	}
	if inputValue(t, page.Body, "amount") != "" || inputValue(t, page.Body, "description") != "" {
		t.Error("amount or description was kept")
	}
}

func TestAnAddedInvestmentKeepsItsHolding(t *testing.T) {
	ta := newTestApp(t)
	page := ta.follow(ta.post("/transactions", holdingForm("investment", "2026-09-01", "5", "new", "ETF")))
	if got := selectedLabel(t, page.Body, "type"); got != "Investment" {
		t.Errorf("type %q", got)
	}
	if got := selectedLabel(t, page.Body, "holding"); got != "ETF" {
		t.Errorf("holding %q", got)
	}
	if !inputHidden(t, page.Body, "new_holding") || inputValue(t, page.Body, "new_holding") != "" {
		t.Error("the new… name input is not hidden and empty")
	}
}

func TestANewCategoryStaysSelectedAsTheEntryTheAddCreated(t *testing.T) {
	ta := newTestApp(t)
	page := ta.follow(ta.post("/transactions", expenseForm("2026-09-01", "5", "Food", "new", "Pets", "new", "Vet")))
	if got := selectedLabel(t, page.Body, "category"); got != "Pets" {
		t.Errorf("category %q, want Pets", got)
	}
	if !inputHidden(t, page.Body, "new_category") || inputValue(t, page.Body, "new_category") != "" {
		t.Error("the new… name input is not hidden and empty")
	}
}

func TestTheSuccessTextShowsOnceAfterTheKeptValuesRedirect(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("2026-09-01", "5", "Lunch", "new", "Food", "new", "Cafe"))
	if b := addButton(t, ta.follow(r).Body); b != "Added successfully!" {
		t.Errorf("button %q", b)
	}
	// A fresh visit of the page does not carry the flag.
	if b := addButton(t, ta.get("/").Body); b != "Add" {
		t.Errorf("button %q, want Add", b)
	}
}

func TestTheAddPageWithoutAddressValuesShowsAnExpenseDatedToday(t *testing.T) {
	body := newTestApp(t).get("/").Body
	if v := inputValue(t, body, "date"); v != "2026-10-05" {
		t.Errorf("date %q", v)
	}
	if got := selectedLabel(t, body, "type"); got != "Expense" {
		t.Errorf("type %q", got)
	}
}

func TestTheAddPageFillsAllFieldsFromItsAddress(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/?type=income&date=2026-01-02&amount=7&description=Pay&category=new&new_category=Job").Body
	if inputValue(t, body, "date") != "2026-01-02" || inputValue(t, body, "amount") != "7" || inputValue(t, body, "description") != "Pay" {
		t.Errorf("date, amount or description not filled:\n%s", body)
	}
	if got := selectedLabel(t, body, "type"); got != "Income" {
		t.Errorf("type %q", got)
	}
	if got := selectedLabel(t, body, "category"); got != "new…" {
		t.Errorf("category %q", got)
	}
	if inputValue(t, body, "new_category") != "Job" || inputHidden(t, body, "new_category") {
		t.Error("the new… name is not filled and shown")
	}
}

func TestUnknownAddressValuesFallBackWithoutHighlights(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/?type=bogus&category=999&subcategory=abc&holding=5&levy_category=3").Body
	if got := selectedLabel(t, body, "type"); got != "Expense" {
		t.Errorf("type %q, want the default Expense", got)
	}
	for _, id := range []string{"category", "subcategory", "levy_category", "holding"} {
		if got := selectedLabel(t, body, id); got != "" {
			t.Errorf("%s = %q, want empty", id, got)
		}
	}
	if h := highlighted(body); len(h) != 0 {
		t.Errorf("highlighted %v", h)
	}
	if b := addButton(t, body); b != "Add" {
		t.Errorf("button %q", b)
	}
	if strings.Contains(body, `class="bubble"`) {
		t.Error("a bubble is shown")
	}
}
