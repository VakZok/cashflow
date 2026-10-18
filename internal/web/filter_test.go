package web_test

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// addDated records an Expense per date, described by its date.
func addDated(ta *testApp, dates ...string) {
	ta.t.Helper()
	for _, d := range dates {
		if r := ta.post("/transactions", expenseForm(d, "3", "On "+d, "new", "Food", "new", "Cafe")); r.Status != 303 {
			ta.t.Fatalf("add %s: status %d", d, r.Status)
		}
	}
}

// selectHTML is the markup of the select with the name, "" if there is none.
func selectHTML(body, name string) string {
	return regexp.MustCompile(`(?s)<select[^>]*name="` + name + `".*?</select>`).FindString(body)
}

// wantNone fails for each unwanted substring found in body.
func wantNone(t *testing.T, body string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		if strings.Contains(body, u) {
			t.Errorf("unexpected %q in:\n%s", u, body)
		}
	}
}

var boundaryDates = []string{"2025-12-31", "2026-01-01", "2026-02-28", "2026-03-01", "2026-12-31", "2027-01-01"}

func TestChoosingAYearShowsOnlyTransactionsOfThatYear(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, boundaryDates...)
	body := ta.get("/transactions?year=2026").Body
	wantAll(t, body, "On 2026-01-01", "On 2026-02-28", "On 2026-03-01", "On 2026-12-31", "1 / 1")
	wantNone(t, body, "On 2025-12-31", "On 2027-01-01")
}

func TestChoosingAMonthShowsOnlyThatCalendarMonthOfTheYear(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, boundaryDates...)
	addDated(ta, "2025-02-15")
	body := ta.get("/transactions?year=2026&month=2").Body
	wantAll(t, body, "On 2026-02-28")
	wantNone(t, body, "On 2026-01-01", "On 2026-03-01", "On 2025-02-15")

	body = ta.get("/transactions?year=2026&month=12").Body
	wantAll(t, body, "On 2026-12-31")
	wantNone(t, body, "On 2027-01-01", "On 2026-03-01")
}

func TestTheYearDropdownListsOnlyYearsWithTransactions(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, "2024-05-05", "2026-01-01", "2026-07-07")
	body := ta.get("/transactions").Body
	years := selectHTML(body, "year")
	wantAll(t, years, `<option value="">Year</option>`, `<option value="2026">2026</option>`, `<option value="2024">2024</option>`)
	wantNone(t, years, "2025", "selected")
	if strings.Index(years, "Year") > strings.Index(years, "2026") || strings.Index(years, "2026") > strings.Index(years, "2024") {
		t.Errorf("years are not All, then newest first:\n%s", years)
	}
	if selectHTML(body, "month") != "" {
		t.Error("Month dropdown shown while Year is All")
	}
}

func TestTheMonthDropdownAppearsOnceAYearIsChosen(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, "2026-01-01")
	body := ta.get("/transactions?year=2026").Body
	wantAll(t, selectHTML(body, "year"), `<option value="2026" selected>2026</option>`)
	months := selectHTML(body, "month")
	wantAll(t, months, `<option value="">Month</option>`, `<option value="1">Jan</option>`, `<option value="12">Dec</option>`)
	wantNone(t, months, "selected")
	if strings.Count(months, "<option") != 13 {
		t.Errorf("want All and 12 months:\n%s", months)
	}
	wantAll(t, selectHTML(ta.get("/transactions?year=2026&month=3").Body, "month"), `<option value="3" selected>Mar</option>`)
}

func TestTheFilterControlsSubmitOnChangeWithoutAnApplyButton(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, "2026-01-01")
	body := ta.get("/transactions?year=2026").Body
	form := regexp.MustCompile(`(?s)<form[^>]*class="filters[^"]*".*?</form>`).FindString(body)
	wantAll(t, form, `method="get"`, `action="/transactions"`)
	wantAll(t, selectHTML(form, "year"), `onchange="this.form.submit()"`)
	wantAll(t, selectHTML(form, "month"), `onchange="this.form.submit()"`)
	wantNone(t, form, "<button", `type="submit"`, `name="page"`)
}

// addMonths records n Expenses in each of 2026-08 and 2026-09; Expense i of a
// month is dated on day i, described "Aug 01", "Sep 01", ...
func addMonths(ta *testApp, n int) {
	ta.t.Helper()
	for _, m := range []string{"08", "09"} {
		for i := 1; i <= n; i++ {
			ta.post("/transactions", expenseForm(fmt.Sprintf("2026-%s-%02d", m, i), "3", fmt.Sprintf("%s %02d", monthName[m], i), "new", "Food", "new", "Cafe"))
		}
	}
}

var monthName = map[string]string{"08": "Aug", "09": "Sep"}

func TestThePagerCountsAndKeepsTheFilter(t *testing.T) {
	ta := newTestApp(t)
	addMonths(ta, 7)
	addDated(ta, "2025-01-01")
	p1 := ta.get("/transactions?year=2026&month=8").Body
	wantAll(t, p1, "Aug 07", "Aug 03", "1 / 2", `href="/transactions?year=2026&amp;month=8&amp;page=2" aria-label="Older"`,
		`href="/transactions?year=2026&amp;month=8&amp;page=1&amp;edit=7#tx-7"`)
	wantNone(t, p1, "Sep ")
	p2 := ta.get("/transactions?year=2026&month=8&page=2").Body
	wantAll(t, p2, "Aug 02", "Aug 01", "2 / 2", `href="/transactions?year=2026&amp;month=8&amp;page=1" aria-label="Newer"`)
	wantAll(t, ta.get("/transactions?year=2026").Body, "1 / 3", `href="/transactions?year=2026&amp;page=2" aria-label="Older"`)
	wantAll(t, ta.get("/transactions?year=2026&month=8&page=9").Body, "2 / 2")
}

func TestClearIsTextOnlyAndRow1IsInThirdsInBothStates(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, "2026-01-01")
	clear := regexp.MustCompile(`(?s)<a class="clear".*?</a>`)
	for _, path := range []string{"/transactions", "/transactions?year=2026&month=1"} {
		got := clear.FindString(ta.get(path).Body)
		wantAll(t, got, `href="/transactions"`, ">Clear</a>")
		wantNone(t, got, "<svg")
	}
	wantAll(t, ta.get("/transactions").Body, `class="filters"`, "grid-template-columns: repeat(3, minmax(0, 1fr))", ".filters:not(.has-year) select.year { grid-column: span 2; }")
	wantAll(t, ta.get("/transactions?year=2026").Body, `class="filters has-year"`)
}

func TestNoFilterDropdownOffersAll(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, "2026-01-01")
	body := ta.get("/transactions?year=2026").Body
	wantAll(t, body, `<option value="">Year</option>`, `<option value="">Month</option>`, `<option value="">Type</option>`)
	wantNone(t, body, `<option value="">All</option>`)
}

func TestAnEmptyFilteredResultIsNotAnEmptyDatabase(t *testing.T) {
	ta := newTestApp(t)
	empty := ta.get("/transactions").Body
	wantAll(t, empty, "No Transactions yet.")
	wantNone(t, empty, `class="filters`, "No matching Transactions.")

	addDated(ta, "2026-01-01")
	r := ta.get("/transactions?year=2026&month=2")
	wantAll(t, r.Body, "No matching Transactions.", `name="year"`, `name="month"`)
	wantNone(t, r.Body, "No Transactions yet.", `aria-label="Newer"`)
}

func TestEditFromAFilteredPageCarriesTheFilter(t *testing.T) {
	ta := newTestApp(t)
	addMonths(ta, 7)
	row := editRow(ta.get("/transactions?year=2026&month=8&page=2&edit=2").Body, "2")
	wantAll(t, row, `name="return" value="transactions:2?year=2026&amp;month=8"`, `href="/transactions?year=2026&amp;month=8&amp;page=2"`)
}

func TestChangeOnAFilteredPageFollowsTheRowWithinTheFilter(t *testing.T) {
	ta := newTestApp(t)
	addMonths(ta, 7)
	// Aug 07 (id 7) moved to Aug 01's day sits last in August: page 2 of the filter.
	r := ta.post("/transactions/7", changeForm(ta, "2026-08-01", "Aug 07", "transactions:1?year=2026&month=8"))
	if r.Status != 303 || r.Location != "/transactions?year=2026&month=8&page=2#tx-7" {
		t.Fatalf("status %d, location %q", r.Status, r.Location)
	}
}

func TestATransactionEditedOutOfTheFilterDisappearsFromThePage(t *testing.T) {
	ta := newTestApp(t)
	addMonths(ta, 6)
	// Aug 01 (id 1), alone on page 2 of August, moves to October: page 2 is past the end.
	r := ta.post("/transactions/1", changeForm(ta, "2026-10-01", "Aug 01", "transactions:2?year=2026&month=8"))
	if r.Status != 303 || r.Location != "/transactions?year=2026&month=8&page=1" {
		t.Fatalf("status %d, location %q", r.Status, r.Location)
	}
	body := ta.follow(r).Body
	wantAll(t, body, "Aug 06", "1 / 1")
	wantNone(t, body, "Aug 01")
}

func TestDeleteFromAFilteredPageReturnsToItClamped(t *testing.T) {
	ta := newTestApp(t)
	addMonths(ta, 6)
	r := ta.post("/transactions/2/delete", url.Values{"return": {"transactions:1?year=2026&month=8"}})
	if r.Location != "/transactions?year=2026&month=8&page=1" {
		t.Errorf("location %q", r.Location)
	}
	// The only row of page 2 of August: back to the new last page of August.
	r = ta.post("/transactions/1/delete", url.Values{"return": {"transactions:2?year=2026&month=8"}})
	if r.Location != "/transactions?year=2026&month=8&page=1" {
		t.Fatalf("location %q", r.Location)
	}
	wantAll(t, ta.follow(r).Body, "1 / 1", "Aug 06")
}

func TestATamperedFilterInTheReturnValueNarrowsNothing(t *testing.T) {
	ta := newTestApp(t)
	addMonths(ta, 6)
	for i, c := range []struct{ ret, want string }{
		{"transactions:1?year=evil", "/transactions?page=1"},
		{"transactions:1?year=2026%26page%3D9", "/transactions?page=1"},
		{"transactions:1?month=8", "/transactions?page=1"},
		{"transactions:1?%zz", "/transactions?page=1"},
		{"transactions:1?year=2026&month=//evil.example", "/transactions?year=2026&page=1"},
	} {
		if r := ta.post(fmt.Sprintf("/transactions/%d/delete", 12-i), url.Values{"return": {c.ret}}); r.Location != c.want {
			t.Errorf("%q: status %d, location %q, want %q", c.ret, r.Status, r.Location, c.want)
		}
	}
}

func TestInvalidYearOrMonthFallsBackToAll(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, "2025-06-01", "2026-06-01")
	for _, q := range []string{"year=abc", "year=2030", "year=1999", "year=+2026", "year=2026x", "year=02026", "month=6"} {
		r := ta.get("/transactions?" + q)
		if r.Status != 200 {
			t.Errorf("%s: status %d", q, r.Status)
		}
		wantAll(t, r.Body, "On 2025-06-01", "On 2026-06-01")
		if selectHTML(r.Body, "month") != "" {
			t.Errorf("%s: Month dropdown shown", q)
		}
	}
	for _, q := range []string{"year=2026&month=0", "year=2026&month=13", "year=2026&month=x", "year=2026&month=-1"} {
		body := ta.get("/transactions?" + q).Body
		wantAll(t, body, "On 2026-06-01")
		wantNone(t, body, "On 2025-06-01")
		wantNone(t, selectHTML(body, "month"), "selected")
	}
}

// typeFilterApp holds one Transaction of every Type, described "Expense"... "Divestment".
func typeFilterApp(t *testing.T) *testApp {
	t.Helper()
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-01-01", "3", "Expense", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", url.Values{"type": {"income"}, "date": {"2026-01-02"}, "amount": {"3"}, "description": {"Income"},
		"category": {"new"}, "new_category": {"Pay"}, "subcategory": {"new"}, "new_subcategory": {"Job"}})
	ta.post("/transactions", levyForm("2026-01-03", "3", "Levy", "new", "Tax", "new", "Federal"))
	ta.post("/transactions", rebateForm("2026-01-04", "3", "Rebate", "new", "Refund", "new", "Federal"))
	ta.post("/transactions", holdingForm("investment", "2026-01-05", "3", "new", "Investment"))
	ta.post("/transactions", holdingForm("divestment", "2026-01-06", "3", "new", "Divestment"))
	return ta
}

var allTypeDescriptions = []string{"Expense", "Income", "Levy", "Rebate", "Investment", "Divestment"}

func TestChoosingATypeShowsOnlyTransactionsOfThatType(t *testing.T) {
	ta := typeFilterApp(t)
	for i, typ := range []string{"expense", "income", "levy", "rebate", "investment", "divestment"} {
		body := ta.get("/transactions?type=" + typ).Body
		wantAll(t, body, "<span>"+allTypeDescriptions[i]+"</span>")
		for j, other := range allTypeDescriptions {
			if j != i {
				wantNone(t, body, "<span>"+other+"</span>")
			}
		}
	}
	ta.post("/transactions", expenseForm("2025-01-01", "3", "Old", "new", "Food", "new", "Cafe"))
	body := ta.get("/transactions?type=expense&year=2026").Body
	wantAll(t, body, "<span>Expense</span>")
	wantNone(t, body, ">Old<")
}

func TestTheTypeDropdownOffersAllThenEveryTypeAndKeepsTheChoice(t *testing.T) {
	ta := typeFilterApp(t)
	types := selectHTML(ta.get("/transactions").Body, "type")
	wantAll(t, types, `aria-label="Type"`, `<option value="">Type</option>`, `<option value="income">Income</option>`, `<option value="divestment">Divestment</option>`)
	wantNone(t, types, "selected")
	if strings.Count(types, "<option") != 7 {
		t.Errorf("want All and six Types:\n%s", types)
	}
	wantAll(t, selectHTML(ta.get("/transactions?type=levy").Body, "type"), `<option value="levy" selected>Levy</option>`)
}

func TestGroupingsDropdownsFollowTheType(t *testing.T) {
	ta := typeFilterApp(t)
	for _, c := range []struct {
		typ, cat, sub string
	}{
		{"income", "Category", "Subcategory"}, {"expense", "Category", "Subcategory"},
		{"levy", "Levy Category", "Levy Subcategory"}, {"rebate", "Levy Category", "Levy Subcategory"},
	} {
		body := ta.get("/transactions?type=" + c.typ).Body
		wantAll(t, selectHTML(body, "category"), `aria-label="`+c.cat+`"`, `<option value="">`+c.cat+`</option>`)
		wantAll(t, selectHTML(body, "subcategory"), `aria-label="`+c.sub+`"`, `<option value="">`+c.sub+`</option>`)
	}
	for _, q := range []string{"", "?type=investment", "?type=divestment"} {
		body := ta.get("/transactions" + q).Body
		for _, name := range []string{"category", "subcategory"} {
			if s := regexp.MustCompile(`(?s)<form[^>]*class="filters.*?</form>`).FindString(body); selectHTML(s, name) != "" {
				t.Errorf("%q: %s dropdown shown", q, name)
			}
		}
	}
}

func TestGroupingsDropdownsListEveryValueOfTheirList(t *testing.T) {
	ta := typeFilterApp(t)
	body := ta.get("/transactions?type=expense").Body
	wantAll(t, selectHTML(body, "category"), ">Food<", ">Pay<")
	wantNone(t, selectHTML(body, "category"), ">Tax<", ">Refund<")
	wantAll(t, selectHTML(body, "subcategory"), ">Cafe<", ">Job<")
	wantNone(t, selectHTML(body, "subcategory"), ">Federal<")
	body = ta.get("/transactions?type=levy").Body
	wantAll(t, selectHTML(body, "category"), ">Tax<", ">Refund<")
	wantNone(t, selectHTML(body, "category"), ">Food<")
	wantAll(t, selectHTML(body, "subcategory"), ">Federal<")
	wantNone(t, selectHTML(body, "subcategory"), ">Cafe<")
}

func TestSelectingGroupingsNarrowsAndCombinesWithAnd(t *testing.T) {
	ta := typeFilterApp(t)
	ta.post("/transactions", expenseForm("2026-01-07", "3", "Pay cafe", "new", "Pay", "1", ""))
	ta.post("/transactions", expenseForm("2026-01-08", "3", "Food job", "1", "", "new", "Job"))
	food, pay := selectedID(ta, "category", "Food"), selectedID(ta, "category", "Pay")
	cafe, job := selectedID(ta, "subcategory", "Cafe"), selectedID(ta, "subcategory", "Job")

	body := ta.get("/transactions?type=expense&category=" + food).Body
	wantAll(t, body, "<span>Expense</span>", "<span>Food job</span>", `<option value="`+food+`" selected>Food</option>`)
	wantNone(t, body, "<span>Pay cafe</span>", "<span>Income</span>")
	body = ta.get("/transactions?type=expense&subcategory=" + cafe).Body
	wantAll(t, body, "<span>Expense</span>", "<span>Pay cafe</span>")
	wantNone(t, body, "<span>Food job</span>")
	body = ta.get("/transactions?type=expense&category=" + food + "&subcategory=" + cafe).Body
	wantAll(t, body, "<span>Expense</span>")
	wantNone(t, body, "<span>Pay cafe</span>", "<span>Food job</span>")
	body = ta.get("/transactions?type=expense&category=" + pay + "&subcategory=" + job).Body
	wantAll(t, body, "No matching Transactions.", `name="type"`, `name="category"`)
	wantNone(t, body, "No Transactions yet.")
}

// selectedID is the value of the option named name in the filter's select.
func selectedID(ta *testApp, sel, name string) string {
	ta.t.Helper()
	body := ta.get("/transactions?type=expense").Body
	m := regexp.MustCompile(`<option value="(\d+)"[^>]*>` + name + `</option>`).FindStringSubmatch(selectHTML(body, sel))
	if m == nil {
		ta.t.Fatalf("no %s option %q", sel, name)
	}
	return m[1]
}

func TestChangingTheTypeDropsTheGroupingsSelection(t *testing.T) {
	ta := typeFilterApp(t)
	body := ta.get("/transactions").Body
	wantAll(t, selectHTML(body, "type"), `this.form.querySelectorAll('.grouping')`, `e.disabled = true`, `this.form.submit()`)
	body = ta.get("/transactions?type=expense").Body
	wantAll(t, selectHTML(body, "category"), `class="grouping"`)
	wantAll(t, selectHTML(body, "subcategory"), `class="grouping"`)
	// A stale grouping that reached the server for a Type without such lists narrows nothing.
	wantAll(t, ta.get("/transactions?type=investment&category=1&subcategory=1").Body, ">Investment<")
}

func TestUnknownTypeOrGroupingIdsFallBackToUnfiltered(t *testing.T) {
	ta := typeFilterApp(t)
	for _, q := range []string{"type=bogus", "type=", "type=Expense", "type=bogus&category=1", "category=1&subcategory=1"} {
		r := ta.get("/transactions?" + q)
		if r.Status != 200 {
			t.Errorf("%s: status %d", q, r.Status)
		}
		wantAll(t, r.Body, "<span>Income</span>", "<span>Divestment</span>")
		wantNone(t, selectHTML(r.Body, "type"), "selected")
	}
	for _, q := range []string{"category=999", "category=abc", "category=-1", "category=1x", "subcategory=999", "category=99999999999999999999999"} {
		r := ta.get("/transactions?type=expense&" + q)
		if r.Status != 200 {
			t.Errorf("%s: status %d", q, r.Status)
		}
		wantAll(t, r.Body, "<span>Expense</span>")
		wantNone(t, r.Body, "No matching Transactions.", "<span>Income</span>")
		wantNone(t, selectHTML(r.Body, "category"), "selected")
		wantNone(t, selectHTML(r.Body, "subcategory"), "selected")
	}
}

func TestThePagerAndEditKeepTypeAndGroupings(t *testing.T) {
	ta := newTestApp(t)
	for i := 1; i <= 7; i++ {
		ta.post("/transactions", expenseForm(fmt.Sprintf("2026-08-%02d", i), "3", fmt.Sprintf("Exp %02d", i), "new", "Food", "new", "Cafe"))
	}
	ta.post("/transactions", url.Values{"type": {"income"}, "date": {"2026-08-09"}, "amount": {"3"}, "description": {"Inc"},
		"category": {"1"}, "subcategory": {"1"}})
	q := "type=expense&category=1&subcategory=1"
	p1 := ta.get("/transactions?" + q).Body
	wantAll(t, p1, "1 / 2", `href="/transactions?`+strings.ReplaceAll(q, "&", "&amp;")+`&amp;page=2" aria-label="Older"`,
		`href="/transactions?`+strings.ReplaceAll(q, "&", "&amp;")+`&amp;page=1&amp;edit=7#tx-7"`)
	wantNone(t, p1, ">Inc<")
	wantAll(t, editRow(ta.get("/transactions?"+q+"&page=2&edit=2").Body, "2"), `name="return" value="transactions:2?`+strings.ReplaceAll(q, "&", "&amp;")+`"`)
}

func TestChangeDeleteAndDiscardReturnToThePageWithTypeAndGroupings(t *testing.T) {
	ta := newTestApp(t)
	for i := 1; i <= 6; i++ {
		ta.post("/transactions", expenseForm(fmt.Sprintf("2026-08-%02d", i), "3", fmt.Sprintf("Exp %02d", i), "new", "Food", "new", "Cafe"))
	}
	ret := "transactions:2?type=expense&category=1&subcategory=1"
	// Exp 01 (id 1) moved to income leaves the filter: back to the clamped same page.
	form := changeForm(ta, "2026-08-01", "Exp 01", ret)
	form.Set("type", "income")
	r := ta.post("/transactions/1", form)
	if r.Status != 303 || r.Location != "/transactions?type=expense&category=1&subcategory=1&page=1" {
		t.Fatalf("status %d, location %q", r.Status, r.Location)
	}
	r = ta.post("/transactions/2/delete", url.Values{"return": {ret}})
	if r.Location != "/transactions?type=expense&category=1&subcategory=1&page=1" {
		t.Errorf("delete: location %q", r.Location)
	}
	wantAll(t, ta.get("/transactions?type=expense&category=1&subcategory=1&page=1&edit=6").Body,
		`href="/transactions?type=expense&amp;category=1&amp;subcategory=1&amp;page=1"`)
}

func TestATamperedTypeOrGroupingInTheReturnValueNarrowsNothing(t *testing.T) {
	ta := newTestApp(t)
	addMonths(ta, 6)
	for i, c := range []struct{ ret, want string }{
		{"transactions:1?type=evil", "/transactions?page=1"},
		{"transactions:1?type=expense%26page%3D9", "/transactions?page=1"},
		{"transactions:1?type=expense&category=//evil.example", "/transactions?type=expense&page=1"},
		{"transactions:1?category=1", "/transactions?page=1"},
	} {
		if r := ta.post(fmt.Sprintf("/transactions/%d/delete", 12-i), url.Values{"return": {c.ret}}); r.Location != c.want {
			t.Errorf("%q: status %d, location %q, want %q", c.ret, r.Status, r.Location, c.want)
		}
	}
}

func TestClearResetsTypeAndGroupings(t *testing.T) {
	ta := typeFilterApp(t)
	wantAll(t, regexp.MustCompile(`(?s)<a class="clear".*?</a>`).FindString(ta.get("/transactions?type=expense&category=1").Body),
		`href="/transactions"`, ">Clear</a>")
}

// holdingFilterApp holds an Investment and a Divestment of ETF, an Investment of Gold and an Expense.
func holdingFilterApp(t *testing.T) *testApp {
	t.Helper()
	ta := newTestApp(t)
	ta.post("/transactions", holdingForm("investment", "2026-01-01", "3", "new", "ETF"))
	ta.post("/transactions", holdingForm("divestment", "2026-01-02", "3", "new", "Gold"))
	ta.post("/transactions", holdingForm("investment", "2026-02-03", "3", "new", "Gold"))
	ta.post("/transactions", expenseForm("2026-01-04", "3", "Lunch", "new", "Food", "new", "Cafe"))
	return ta
}

// holdingID is the id of the Holding named name in the filter's Holding dropdown.
func holdingID(ta *testApp, name string) string {
	ta.t.Helper()
	m := regexp.MustCompile(`<option value="(\d+)"[^>]*>` + name + `</option>`).FindStringSubmatch(selectHTML(ta.get("/transactions?type=investment").Body, "holding"))
	if m == nil {
		ta.t.Fatalf("no Holding option %q", name)
	}
	return m[1]
}

func TestTheHoldingDropdownAppearsOnlyForInvestmentAndDivestment(t *testing.T) {
	ta := holdingFilterApp(t)
	for _, typ := range []string{"investment", "divestment"} {
		body := ta.get("/transactions?type=" + typ).Body
		sel := selectHTML(body, "holding")
		wantAll(t, sel, `aria-label="Holding"`, `<option value="">Holding</option>`, ">ETF<", ">Gold<", `onchange="this.form.submit()"`)
		wantNone(t, body, `name="search"`)
		wantNone(t, selectHTML(body, "category")+selectHTML(body, "subcategory"), "<select")
	}
	for _, q := range []string{"", "?type=expense", "?type=levy"} {
		wantNone(t, ta.get("/transactions"+q).Body, `name="holding"`)
	}
}

func TestSelectingAHoldingShowsOnlyItsTransactionsOfTheType(t *testing.T) {
	ta := holdingFilterApp(t)
	gold := holdingID(ta, "Gold")
	body := ta.get("/transactions?type=investment&holding=" + gold).Body
	wantAll(t, body, "2026-02-03")
	wantNone(t, body, "2026-01-01", "2026-01-02", "Lunch")
	wantAll(t, ta.get("/transactions?type=divestment&holding="+gold).Body, "2026-01-02")
	wantAll(t, selectHTML(body, "holding"), `value="`+gold+`" selected`)
	// Combines with Period using AND.
	body = ta.get("/transactions?type=investment&holding=" + gold + "&year=2026&month=1").Body
	wantAll(t, body, "No matching Transactions.")
}

func TestAnUnknownHoldingFallsBackToUnfiltered(t *testing.T) {
	ta := holdingFilterApp(t)
	for _, h := range []string{"999", "abc", "-1", "", "99999999999999999999999"} {
		r := ta.get("/transactions?type=investment&holding=" + h)
		if r.Status != 200 {
			t.Errorf("%q: status %d", h, r.Status)
		}
		wantAll(t, r.Body, "2026-01-01", "2026-02-03")
		wantNone(t, r.Body, "No matching Transactions.")
		wantNone(t, selectHTML(r.Body, "holding"), "selected")
	}
	// A stale Holding for a Type without one narrows nothing.
	wantAll(t, ta.get("/transactions?type=expense&holding=1").Body, "Lunch")
}

func TestTheHoldingSurvivesPagerEditAndReturns(t *testing.T) {
	ta := holdingFilterApp(t)
	gold := holdingID(ta, "Gold")
	q := "type=investment&holding=" + gold
	wantAll(t, ta.get("/transactions?"+q+"&page=1&edit=3").Body, `href="/transactions?type=investment&amp;holding=`+gold+`&amp;page=1"`)
	r := ta.post("/transactions/3/delete", url.Values{"return": {"transactions:1?" + q}})
	if r.Location != "/transactions?"+q+"&page=1" {
		t.Errorf("delete: location %q", r.Location)
	}
}

func TestChangingTheTypeDropsTheHoldingAndClearResetsIt(t *testing.T) {
	ta := holdingFilterApp(t)
	body := ta.get("/transactions?type=investment").Body
	wantAll(t, selectHTML(body, "holding"), `class="grouping holding"`)
	wantAll(t, selectHTML(body, "type"), `e.disabled = true`)
	wantAll(t, regexp.MustCompile(`(?s)<a class="clear".*?</a>`).FindString(ta.get("/transactions?type=investment&holding=1").Body),
		`href="/transactions"`, ">Clear</a>")
}

// searchApp holds an Investment in ETF World, an Expense "Etf fees", Expense "Rent" and a Levy "Rent tax".
func searchApp(t *testing.T) *testApp {
	t.Helper()
	ta := newTestApp(t)
	ta.post("/transactions", holdingForm("investment", "2026-01-01", "3", "new", "ETF World"))
	ta.post("/transactions", expenseForm("2026-01-02", "3", "Etf fees", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", expenseForm("2026-02-03", "3", "Rent", "new", "Home", "new", "Flat"))
	ta.post("/transactions", levyForm("2026-02-04", "3", "Rent tax", "new", "Tax", "new", "Federal"))
	return ta
}

func TestSearchMatchesDescriptionAndHoldingCombinedWithThePeriod(t *testing.T) {
	ta := searchApp(t)
	body := ta.get("/transactions?search=etf").Body
	wantAll(t, body, "<span>ETF World</span>", "<span>Etf fees</span>")
	wantNone(t, body, "<span>Rent</span>", "<span>Rent tax</span>")
	wantAll(t, ta.get("/transactions?search=etf&year=2026&month=2").Body, "No matching Transactions.", `name="search"`)
	wantNone(t, ta.get("/transactions?search=rent&type=expense").Body, "<span>Rent tax</span>")
	wantAll(t, ta.get("/transactions?search=rent").Body, "<span>Rent</span>", "<span>Rent tax</span>")
}

func TestSearchCombinesWithGroupingsAndEscapesItsValue(t *testing.T) {
	ta := searchApp(t)
	wantAll(t, ta.get("/transactions?type=expense&search=e&category=1").Body, "Etf fees")
	wantNone(t, ta.get("/transactions?type=expense&search=e&category=1").Body, "<span>Rent</span>")
	body := ta.get(`/transactions?search=%22%3E%3Cb%3E`).Body
	wantAll(t, body, `value="&#34;&gt;&lt;b&gt;"`, "No matching Transactions.")
	wantNone(t, body, "<b>")
}

func TestSearchFieldPlacementFollowsTheType(t *testing.T) {
	ta := searchApp(t)
	all := ta.get("/transactions").Body
	wantAll(t, all, `type="search"`)
	wantAll(t, regexp.MustCompile(`(?s)<div class="row2">.*?</div>`).FindString(all), `name="search"`)
	for _, typ := range []string{"expense", "income", "levy", "rebate"} {
		body := ta.get("/transactions?type=" + typ).Body
		wantAll(t, body, `name="search"`)
		wantNone(t, regexp.MustCompile(`(?s)<div class="row2">.*?</div>`).FindString(body), `name="search"`)
		wantAll(t, regexp.MustCompile(`(?s)<div class="row3">.*?</div>`).FindString(body), `name="search"`)
	}
	for _, typ := range []string{"investment", "divestment"} {
		wantNone(t, ta.get("/transactions?type="+typ).Body, `name="search"`)
	}
}

func TestSearchTextSurvivesTypeChangesWhereTheFieldExistsAndIsDroppedOtherwise(t *testing.T) {
	ta := searchApp(t)
	wantAll(t, ta.get("/transactions?type=expense&search=rent").Body, `value="rent"`)
	wantAll(t, ta.get("/transactions?type=&search=rent").Body, `value="rent"`)
	body := ta.get("/transactions?type=investment&search=etf").Body
	wantNone(t, body, "<span>Rent", `value="etf"`, "No matching Transactions.")
	wantAll(t, body, "ETF World")
	wantAll(t, regexp.MustCompile(`(?s)<a class="clear".*?</a>`).FindString(ta.get("/transactions?search=rent").Body), `href="/transactions"`)
}

func TestSearchSurvivesPagerEditAndReturns(t *testing.T) {
	ta := searchApp(t)
	body := ta.get("/transactions?search=rent&edit=3").Body
	wantAll(t, body, `href="/transactions?search=rent&amp;page=1"`, `value="transactions:1?search=rent"`)
	r := ta.post("/transactions/3/delete", url.Values{"return": {"transactions:1?search=rent"}})
	if r.Location != "/transactions?search=rent&page=1" {
		t.Errorf("delete: location %q", r.Location)
	}
	wantNone(t, ta.get("/transactions?search=rent").Body, "<span>Rent</span>")
}

func TestSearchAddsTheFilterToThePagerLinks(t *testing.T) {
	ta := newTestApp(t)
	for i := 1; i <= 6; i++ {
		ta.post("/transactions", expenseForm(fmt.Sprintf("2026-01-%02d", i), "3", fmt.Sprintf("Rent %d", i), "new", "Food", "new", "Cafe"))
	}
	wantAll(t, ta.get("/transactions?search=rent").Body, `href="/transactions?search=rent&amp;page=2"`, "1 / 2")
}
