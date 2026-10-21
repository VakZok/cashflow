package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

// The Add page: a failed Add is marked data-failed, and coming back with the
// marker re-checks the carried input without saving or creating anything.

func TestAFailedAddMarksItsFormSoLeavingCarriesTheMarker(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("2026-10-05", "5", "", "new", "Food", "new", "Cafe"))
	wantAll(t, r.Body, ` data-carry data-failed="add">`)
	if ok := ta.get("/?type=expense&date=2026-10-05&amount=5&description=Lunch").Body; strings.Contains(ok, "data-failed") {
		t.Error("a form without problems is marked")
	}
}

func TestReturningToAFailedAddShowsTheMissingFieldsAgain(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/?type=expense&date=2026-10-05&amount=5&category=new&new_category=Food&subcategory=new&new_subcategory=Cafe&failed=add").Body
	if got := strings.Join(highlighted(body), ","); got != "description" {
		t.Errorf("highlighted %q, want description", got)
	}
	if b := addButton(t, body); b != "Missing fields" {
		t.Errorf("button %q, want Missing fields", b)
	}
	// Nothing was created by the re-check.
	if strings.Contains(ta.get("/categories").Body, "Food") {
		t.Error("the re-check created a Category")
	}
}

func TestAFieldFixedAfterTheFailedAddIsNotMarkedAgain(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/?type=expense&date=2026-10-05&amount=5&description=Lunch&category=new&new_category=Food&subcategory=new&new_subcategory=Cafe&failed=add").Body
	if h := highlighted(body); len(h) != 0 {
		t.Errorf("highlighted %v", h)
	}
	if b := addButton(t, body); b != "Add" {
		t.Errorf("button %q, want Add", b)
	}
	if strings.Contains(body, "data-failed") {
		t.Error("the marker is carried on without a problem")
	}
}

func TestInputCarriedWithoutAFailedSubmitShowsNoProblems(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/?type=expense&date=2026-10-05&amount=0").Body
	if h := highlighted(body); len(h) != 0 || strings.Contains(body, `class="bubble"`) {
		t.Errorf("problems shown: %v", h)
	}
	if b := addButton(t, body); b != "Add" {
		t.Errorf("button %q", b)
	}
}

func TestReturningToAFailedAddWithAnAmountProblemShowsTheBubble(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/?type=expense&date=2026-10-05&amount=0&description=Lunch&category=new&new_category=Food&subcategory=new&new_subcategory=Cafe&failed=add").Body
	wantAll(t, body, `<span class="bubble" role="alert">1 to 100.000.000</span>`)
	if b := addButton(t, body); b != "Add" {
		t.Errorf("button %q, want Add", b)
	}
}

func TestADuplicateAddComesBackMarkedUntilTheExistingOneIsDeleted(t *testing.T) {
	ta := newTestApp(t)
	first := expenseForm("2026-10-05", "5", "Lunch", "new", "Food", "new", "Cafe")
	ta.post("/transactions", first)
	if r := ta.post("/transactions", first); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("the duplicate got status %d", r.Status)
	}
	again := "/?type=expense&date=2026-10-05&amount=5&description=Lunch&category=1&subcategory=1&failed=add"
	if b := addButton(t, ta.get(again).Body); b != "Already exists" {
		t.Errorf("button %q, want Already exists", b)
	}
	ta.post("/transactions/1/delete", nil)
	if b := addButton(t, ta.get(again).Body); b != "Add" {
		t.Errorf("after deleting the existing one, button %q, want Add", b)
	}
	// Re-checking added nothing.
	if strings.Contains(ta.get("/transactions?year=all").Body, "tx-2") {
		t.Error("the re-check created a Transaction")
	}
}

// The Transactions page: a failed Apply leaves the edit marked, with the
// address to carry the edit into.

func TestAFailedApplyMarksTheEditAndNamesTheAddressToCarryItInto(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	form := expenseForm("2026-10-02", "12", "", "1", "", "1", "")
	form.Set("return", "transactions:1?year=2026&month=10")
	r := ta.post("/transactions/1", form)
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", r.Status)
	}
	wantAll(t, r.Body, ` data-carry="e_" data-address="/transactions?year=2026&amp;month=10&amp;page=1&amp;edit=1" data-failed="apply">`)
}

func TestReturningToAFailedApplyReopensTheEditWithItsProblems(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	body := ta.get("/transactions?year=2026&month=10&page=1&edit=1&e_type=expense&e_date=2026-10-02&e_amount=12&e_category=1&e_subcategory=1&failed=apply").Body
	if got := strings.Join(highlighted(body), ","); got != "edit-description" {
		t.Errorf("highlighted %q, want edit-description", got)
	}
	if b := applyButton(t, body); b != "Missing fields" {
		t.Errorf("button %q, want Missing fields", b)
	}
	// Without the marker the same carried input shows no problems.
	body = ta.get("/transactions?year=2026&month=10&page=1&edit=1&e_type=expense&e_date=2026-10-02&e_amount=12&e_category=1&e_subcategory=1").Body
	if len(highlighted(body)) != 0 || applyButton(t, body) != "Apply" {
		t.Error("problems shown without the marker")
	}
}

func TestAFixedEditAndAnUnchangedOneAreNotMarkedAgain(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	// Fixed: the description is there. The Transaction is not a Duplicate of itself.
	body := ta.get("/transactions?edit=1&e_type=expense&e_date=2026-10-02&e_amount=12&e_description=Lunch&e_category=1&e_subcategory=1&failed=apply").Body
	if len(highlighted(body)) != 0 || applyButton(t, body) != "Apply" || strings.Contains(body, "data-failed") {
		t.Errorf("marked again:\n%s", editRow(body, "1"))
	}
}

func TestReturningToAFailedApplyReflectsADuplicateDeletedMeanwhile(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", expenseForm("2026-10-03", "9", "Dinner", "1", "", "1", ""))
	again := "/transactions?edit=2&e_type=expense&e_date=2026-10-02&e_amount=9&e_description=Lunch&e_category=1&e_subcategory=1&failed=apply"
	if b := applyButton(t, ta.get(again).Body); b != "Already exists" {
		t.Errorf("button %q, want Already exists", b)
	}
	ta.post("/transactions/1/delete", url.Values{})
	if b := applyButton(t, ta.get(again).Body); b != "Apply" {
		t.Errorf("after deleting the other one, button %q, want Apply", b)
	}
	// The re-check applied nothing: the Transaction is as stored.
	wantAll(t, ta.get("/transactions?year=all").Body, "Dinner")
}

// The Edit Categories page: a refused Rename leaves the open rename marked.

func TestARefusedRenameMarksItsFormAndReturningShowsTakenAgain(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	addEntry(ta, "category", "Rent")
	r := ta.post("/categories/2", url.Values{"list": {"category"}, "name": {"food"}})
	body := ta.follow(r).Body
	wantAll(t, openRow(body), `data-carry="e_" data-failed="rename">`, ">Taken</button>")

	// Back from another page the typed name is carried with the marker.
	body = ta.get("/categories?list=category&page=1&edit=2&e_name=FOOD&failed=rename").Body
	wantAll(t, openRow(body), `value="FOOD"`, ">Taken</button>")
	// Nothing was renamed by the re-check.
	wantAll(t, ta.get("/categories").Body, "Rent")
}

func TestARenameFixedAfterBeingRefusedIsNotMarkedAgain(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	addEntry(ta, "category", "Rent")
	row := openRow(ta.get("/categories?list=category&page=1&edit=2&e_name=Home&failed=rename").Body)
	wantAll(t, row, ">Apply</button>")
	if strings.Contains(row, "data-failed") || strings.Contains(row, "Taken") {
		t.Errorf("marked again:\n%s", row)
	}
	// Its own name, in another case, is not taken.
	if row = openRow(ta.get("/categories?list=category&page=1&edit=2&e_name=rent&failed=rename").Body); strings.Contains(row, "Taken") {
		t.Errorf("an entry is taken by itself:\n%s", row)
	}
}

func TestATakenNameInAnotherListIsNotTakenAndNoMarkerShowsNoProblem(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	addEntry(ta, "subcategory", "Cafe")
	if row := openRow(ta.get("/categories?list=subcategory&page=1&edit=1&e_name=Food&failed=rename").Body); strings.Contains(row, "Taken") {
		t.Errorf("another list's name is taken:\n%s", row)
	}
	addEntry(ta, "category", "Rent")
	if row := openRow(ta.get("/categories?list=category&page=1&edit=2&e_name=Food").Body); strings.Contains(row, "Taken") {
		t.Errorf("a problem is shown without the marker:\n%s", row)
	}
}

// The Repeating page: a failed Save List or Add Transactions leaves the form
// marked with the pressed button.

// wantRechecked checks a Repeating page re-checked on return: the labels of both
// buttons, the highlighted inputs and the marked rows.
func wantRechecked(t *testing.T, r response, save, add, inputs string, marked ...int) {
	t.Helper()
	if r.Status != http.StatusOK {
		t.Fatalf("status %d\n%s", r.Status, r.Body)
	}
	if got := saveButton(t, r.Body); got != save {
		t.Errorf("Save List button %q, want %q", got, save)
	}
	if got := addButtonText(t, r.Body); got != add {
		t.Errorf("Add Transactions button %q, want %q", got, add)
	}
	if got := highlightedInputs(r.Body); got != inputs {
		t.Errorf("highlighted %q, want %q", got, inputs)
	}
	if got := markedRows(r.Body); !slices.Equal(got, marked) {
		t.Errorf("marked rows %v, want %v", got, marked)
	}
}

func TestAFailedRepeatingSubmitMarksItsFormWithThePressedButton(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(repRow("expense", "", "Gym"))
	f.Set("action", "save")
	wantAll(t, ta.saveList(f).Body, ` data-carry data-failed="save">`)
	f.Set("action", "add")
	wantAll(t, ta.saveList(f).Body, ` data-carry data-failed="add">`)
	// A date missing at Add Transactions marks it as well.
	g := repeatingForm(repRow("expense", "8", "Rent"))
	g.Set("action", "add")
	g.Set("date", "")
	wantAll(t, ta.saveList(g).Body, ` data-carry data-failed="add">`)
	if body := ta.get("/repeating").Body; strings.Contains(body, "data-failed") {
		t.Error("a form without problems is marked")
	}
}

func TestReturningToAFailedSaveListShowsDuplicatesAndMissingFieldsAgain(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(
		[7]string{"expense", "", "Gym", "new", "Cat", "new", "Sub"},
		repRow("expense", "8", "Rent"), repRow("expense", "9", "Rent"))
	f.Set("date", "2026-10")
	f.Set("failed", "save")
	wantRechecked(t, ta.carried(f), "Duplicate rows and missing fields", "Add Transactions", "amount_0", 2)
	// Nothing was saved or created.
	if rowCount(ta.get("/repeating").Body) != 1 || strings.Contains(ta.get("/categories").Body, "<span>Cat</span>") {
		t.Error("the re-check saved or created something")
	}
	// Without the marker the same input shows no problems.
	f.Del("failed")
	wantRechecked(t, ta.carried(f), "Save List", "Add Transactions", "")
}

func TestReturningToAFailedSaveListKeepsAnAmountBubbleAndFixedRowsClean(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(repRow("expense", "0", "Gym"), repRow("expense", "8", "Rent"))
	f.Set("failed", "save")
	body := ta.carried(f).Body
	wantAll(t, body, `<span class="bubble" role="alert">1 to 100.000.000</span>`)
	// Fixed since: usual labels, and no marker is carried on.
	f = repeatingForm(repRow("expense", "5", "Gym"), repRow("expense", "8", "Rent"))
	f.Set("failed", "save")
	r := ta.carried(f)
	wantRechecked(t, r, "Save List", "Add Transactions", "")
	if strings.Contains(r.Body, "data-failed") {
		t.Error("the marker is carried on without a problem")
	}
}

func TestReturningToAFailedAddTransactionsWithoutAMonthMarksTheMonth(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(repRow("expense", "8", "Rent"))
	f.Set("failed", "add") // the cleared Month is left out of the carried address
	r := ta.carried(f)
	wantRechecked(t, r, "Save List", "Missing fields", "date")
	// A Month filled in since: usual labels.
	f.Set("date", "2026-10")
	wantRechecked(t, ta.carried(f), "Save List", "Add Transactions", "")
	// Nothing was booked.
	if strings.Contains(ta.get("/transactions?year=all").Body, "Rent") {
		t.Error("the re-check booked a Transaction")
	}
}

func TestReturningToAFailedAddTransactionsShowsTheListsProblemsOnThatButton(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(repRow("expense", "8", "Rent"), repRow("expense", "9", "Rent"))
	f.Set("date", "2026-10")
	f.Set("failed", "add")
	wantRechecked(t, ta.carried(f), "Save List", "Duplicate rows", "", 1)
	// An empty list.
	e := url.Values{"row": {"0"}, "type_0": {"expense"}, "date": {"2026-10"}, "failed": {"add"}}
	wantRechecked(t, ta.carried(e), "Save List", "List is empty", "")
}
