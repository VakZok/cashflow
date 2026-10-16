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
	wantAll(t, years, `<option value="">All</option>`, `<option value="2026">2026</option>`, `<option value="2024">2024</option>`)
	wantNone(t, years, "2025", "selected")
	if strings.Index(years, "All") > strings.Index(years, "2026") || strings.Index(years, "2026") > strings.Index(years, "2024") {
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
	wantAll(t, months, `<option value="">All</option>`, `<option value="1">Jan</option>`, `<option value="12">Dec</option>`)
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

func TestClearShowsTextUnfilteredAndOnlyItsIconOnceAYearIsChosen(t *testing.T) {
	ta := newTestApp(t)
	addDated(ta, "2026-01-01")
	clear := regexp.MustCompile(`(?s)<a class="clear".*?</a>`)
	wantAll(t, clear.FindString(ta.get("/transactions").Body), `href="/transactions"`, "<svg", "<span>Clear</span>")
	tight := clear.FindString(ta.get("/transactions?year=2026&month=1").Body)
	wantAll(t, tight, `href="/transactions"`, `aria-label="Clear"`, "<svg")
	wantNone(t, tight, "<span>Clear</span>")
	wantAll(t, ta.get("/transactions?year=2026").Body, `class="filters tight"`)
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
