package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// highlighted lists the ids of the inputs of an Add page body that carry the red highlight.
func highlighted(body string) []string {
	var ids []string
	for _, m := range regexp.MustCompile(`<(?:input|select) id="([^"]*)" class="invalid"`).FindAllStringSubmatch(body, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// addButton is the label of the Add button.
func addButton(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`<button class="primary" type="submit" data-resting="Add"[^>]*>([^<]*)</button>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no Add button in:\n%s", body)
	}
	return m[1]
}

func TestMissingFieldsAreHighlightedAndTheButtonSaysSo(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("2026-10-05", "5", "", "", "", "new", "Cafe"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	got := highlighted(r.Body)
	if strings.Join(got, ",") != "description,category" {
		t.Errorf("highlighted %v, want description and category only", got)
	}
	if b := addButton(t, r.Body); b != "Missing fields" {
		t.Errorf("button %q, want Missing fields", b)
	}
	if strings.Contains(r.Body, `class="bubble"`) || strings.Contains(r.Body, `class="errors"`) {
		t.Error("a bubble or error box is shown")
	}
}

func TestANewChoiceWithoutANameAndAClearedDateAreMissingFields(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("", "5", "Lunch", "new", " ", "new", "Cafe"))
	if got := strings.Join(highlighted(r.Body), ","); got != "date,new_category" {
		t.Errorf("highlighted %q, want date and new_category", got)
	}
	// The highlight comes back when the problem is still there.
	r = ta.post("/transactions", expenseForm("", "5", "Lunch", "new", " ", "new", "Cafe"))
	if got := strings.Join(highlighted(r.Body), ","); got != "date,new_category" {
		t.Errorf("submitting again highlighted %q", got)
	}
}

func TestAHoldingTypeHighlightsAMissingHoldingOnly(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", holdingForm("investment", "2026-10-05", "5", "", ""))
	if got := strings.Join(highlighted(r.Body), ","); got != "holding" {
		t.Errorf("highlighted %q, want holding", got)
	}
	if b := addButton(t, r.Body); b != "Missing fields" {
		t.Errorf("button %q", b)
	}
}

func TestAnAmountProblemShowsTheBubbleAndLeavesTheButtonAlone(t *testing.T) {
	for _, amount := range []string{"0", "100000001", "", "12.5", "abc"} {
		ta := newTestApp(t)
		r := ta.post("/transactions", expenseForm("2026-10-05", amount, "Lunch", "new", "Food", "new", "Cafe"))
		wantAll(t, r.Body, `<span class="bubble" role="alert">1 to 100.000.000</span>`)
		if b := addButton(t, r.Body); b != "Add" {
			t.Errorf("amount %q: button %q, want Add", amount, b)
		}
		if len(highlighted(r.Body)) != 0 {
			t.Errorf("amount %q highlights %v", amount, highlighted(r.Body))
		}
	}
	ta := newTestApp(t)
	if body := ta.get("/").Body; strings.Contains(body, `class="bubble"`) {
		t.Error("a fresh Add page shows the bubble")
	}
}

func TestADuplicateShowsAlreadyExistsOnTheButton(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-05", "5", "Lunch", "new", "Food", "new", "Cafe"))
	r := ta.post("/transactions", expenseForm("2026-10-05", "7", "lunch", "new", "Food", "new", "Cafe"))
	if r.Status != http.StatusUnprocessableEntity || addButton(t, r.Body) != "Already exists" {
		t.Errorf("status %d, button %q", r.Status, addButton(t, r.Body))
	}
	if len(highlighted(r.Body)) != 0 {
		t.Errorf("a Duplicate highlights %v", highlighted(r.Body))
	}

	ta.post("/transactions", holdingForm("investment", "2026-10-05", "5", "new", "ETF"))
	r = ta.post("/transactions", holdingForm("divestment", "2026-10-05", "9", "new", "etf"))
	if r.Status != http.StatusUnprocessableEntity || addButton(t, r.Body) != "Already exists" {
		t.Errorf("holding: status %d, button %q", r.Status, addButton(t, r.Body))
	}
}

func TestAHandCraftedRequestShowsInvalidInputWithoutHighlightsAndSavesNothing(t *testing.T) {
	ta := newTestApp(t)
	cases := map[string]func(f map[string][]string){
		"unknown type":      func(f map[string][]string) { f["type"] = []string{"bogus"} },
		"expense + holding": func(f map[string][]string) { f["holding"] = []string{"new"}; f["new_holding"] = []string{"Fund"} },
		"long description":  func(f map[string][]string) { f["description"] = []string{strings.Repeat("d", 101)} },
		"long new name":     func(f map[string][]string) { f["new_category"] = []string{strings.Repeat("c", 51)} },
		"out of range date": func(f map[string][]string) { f["date"] = []string{"10000-01-01"} },
		"unknown category":  func(f map[string][]string) { f["category"] = []string{"999"}; f["new_category"] = []string{""} },
	}
	for name, mutate := range cases {
		f := expenseForm("2026-10-05", "5", "Lunch", "new", "Food", "new", "Cafe")
		mutate(f)
		r := ta.post("/transactions", f)
		if r.Status != http.StatusUnprocessableEntity || addButton(t, r.Body) != "Invalid input" {
			t.Errorf("%s: status %d, button %q", name, r.Status, addButton(t, r.Body))
		}
		if len(highlighted(r.Body)) != 0 {
			t.Errorf("%s highlights %v", name, highlighted(r.Body))
		}
	}
	if body := ta.get("/transactions").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a hand-crafted request saved something:\n%s", body)
	}
}

func TestMissingFieldsWinOverOtherProblemsOnTheButton(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("2026-10-05", "0", "", "new", "Food", "new", "Cafe"))
	if b := addButton(t, r.Body); b != "Missing fields" {
		t.Errorf("button %q", b)
	}
	wantAll(t, r.Body, "1 to 100.000.000")
}

func TestASuccessfulAddRedirectsToAPageWhoseButtonShowsTheSuccessText(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("2026-10-05", "5", "Lunch", "new", "Food", "new", "Cafe"))
	if r.Status != http.StatusSeeOther || r.Location != "/?added=1" {
		t.Fatalf("status %d, location %q", r.Status, r.Location)
	}
	page := ta.follow(r)
	if b := addButton(t, page.Body); b != "Added successfully!" {
		t.Errorf("button %q", b)
	}
	wantAll(t, page.Body, `data-flash`, `name="date" value="2026-10-05"`) // the form is reset
	if strings.Contains(page.Body, `value="Lunch"`) {
		t.Error("the form kept the added Transaction's values")
	}
	if b := addButton(t, ta.get("/").Body); b != "Add" {
		t.Errorf("plain page: button %q, want Add", b)
	}
}

func TestTheSharedErrorScriptAndStyleAreOnThePages(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-05", "5", "Lunch", "new", "Food", "new", "Cafe"))
	// The Transactions page's edit rows reuse them.
	for _, path := range []string{"/", "/transactions?edit=1"} {
		wantAll(t, ta.get(path).Body, "function onEdit", "input.invalid:not([type=hidden])", ".bubble")
	}
	wantAll(t, ta.get("/").Body, `<form method="post" action="/transactions" data-keep-scroll>`)
}
