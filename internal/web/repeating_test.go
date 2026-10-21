package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// listButton is the label of the Repeating page's button whose usual label is resting.
func listButton(t *testing.T, body, resting string) string {
	t.Helper()
	m := regexp.MustCompile(`data-resting="` + resting + `"[^>]*>([^<]*)</button>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no %s button in:\n%s", resting, body)
	}
	return m[1]
}

func saveButton(t *testing.T, body string) string {
	t.Helper()
	return listButton(t, body, "Save List")
}

func addButtonText(t *testing.T, body string) string {
	t.Helper()
	return listButton(t, body, "Add Transactions")
}

// highlightedInputs lists the names of the Repeating page's inputs that carry the red highlight.
func highlightedInputs(body string) string {
	var names []string
	for _, m := range regexp.MustCompile(`name="([^"]*)" class="invalid"|id="(date)" class="invalid"`).FindAllStringSubmatch(body, -1) {
		names = append(names, m[1]+m[2])
	}
	return strings.Join(names, ",")
}

// markedRows lists the indexes of the rendered rows marked as duplicates.
func markedRows(body string) []int {
	var marked []int
	for i, m := range regexp.MustCompile(`class="rep-row( duplicate)?"`).FindAllStringSubmatch(body, -1) {
		if m[1] != "" {
			marked = append(marked, i)
		}
	}
	return marked
}

// wantRejected checks a rejected Save List or Add Transactions: status 422,
// the pressed button's label, and the highlighted inputs and marked rows.
func wantRejected(t *testing.T, r response, action, button, inputs string, marked ...int) {
	t.Helper()
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422\n%s", r.Status, r.Body)
	}
	pressed, other := saveButton(t, r.Body), addButtonText(t, r.Body)
	if action == "add" {
		pressed, other = other, pressed
	}
	if pressed != button {
		t.Errorf("button %q, want %q", pressed, button)
	}
	if other != "Save List" && other != "Add Transactions" {
		t.Errorf("the other button reads %q", other)
	}
	if got := highlightedInputs(r.Body); got != inputs {
		t.Errorf("highlighted %q, want %q", got, inputs)
	}
	if got := markedRows(r.Body); !slices.Equal(got, marked) {
		t.Errorf("marked rows %v, want %v", got, marked)
	}
}

// repeatingForm builds a Save List submission; each row is
// {type, amount, description, category, newCategory, subcategory, newSubcategory}.
func repeatingForm(rows ...[7]string) url.Values {
	f := url.Values{}
	for i, r := range rows {
		n := strconv.Itoa(i)
		f.Add("row", n)
		names := []string{"type", "amount", "description", "category", "new_category", "subcategory", "new_subcategory"}
		if r[0] == "levy" || r[0] == "rebate" {
			names = []string{"type", "amount", "description", "levy_category", "new_levy_category", "levy_subcategory", "new_levy_subcategory"}
		}
		if r[0] == "investment" || r[0] == "divestment" {
			names = []string{"type", "amount", "description", "holding", "new_holding", "", ""}
		}
		for j, name := range names {
			if name == "" {
				continue
			}
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
	wantAll(t, ta.get("/").Body, `href="/repeating"`, `aria-label="Repeating"`)
	r := ta.get("/repeating")
	if r.Status != http.StatusOK {
		t.Fatalf("status %d", r.Status)
	}
	wantAll(t, r.Body, `href="/"`, `href="/report"`, "Save List")
	if n := rowCount(r.Body); n != 1 {
		t.Errorf("%d rows, want 1", n)
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
	if b := saveButton(t, body); b != "List saved!" || !strings.Contains(body, `data-resting="Save List" data-flash>`) {
		t.Errorf("Save List button %q, want the List saved! success text", b)
	}
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
		"amount_1":                    {"income", "", "Salary", "new", "Newcat", "new", "Newsub"},
		"description_1":               {"income", "5", "", "new", "Newcat", "new", "Newsub"},
		"category_1":                  {"income", "5", "Salary", "", "", "new", "Newsub"},
		"subcategory_1":               {"income", "5", "Salary", "new", "Newcat", "", ""},
		"new_category_1":              {"income", "5", "Salary", "new", "", "new", "Newsub"},
		"description_1,subcategory_1": {"income", "5", "", "new", "Newcat", "", ""},
	} {
		r := ta.saveList(repeatingForm(repRow("expense", "9", "Fine"), row))
		wantRejected(t, r, "save", "Missing fields", name)
		wantAll(t, r.Body, `value="Fine"`, `value="9"`)
		if !strings.HasPrefix(name, "description") {
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

func TestTheRepeatingPageHasNoErrorOrNoticeBox(t *testing.T) {
	ta := newTestApp(t)
	bodies := []string{
		ta.saveList(repeatingForm([7]string{"income", "", "x", "", "", "", ""})).Body,
		ta.saveList(repeatingForm(repRow("expense", "8", "Rent"), repRow("expense", "9", "Rent"))).Body,
		mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent"))).Body,
		ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent")).Body,
		ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent")).Body,
	}
	for _, body := range bodies {
		for _, old := range []string{`class="errors"`, `class="notice"`, "must be", "in the list:", "added for"} {
			if strings.Contains(body, old) {
				t.Errorf("page contains %q:\n%s", old, body)
			}
		}
	}
}

func TestTheRepeatingPageIsTitledRepeatingTransactions(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/repeating").Body, "<title>Repeating Transactions</title>", "<h1>Repeating</h1>")
}

func TestSaveListKeepsTheScrollPosition(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/repeating").Body, `<form method="post" action="/repeating" data-keep-scroll data-carry>`)
}

func TestTwoRowsWithTheSameTypeAndDescriptionMarkOnlyTheSecond(t *testing.T) {
	for _, action := range []string{"save", "add"} {
		ta := newTestApp(t)
		f := addForm("2026-10-01", repRow("expense", "800", "Rent"), repRow("expense", "900", "Rent"))
		f.Set("action", action)
		wantRejected(t, ta.saveList(f), action, "Duplicate rows", "", 1)
	}
}

func TestEveryRowAfterTheFirstOfEachCollidingSetIsMarked(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm(repRow("expense", "1", "Rent"), repRow("expense", "2", "Gym"), repRow("income", "3", "Rent"),
		repRow("expense", "4", "rent"), repRow("expense", "5", "GYM"), repRow("expense", "6", "Rent")))
	wantRejected(t, r, "save", "Duplicate rows", "", 3, 4, 5)
}

func TestMissingFieldsAndDuplicatesAreShownTogether(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm(
		[7]string{"expense", "5", "Gym", "new", "Cat", "", ""},
		repRow("expense", "8", "Rent"), repRow("expense", "9", "Rent")))
	wantRejected(t, r, "save", "Duplicate rows and missing fields", "subcategory_0", 2)
	// A row missing what its rule compares collides with none.
	r = ta.saveList(repeatingForm(repRow("expense", "8", ""), repRow("expense", "9", ""), holdRow("investment", "5", "", ""), holdRow("divestment", "6", "", "")))
	wantRejected(t, r, "save", "Missing fields", "description_0,description_1,holding_2,holding_3")
}

func TestSavingAgainHighlightsWhatIsStillWrong(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm([7]string{"expense", "", "Gym", "new", "", "new", "Sub"}, repRow("expense", "8", "Rent"), repRow("expense", "9", "Rent")))
	wantRejected(t, r, "save", "Duplicate rows and missing fields", "amount_0,new_category_0", 2)
	// The amount is filled in and the duplicate removed; the new name is still missing.
	r = ta.saveList(repeatingForm([7]string{"expense", "5", "Gym", "new", "", "new", "Sub"}, repRow("expense", "8", "Rent")))
	wantRejected(t, r, "save", "Missing fields", "new_category_0")
}

func TestABlankRowIsNeitherHighlightedNorMarked(t *testing.T) {
	ta := newTestApp(t)
	blank := [7]string{"expense", "", "", "", "", "", ""}
	r := ta.saveList(repeatingForm(blank, [7]string{"expense", "5", "", "new", "Cat", "new", "Sub"}, blank))
	wantRejected(t, r, "save", "Missing fields", "description_1")
}

func TestAnInvalidAmountShowsTheBubbleOnItsRowOnly(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm(repRow("expense", "8", "Rent"), repRow("expense", "0", "Gym")))
	wantRejected(t, r, "save", "Save List", "")
	rows := strings.Split(r.Body, `class="rep-row"`)
	if strings.Contains(rows[1], `class="bubble"`) || !strings.Contains(rows[2], `<span class="bubble" role="alert">1 to 100.000.000</span>`) {
		t.Errorf("bubble not on the second row only:\n%s", r.Body)
	}
	if strings.Contains(ta.get("/repeating").Body, `class="bubble"`) {
		t.Error("a fresh page shows the bubble")
	}
}

func TestTheSuccessTextIsGoneOnTheNextLoad(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent")))
	body := ta.get("/repeating").Body
	if saveButton(t, body) != "Save List" || strings.Contains(body, " data-flash>") {
		t.Errorf("reload shows a success text:\n%s", body)
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
	wantRejected(t, ta.saveList(repeatingForm(rows...)), "save", "Invalid input", "")
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

func TestRepeatingAppliesTheHomeFormLimits(t *testing.T) {
	ta := newTestApp(t)
	for _, amount := range []string{"0", "-5", "12,5", "1.5", "abc", "100000001"} {
		r := ta.saveList(repeatingForm(repRow("income", amount, "x")))
		wantRejected(t, r, "save", "Save List", "")
		wantAll(t, r.Body, `<span class="bubble" role="alert">1 to 100.000.000</span>`)
	}
	mustSave(t, ta, repeatingForm(repRow("income", "100000000", "x")))

	// Only a hand-crafted request gets past the inputs' maxlength or picks an id the selects never offer.
	for _, row := range [][7]string{
		repRow("income", "5", strings.Repeat("d", 101)),
		{"income", "5", "x", "new", strings.Repeat("n", 51), "new", "ok"},
		{"income", "5", "x", "new", "ok", "new", strings.Repeat("n", 51)},
		{"income", "5", "x", "99", "", "99", ""},
	} {
		wantRejected(t, ta.saveList(repeatingForm(row)), "save", "Invalid input", "")
	}
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
	if body := ta.get("/transactions").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was created:\n%s", body)
	}
}

func TestSavingRejectsRowsWithTheSameTypeAndDescriptionIgnoringCase(t *testing.T) {
	for _, typ := range []string{"income", "expense"} {
		for _, action := range []string{"save", "add"} {
			ta := newTestApp(t)
			f := addForm("2026-10-01", repRow(typ, "800", "Rent"), repRow(typ, "30", "Gym"), repRow(typ, "5", "rENT"), repRow(typ, "6", "gym"))
			f.Set("action", action)
			r := ta.saveList(f)
			wantRejected(t, r, action, "Duplicate rows", "", 2, 3)
			wantAll(t, r.Body, `value="rENT"`, `value="gym"`)
			if n := rowCount(ta.get("/repeating").Body); n != 1 {
				t.Errorf("%d rows saved, want the untouched blank list", n)
			}
			if strings.Contains(ta.get("/transactions").Body, "Rent") {
				t.Errorf("a Transaction was booked")
			}
		}
	}
}

func TestSavingAcceptsTheSameDescriptionUnderDifferentTypes(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("income", "5", "Refund"), repRow("expense", "5", "Refund")))
}

func TestInListDuplicateCreatesNoNewNames(t *testing.T) {
	ta := newTestApp(t)
	ta.saveList(repeatingForm(repRow("expense", "8", "Rent"), repRow("expense", "9", "Rent")))
	if strings.Contains(ta.get("/").Body, ">Cat<") {
		t.Errorf("a new Category was created by a rejected save")
	}
}

func TestAnIncompleteRowCanStillBeADuplicate(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm(repRow("expense", "8", "Rent"), [7]string{"expense", "", "Rent", "", "", "new", "Sub"}))
	wantRejected(t, r, "save", "Duplicate rows and missing fields", "amount_1,category_1", 1)
}

func TestMissingFieldsOutrankAHandCraftedProblemOnTheButton(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm(repRow("bogus", "5", "x"), repRow("expense", "", "Rent")))
	wantRejected(t, r, "save", "Missing fields", "amount_1")
	r = ta.saveList(repeatingForm(repRow("bogus", "5", "x"), repRow("expense", "8", "Rent"), repRow("expense", "9", "Rent")))
	wantRejected(t, r, "save", "Invalid input", "", 2)
}

func TestRepeatingOffersEveryTypeAndRejectsUnknownOnes(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/repeating").Body, `>Income</option>`, `>Expense</option>`, `>Levy</option>`, `>Rebate</option>`,
		`>Investment</option>`, `>Divestment</option>`)
	wantRejected(t, ta.saveList(repeatingForm(repRow("bogus", "5", "x"))), "save", "Invalid input", "")
}

// holdRow is an Investment or Divestment row picking or creating the Holding.
func holdRow(typ, amount, holding, newName string) [7]string {
	return [7]string{typ, amount, "", holding, newName, "", ""}
}

func TestRepeatingRowShowsOnlyAmountAndHoldingForInvestmentAndDivestment(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(holdRow("investment", "50", "new", "ETF"), repRow("expense", "8", "Rent")))
	body := ta.get("/repeating").Body
	wantAll(t, body, `name="holding_1"`, `name="new_holding_1"`, `data-holding`)
	rows := strings.Split(body, `class="rep-row"`)
	wantAll(t, rows[2], `<option value="1" selected>ETF</option>`, `name="description_1" maxlength="100" placeholder="Description" aria-label="Description" value="" hidden disabled`,
		`class="lists-default" hidden`, `class="lists-levy" hidden`)
	if strings.Contains(rows[2], `class="lists-holding" hidden`) {
		t.Errorf("Investment row hides its Holding:\n%s", rows[2])
	}
	wantAll(t, rows[1], `class="lists-holding" hidden`)
	if strings.Contains(rows[1], `class="lists-default" hidden`) || strings.Contains(rows[1], `description_0" maxlength="100" placeholder="Description" aria-label="Description" value="Rent" hidden`) {
		t.Errorf("Expense row hides its own fields:\n%s", rows[1])
	}
}

func TestSavedInvestmentAndDivestmentRowsComeAfterIncomeAndExpenseBeforeLevyAndRebate(t *testing.T) {
	ta := newTestApp(t)
	body := mustSave(t, ta, repeatingForm(
		repRow("rebate", "1", "RebateA"), holdRow("divestment", "2", "new", "HoldD1"), repRow("levy", "3", "LevyA"),
		holdRow("investment", "4", "new", "HoldI2"), repRow("expense", "5", "ExpenseA"),
		holdRow("investment", "6", "new", "HoldI1"), repRow("income", "7", "IncomeA"))).Body
	selected := regexp.MustCompile(`value="(IncomeA|ExpenseA|LevyA|RebateA)"|<option value="\d+" selected>(Hold\w+)</option>`)
	var got []string
	for _, row := range strings.Split(body, `class="rep-row"`)[1:] {
		// A Holding option is only selected on a Holding row; the others show their description.
		for _, m := range selected.FindAllStringSubmatch(row, -1) {
			if m[1] != "" || strings.HasPrefix(m[2], "Hold") {
				got = append(got, m[1]+m[2])
				break
			}
		}
	}
	want := []string{"IncomeA", "ExpenseA", "HoldI2", "HoldI1", "HoldD1", "LevyA", "RebateA"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order %v, want %v", got, want)
	}
}

func TestIncompleteInvestmentAndDivestmentRowsBlockSaveAndBooking(t *testing.T) {
	for _, typ := range []string{"investment", "divestment"} {
		for _, action := range []string{"save", "add"} {
			for _, c := range []struct {
				row            [7]string
				button, inputs string
			}{
				{holdRow(typ, "", "new", "ETF"), "Missing fields", "amount_0"},
				{holdRow(typ, "5", "", ""), "Missing fields", "holding_0"},
				{holdRow(typ, "5", "new", ""), "Missing fields", "new_holding_0"},
				{holdRow(typ, "5", "999", ""), "Invalid input", ""},
			} {
				row := c.row
				ta := newTestApp(t)
				f := addForm("2026-10-01", row)
				f.Set("action", action)
				wantRejected(t, ta.saveList(f), action, c.button, c.inputs)
				if strings.Contains(ta.get("/").Body, "ETF") || strings.Contains(ta.get("/repeating").Body, ">ETF</option>") {
					t.Errorf("%s/%s %v: something was saved or created", typ, action, row)
				}
			}
		}
	}
}

func TestInvestmentRowIgnoresADescriptionAndTheOtherListsFields(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(holdRow("investment", "5", "new", "ETF"))
	f.Set("description_0", "Stray")
	f.Set("category_0", "new")
	f.Set("new_category_0", "StrayCat")
	body := mustSave(t, ta, f).Body
	if strings.Contains(body, "Stray") {
		t.Errorf("a description or Category was kept on an Investment row:\n%s", body)
	}
}

func TestHoldingNameLimitAppliesToInvestmentRows(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm(holdRow("divestment", "5", "new", strings.Repeat("n", 51))))
	wantRejected(t, r, "save", "Invalid input", "")
}

func TestSavingRejectsTwoRowsWithTheSameHoldingWhateverTheirType(t *testing.T) {
	for _, types := range [][2]string{{"investment", "investment"}, {"divestment", "divestment"}, {"investment", "divestment"}} {
		ta := newTestApp(t)
		r := ta.saveList(repeatingForm(holdRow(types[0], "5", "new", "ETF"), holdRow(types[1], "6", "new", "etf")))
		wantRejected(t, r, "save", "Duplicate rows", "", 1)
		if strings.Contains(ta.get("/repeating").Body, ">ETF</option>") {
			t.Errorf("%v: a Holding was created by a rejected save", types)
		}
	}
}

func TestSavingRejectsAnExistingHoldingPickedTwiceOrAlsoEnteredAsNew(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(holdRow("investment", "5", "new", "ETF")))
	r := ta.saveList(repeatingForm(holdRow("investment", "5", "1", ""), holdRow("divestment", "6", "1", "")))
	wantRejected(t, r, "save", "Duplicate rows", "", 1)
	r = ta.saveList(repeatingForm(holdRow("investment", "5", "1", ""), holdRow("divestment", "6", "new", "Etf")))
	wantRejected(t, r, "save", "Duplicate rows", "", 1)
}

func TestSavingAcceptsDifferentHoldingsAndTheSameNameAsADescription(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(holdRow("investment", "5", "new", "ETF"), holdRow("divestment", "6", "new", "Bonds"), repRow("expense", "5", "ETF")))
}

func TestRepeatingRowShowsLevyListsForLevyAndRebateOnly(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("levy", "250", "Income tax"), repRow("expense", "8", "Rent")))
	body := ta.get("/repeating").Body
	wantAll(t, body, `name="levy_category_0"`, `name="levy_subcategory_0"`, `name="category_1"`, `data-levy-lists`)
	// The levy lists of the Expense row and the default lists of the Levy row are not submitted.
	rows := strings.Split(body, `class="rep-row"`)
	if strings.Contains(rows[1], `class="lists-default" hidden`) || !strings.Contains(rows[1], `class="lists-levy" hidden`) {
		t.Errorf("Expense row shows the wrong lists:\n%s", rows[1])
	}
	if !strings.Contains(rows[2], `class="lists-default" hidden`) || strings.Contains(rows[2], `class="lists-levy" hidden`) {
		t.Errorf("Levy row shows the wrong lists:\n%s", rows[2])
	}
}

func TestSavedLevyAndRebateRowsComeAfterTheOtherTypesLevyBeforeRebate(t *testing.T) {
	ta := newTestApp(t)
	body := mustSave(t, ta, repeatingForm(
		repRow("rebate", "1", "RebateA"), repRow("levy", "2", "LevyA"), repRow("expense", "3", "ExpenseA"),
		repRow("rebate", "4", "RebateB"), repRow("income", "5", "IncomeA"), repRow("levy", "6", "LevyB"))).Body
	last := -1
	for _, d := range []string{"IncomeA", "ExpenseA", "LevyA", "LevyB", "RebateA", "RebateB"} {
		i := strings.Index(body, `value="`+d+`"`)
		if i < 0 || i < last {
			t.Fatalf("%s at %d after %d", d, i, last)
		}
		last = i
	}
	wantAll(t, body, ">Cat</option>", ">Sub</option>")
}

func TestIncompleteLevyAndRebateRowsBlockSaveAndBooking(t *testing.T) {
	for _, typ := range []string{"levy", "rebate"} {
		for _, action := range []string{"save", "add"} {
			for row, input := range map[[7]string]string{
				{typ, "", "Tax", "new", "Tax", "new", "Sub"}: "amount_0",
				{typ, "5", "", "new", "Tax", "new", "Sub"}:   "description_0",
				{typ, "5", "Tax", "", "", "new", "Sub"}:      "levy_category_0",
				{typ, "5", "Tax", "new", "Tax", "", ""}:      "levy_subcategory_0",
				{typ, "5", "Tax", "new", "", "new", "Sub"}:   "new_levy_category_0",
			} {
				ta := newTestApp(t)
				f := addForm("2026-10-01", row)
				f.Set("action", action)
				wantRejected(t, ta.saveList(f), action, "Missing fields", input)
				if strings.Contains(ta.get("/").Body, "Tax") || strings.Contains(ta.get("/repeating").Body, ">Tax</option>") {
					t.Errorf("%s/%s %v: something was saved or created", typ, action, row)
				}
			}
		}
	}
}

func TestLevyRowIgnoresTheDefaultListsAndUsesItsOwn(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(repRow("levy", "5", "Tax"))
	f.Set("category_0", "new")
	f.Set("new_category_0", "Stray")
	// A default-list value on a Levy row is not a Category of the Levy.
	mustSave(t, ta, f)
	if strings.Contains(ta.get("/repeating").Body, "Stray") {
		t.Errorf("a Category was created from a Levy row")
	}
}

func TestLevyAndRebateRowsApplyTheListNameLimit(t *testing.T) {
	ta := newTestApp(t)
	r := ta.saveList(repeatingForm([7]string{"levy", "5", "x", "new", strings.Repeat("n", 51), "new", "ok"}))
	wantRejected(t, r, "save", "Invalid input", "")
	r = ta.saveList(repeatingForm([7]string{"rebate", "5", "x", "new", "ok", "new", strings.Repeat("n", 51)}))
	wantRejected(t, r, "save", "Invalid input", "")
}

func TestSavingRejectsTwoLevyOrTwoRebateRowsWithTheSameDescriptionButAllowsLevyAndRebate(t *testing.T) {
	for _, typ := range []string{"levy", "rebate"} {
		ta := newTestApp(t)
		r := ta.saveList(repeatingForm(repRow(typ, "5", "Tax"), repRow(typ, "6", "tAX")))
		wantRejected(t, r, "save", "Duplicate rows", "", 1)
		if strings.Contains(ta.get("/repeating").Body, ">Cat</option>") {
			t.Errorf("%s: a Levy Category was created by a rejected save", typ)
		}
	}
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("levy", "5", "Tax"), repRow("rebate", "5", "Tax"), repRow("income", "5", "Tax")))
}
