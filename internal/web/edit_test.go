package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// editRow returns the <li> of the Transaction with the id, or "" if none.
func editRow(body string, id string) string {
	return regexp.MustCompile(`(?s)<li id="tx-` + id + `".*?\n  </li>`).FindString(body)
}

func TestClickingARowOffersItsEditFormPrefilled(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", validForm("investment", "2026-10-03", "90", "ETF World"))

	list := ta.get("/").Body
	wantAll(t, editRow(list, "1"), `href="/?edit=1"`, "Lunch")

	row := editRow(ta.get("/?edit=1").Body, "1")
	wantAll(t, row,
		`action="/transactions/1"`,
		`value="2026-10-02"`, `value="12"`, `value="Lunch"`,
		`<option value="expense" selected>`,
		`>Food</option>`, `>Cafe</option>`,
		`href="/"`, "Discard",
		"Change", "Delete", `formaction="/transactions/1/delete"`)
	if strings.Contains(row, "Add</button>") {
		t.Error("edit row still offers Add")
	}
	// Only the row being edited turns into a form.
	if other := editRow(ta.get("/?edit=1").Body, "2"); strings.Contains(other, "<form") {
		t.Errorf("other row is a form: %s", other)
	}

	inv := editRow(ta.get("/?edit=2").Body, "2")
	wantAll(t, inv, `<option value="investment" selected>`, `value="90"`, `>ETF World</option>`)
}

func TestEditingAMissingTransactionShowsTheNormalList(t *testing.T) {
	ta := newTestApp(t)
	for _, p := range []string{"/?edit=99", "/?edit=abc"} {
		r := ta.get(p)
		if r.Status != http.StatusOK || strings.Contains(r.Body, `action="/transactions/`) {
			t.Errorf("%s: status %d, body has edit form", p, r.Status)
		}
	}
}

func TestChangeUpdatesTheTransactionAndLandsOnItsRow(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	cat := optionValue(t, ta.get("/").Body, "category", "Food")
	sub := optionValue(t, ta.get("/").Body, "subcategory", "Cafe")

	r := ta.post("/transactions/1", expenseForm("2026-10-04", "15", "Late lunch", cat, "", sub, ""))
	if r.Status != http.StatusSeeOther || r.Location != "/#tx-1" {
		t.Fatalf("status %d, location %q; want 303 /#tx-1\n%s", r.Status, r.Location, r.Body)
	}
	row := editRow(ta.follow(r).Body, "1")
	wantAll(t, row, "Late lunch", "€15", "2026-10-04")
	if strings.Contains(row, ">Lunch<") {
		t.Errorf("old description still shown: %s", row)
	}
}

func TestChangeWithoutChangingAnythingSucceeds(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	cat := optionValue(t, ta.get("/").Body, "category", "Food")
	sub := optionValue(t, ta.get("/").Body, "subcategory", "Cafe")
	r := ta.post("/transactions/1", expenseForm("2026-10-02", "12", "Lunch", cat, "", sub, ""))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status %d, want 303\n%s", r.Status, r.Body)
	}
	wantAll(t, editRow(ta.follow(r).Body, "1"), "Lunch", "€12", "Food", "Cafe")
}

func TestChangingTheTypeAppliesTheNewTypesFields(t *testing.T) {
	pairs := [][2]string{{"expense", "investment"}, {"investment", "expense"}, {"income", "divestment"}, {"divestment", "income"}, {"expense", "income"}, {"investment", "divestment"}}
	for i, p := range pairs {
		from, to := p[0], p[1]
		ta := newTestApp(t)
		ta.post("/transactions", validForm(from, "2026-10-02", "10", "Thing"))
		r := ta.post("/transactions/1", validForm(to, "2026-10-03", "20", "Other"))
		if r.Status != http.StatusSeeOther {
			t.Fatalf("%d %s->%s: status %d\n%s", i, from, to, r.Status, r.Body)
		}
		row := editRow(ta.follow(r).Body, "1")
		wantAll(t, row, "€20", "2026-10-03")
		if to == "investment" || to == "divestment" {
			wantAll(t, row, "<span>Other</span>")
			if strings.Contains(row, "Misc") || strings.Contains(row, "Thing") {
				t.Errorf("%s->%s kept old fields: %s", from, to, row)
			}
		} else {
			wantAll(t, row, "<span>Other</span>", "Misc", "Other")
		}
	}
}

func TestChangeRejectsWhatCreateRejectsAndKeepsTheRowInEditMode(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", validForm("expense", "2026-10-02", "10", "Lunch"))
	cases := map[string]func(f map[string][]string){
		"amount":          func(f map[string][]string) { f["amount"] = []string{"0"} },
		"date":            func(f map[string][]string) { f["date"] = []string{"1999-01-01"} },
		"description":     func(f map[string][]string) { f["description"] = []string{""} },
		"holding on exp.": func(f map[string][]string) { f["holding"] = []string{"new"}; f["new_holding"] = []string{"X"} },
	}
	for name, mutate := range cases {
		f := validForm("expense", "2026-10-02", "10", "Lunch")
		f.Set("category", optionValue(t, ta.get("/").Body, "category", "Misc"))
		f.Set("new_category", "")
		f.Set("subcategory", optionValue(t, ta.get("/").Body, "subcategory", "Other"))
		f.Set("new_subcategory", "")
		mutate(f)
		r := ta.post("/transactions/1", f)
		if r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", name, r.Status)
			continue
		}
		row := editRow(r.Body, "1")
		wantAll(t, row, `role="alert"`, `action="/transactions/1"`)
	}
	// The stored Transaction is unchanged.
	wantAll(t, editRow(ta.get("/").Body, "1"), "Lunch", "€10")
}

func TestEditIsNeverADuplicateOfItselfButOfOthers(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", validForm("expense", "2026-10-02", "10", "Lunch"))
	ta.post("/transactions", validForm("expense", "2026-10-03", "10", "Dinner"))
	ta.post("/transactions", validForm("investment", "2026-10-02", "10", "ETF"))
	ta.post("/transactions", validForm("investment", "2026-10-03", "10", "Fund"))
	body := ta.get("/").Body
	cat, sub := optionValue(t, body, "category", "Misc"), optionValue(t, body, "subcategory", "Other")
	exp := func(date, desc string) map[string][]string { return expenseForm(date, "11", desc, cat, "", sub, "") }

	// Same date and description (other case) as itself: fine.
	if r := ta.post("/transactions/1", exp("2026-10-02", "LUNCH")); r.Status != http.StatusSeeOther {
		t.Errorf("own date+description: status %d\n%s", r.Status, r.Body)
	}
	// Onto another Transaction's date and description: rejected, with the message.
	r := ta.post("/transactions/1", exp("2026-10-03", "dinner"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("onto another: status %d, want 422", r.Status)
	}
	wantAll(t, editRow(r.Body, "1"), errDuplicateText)

	// Holdings: own date+holding fine, another's rejected.
	hold := optionValue(t, body, "holding", "ETF")
	other := optionValue(t, body, "holding", "Fund")
	if r := ta.post("/transactions/3", holdingForm("investment", "2026-10-02", "99", hold, "")); r.Status != http.StatusSeeOther {
		t.Errorf("own date+holding: status %d\n%s", r.Status, r.Body)
	}
	r = ta.post("/transactions/3", holdingForm("divestment", "2026-10-03", "99", other, ""))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("holding onto another: status %d, want 422", r.Status)
	}
	wantAll(t, editRow(r.Body, "3"), "date and holding")
}

const errDuplicateText = "date and description already exists"

func TestEditAndDeleteOfAMissingTransactionAre404(t *testing.T) {
	ta := newTestApp(t)
	for _, p := range []string{"/transactions/9", "/transactions/9/delete", "/transactions/x"} {
		if r := ta.post(p, validForm("expense", "2026-10-02", "10", "Lunch")); r.Status != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", p, r.Status)
		}
	}
}

func TestDeleteRemovesTheTransactionAndReturnsToTheList(t *testing.T) {
	ta := newTestApp(t)
	for _, typ := range allTypes {
		ta.post("/transactions", validForm(typ, "2026-10-02", "10", "Item "+typ))
	}
	r := ta.post("/transactions/2/delete", nil)
	if r.Status != http.StatusSeeOther || r.Location != "/" {
		t.Fatalf("status %d, location %q; want 303 /", r.Status, r.Location)
	}
	body := ta.follow(r).Body
	if editRow(body, "2") != "" || strings.Contains(body, "Item income") {
		t.Error("deleted Transaction still listed")
	}
	wantAll(t, editRow(body, "1"), "Item expense")
	wantAll(t, editRow(body, "3"), "Item investment")
}

func TestChangeIsTheFormsDefaultButton(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", validForm("expense", "2026-10-02", "10", "Lunch"))
	row := editRow(ta.get("/?edit=1").Body, "1")
	// Enter in a field presses the first submit button: that must not be Delete.
	if first := strings.Index(row, "<button"); first < 0 || strings.Index(row, "Change") > strings.Index(row, "Delete") {
		t.Errorf("Delete precedes Change in the row:\n%s", row)
	}
}

func TestInvalidChangeOfAMissingTransactionIs404(t *testing.T) {
	ta := newTestApp(t)
	f := validForm("expense", "2026-10-02", "0", "Lunch")
	if r := ta.post("/transactions/9", f); r.Status != http.StatusNotFound {
		t.Errorf("status %d, want 404", r.Status)
	}
}
