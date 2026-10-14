package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const errIncomplete = "All fields must be filled in for every row."

// repeatingForm builds a Save List submission; each row is
// {type, amount, description, category, newCategory, subcategory, newSubcategory}.
func repeatingForm(rows ...[7]string) url.Values {
	f := url.Values{}
	for i, r := range rows {
		n := strconv.Itoa(i)
		f.Add("row", n)
		for j, name := range []string{"type", "amount", "description", "category", "new_category", "subcategory", "new_subcategory"} {
			f.Set(name+"_"+n, r[j])
		}
	}
	return f
}

func repRow(typ, amount, desc string) [7]string {
	return [7]string{typ, amount, desc, "new", "Cat", "new", "Sub"}
}

func (ta *testApp) saveList(f url.Values) response {
	ta.t.Helper()
	return ta.post("/repeating", f)
}

func mustSave(t *testing.T, ta *testApp, f url.Values) response {
	t.Helper()
	r := ta.saveList(f)
	if r.Status != http.StatusSeeOther {
		t.Fatalf("Save List = %d, want 303\n%s", r.Status, r.Body)
	}
	return ta.follow(r)
}

func rowCount(body string) int { return strings.Count(body, `name="row"`) }

func TestRepeatingPageIsLinkedAndStartsWithOneBlankRow(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/").Body, `href="/repeating"`, ">Repeating<")
	r := ta.get("/repeating")
	if r.Status != http.StatusOK {
		t.Fatalf("status %d", r.Status)
	}
	wantAll(t, r.Body, `href="/"`, `href="/report"`, "Save List")
	if n := rowCount(r.Body); n != 1 {
		t.Errorf("%d rows, want 1", n)
	}
	for _, unwanted := range []string{"Add Transactions", `type="date"`} {
		if strings.Contains(r.Body, unwanted) {
			t.Errorf("page contains %q", unwanted)
		}
	}
}

func TestSavedListIsGroupedIncomeThenExpenseInSavedOrder(t *testing.T) {
	ta := newTestApp(t)
	body := mustSave(t, ta, repeatingForm(
		repRow("expense", "800", "Rent"),
		repRow("income", "2000", "Salary"),
		repRow("expense", "30", "Gym"),
		repRow("income", "100", "Pension"),
	)).Body
	wantAll(t, body, "List saved")
	order := []string{"Salary", "Pension", "Rent", "Gym"}
	last := -1
	for _, d := range order {
		i := strings.Index(body, `value="`+d+`"`)
		if i < 0 || i < last {
			t.Fatalf("%q out of order (at %d, previous %d) in:\n%s", d, i, last, body)
		}
		last = i
	}
	wantAll(t, body, `value="2000"`, `value="800"`, ">Cat</option>", ">Sub</option>")

	ta.restart()
	if n := rowCount(ta.get("/repeating").Body); n != 4 {
		t.Errorf("after restart %d rows, want 4", n)
	}
}

func TestSavedRowsPreselectTheirTypeCategoryAndSubcategory(t *testing.T) {
	ta := newTestApp(t)
	body := mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent"))).Body
	wantAll(t, body, `<option value="expense" selected>`, `<option value="1" selected>Cat</option>`, `<option value="1" selected>Sub</option>`)
}

func TestIncompleteRowBlocksSaveAndKeepsTypedValues(t *testing.T) {
	ta := newTestApp(t)
	for name, row := range map[string][7]string{
		"amount":      {"income", "", "Salary", "new", "Newcat", "new", "Newsub"},
		"description": {"income", "5", "", "new", "Newcat", "new", "Newsub"},
		"category":    {"income", "5", "Salary", "", "", "new", "Newsub"},
		"subcategory": {"income", "5", "Salary", "new", "Newcat", "", ""},
		"new name":    {"income", "5", "Salary", "new", "", "new", "Newsub"},
	} {
		r := ta.saveList(repeatingForm(repRow("expense", "9", "Fine"), row))
		if r.Status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: status %d, want 422", name, r.Status)
		}
		wantAll(t, r.Body, errIncomplete, `value="Fine"`, `value="9"`)
		if name != "description" {
			wantAll(t, r.Body, `value="Salary"`)
		}
	}
	body := ta.get("/repeating").Body
	if rowCount(body) != 1 || strings.Contains(body, "Fine") {
		t.Errorf("something was saved:\n%s", body)
	}
	if strings.Contains(body, "Newcat") || strings.Contains(body, "Cat</option>") {
		t.Errorf("a new… name was created by a failed save:\n%s", body)
	}
}

func TestErrorBoxComesAfterTheSaveListButton(t *testing.T) {
	ta := newTestApp(t)
	body := ta.saveList(repeatingForm([7]string{"income", "", "x", "", "", "", ""})).Body
	b, e := strings.Index(body, "Save List"), strings.Index(body, errIncomplete)
	if b < 0 || e < b {
		t.Errorf("button at %d, error at %d", b, e)
	}
}

func TestBlankRowsAreDroppedAndAnEmptyListRendersOneBlankRow(t *testing.T) {
	ta := newTestApp(t)
	blank := [7]string{"income", "", "", "", "", "", ""}
	body := mustSave(t, ta, repeatingForm(blank, repRow("income", "5", "Salary"), blank, [7]string{"expense", "", "  ", "", "", "", ""})).Body
	if n := rowCount(body); n != 1 {
		t.Errorf("%d rows, want 1", n)
	}
	body = mustSave(t, ta, repeatingForm(blank)).Body
	if n := rowCount(body); n != 1 || strings.Contains(body, "Salary") {
		t.Errorf("empty list: %d rows\n%s", n, body)
	}
}

func TestRemovingRowsAndAddingRowsReplacesTheList(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("income", "5", "Salary"), repRow("expense", "7", "Rent")))
	body := mustSave(t, ta, repeatingForm(repRow("expense", "7", "Rent"), repRow("expense", "3", "Gym"))).Body
	if strings.Contains(body, "Salary") || rowCount(body) != 2 {
		t.Errorf("list not replaced:\n%s", body)
	}
}

func TestNewNamesAreReusedAndCreatedOncePerSave(t *testing.T) {
	ta := newTestApp(t)
	body := mustSave(t, ta, repeatingForm(repRow("income", "5", "A"), repRow("expense", "6", "B"))).Body
	if n := strings.Count(body, ">Cat</option>"); n != 2 { // once in each of the two rows
		t.Errorf("Category offered %d times: %s", n, body)
	}
	// The Categories are the home form's too, and each was created once.
	home := ta.get("/").Body
	if strings.Count(home, ">Cat</option>") != 1 || strings.Count(home, ">Sub</option>") != 1 {
		t.Errorf("home form does not offer Cat and Sub exactly once:\n%s", home)
	}
}

func TestExistingCategoriesCanBePicked(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "5", "Lunch", "new", "Food", "new", "Restaurant"))
	body := mustSave(t, ta, repeatingForm([7]string{"expense", "40", "Lunch club", "1", "", "1", ""})).Body
	wantAll(t, body, `<option value="1" selected>Food</option>`, `<option value="1" selected>Restaurant</option>`)
}

func TestRepeatingCapIsFifty(t *testing.T) {
	ta := newTestApp(t)
	var rows [][7]string
	for i := 0; i < 50; i++ {
		rows = append(rows, repRow("expense", "1", "Row "+strconv.Itoa(i)))
	}
	if n := rowCount(mustSave(t, ta, repeatingForm(rows...)).Body); n != 50 {
		t.Fatalf("%d rows, want 50", n)
	}
	rows = append(rows, repRow("expense", "1", "One too many"))
	r := ta.saveList(repeatingForm(rows...))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("51 rows: status %d, want 422", r.Status)
	}
	wantAll(t, r.Body, "at most 50 rows")
	if strings.Contains(ta.get("/repeating").Body, "One too many") {
		t.Error("the 51-row list was saved")
	}
}

func TestBlankRowsDoNotCountAgainstTheCap(t *testing.T) {
	ta := newTestApp(t)
	var rows [][7]string
	for i := 0; i < 50; i++ {
		rows = append(rows, repRow("expense", "1", "Row "+strconv.Itoa(i)))
	}
	rows = append(rows, [7]string{"income", "", "", "", "", "", ""})
	mustSave(t, ta, repeatingForm(rows...))
}

func TestRepeatingOffersOnlyIncomeAndExpense(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/repeating").Body
	wantAll(t, body, `>Income</option>`, `>Expense</option>`)
	for _, label := range []string{"Levy", "Rebate", "Investment", "Divestment"} {
		if strings.Contains(body, ">"+label+"</option>") {
			t.Errorf("%s is offered", label)
		}
	}
	for _, typ := range []string{"levy", "rebate", "investment", "divestment", "bogus"} {
		r := ta.saveList(repeatingForm(repRow(typ, "5", "x")))
		if r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", typ, r.Status)
		}
		wantAll(t, r.Body, "Type must be Expense or Income.")
	}
}

func TestRepeatingAppliesTheHomeFormLimits(t *testing.T) {
	ta := newTestApp(t)
	for _, amount := range []string{"0", "-5", "12,5", "1.5", "abc", "100000001"} {
		r := ta.saveList(repeatingForm(repRow("income", amount, "x")))
		if r.Status != http.StatusUnprocessableEntity {
			t.Errorf("amount %q: status %d, want 422", amount, r.Status)
		}
		wantAll(t, r.Body, "Amount must be a whole number of euros between 1 and 100.000.000.")
	}
	mustSave(t, ta, repeatingForm(repRow("income", "100000000", "x")))

	r := ta.saveList(repeatingForm(repRow("income", "5", strings.Repeat("d", 101))))
	wantAll(t, r.Body, "Description must be at most 100 characters.")
	r = ta.saveList(repeatingForm([7]string{"income", "5", "x", "new", strings.Repeat("n", 51), "new", "ok"}))
	wantAll(t, r.Body, "Category name must be at most 50 characters.")
	r = ta.saveList(repeatingForm([7]string{"income", "5", "x", "new", "ok", "new", strings.Repeat("n", 51)}))
	wantAll(t, r.Body, "Subcategory name must be at most 50 characters.")
	r = ta.saveList(repeatingForm([7]string{"income", "5", "x", "99", "", "99", ""}))
	wantAll(t, r.Body, errIncomplete)
}

func TestRepeatingHostileInputIsEscaped(t *testing.T) {
	ta := newTestApp(t)
	body := mustSave(t, ta, repeatingForm(repRow("income", "5", `<script>alert(1)</script>`))).Body
	if strings.Contains(body, "<script>alert(1)") {
		t.Error("description rendered unescaped")
	}
}

func TestRepeatingCreatesNoTransactionsAndAppearsInNoReport(t *testing.T) {
	ta := newTestApp(t)
	const path = "/report?period=month&date=2026-10"
	before := ta.mustReport(path)
	mustSave(t, ta, repeatingForm(repRow("income", "2000", "Salary"), repRow("expense", "800", "Rent")))
	if after := ta.mustReport(path); after != before {
		t.Errorf("report changed:\n%s", after)
	}
	if body := ta.get("/").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was created:\n%s", body)
	}
}
