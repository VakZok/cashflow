package web_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// addTransactions records n Expenses; Expense i is dated 2026-09-(i), so a higher i is newer.
func addTransactions(ta *testApp, n int) {
	ta.t.Helper()
	for i := 1; i <= n; i++ {
		ta.post("/transactions", expenseForm(fmt.Sprintf("2026-09-%02d", i), "3", fmt.Sprintf("Item %02d", i), "new", "Food", "new", "Cafe"))
	}
}

func TestTransactionsPageListsNewestFirstFivePerPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)

	p1 := ta.get("/transactions?year=all").Body
	wantAll(t, p1, "Item 12", "Item 11", "Item 10", "Item 09", "Item 08", "1 / 3")
	if strings.Contains(p1, "Item 07") {
		t.Error("page 1 shows a sixth Transaction")
	}
	if strings.Index(p1, "Item 12") > strings.Index(p1, "Item 11") {
		t.Error("page 1 is not newest first")
	}
	p2 := ta.get("/transactions?year=all&page=2").Body
	wantAll(t, p2, "Item 07", "Item 03", "2 / 3")
	p3 := ta.get("/transactions?year=all&page=3").Body
	wantAll(t, p3, "Item 02", "Item 01", "3 / 3")
}

func TestTransactionsPageOrdersSameDateByLatestEntered(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-09-01", "3", "First entered", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", expenseForm("2026-09-01", "3", "Last entered", "", "", "", ""))
	body := ta.get("/transactions?year=all").Body
	if strings.Index(body, "Last entered") > strings.Index(body, "First entered") {
		t.Error("same-date Transactions are not ordered last entered first")
	}
}

func TestTransactionsPageArrows(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)

	first := ta.get("/transactions?year=all").Body
	wantAll(t, first, `href="/transactions?year=all&amp;page=2" aria-label="Older"`, `<span class="arrow" aria-label="Newer">`)
	middle := ta.get("/transactions?year=all&page=2").Body
	wantAll(t, middle, `href="/transactions?year=all&amp;page=1" aria-label="Newer"`, `href="/transactions?year=all&amp;page=3" aria-label="Older"`)
	last := ta.get("/transactions?year=all&page=3").Body
	wantAll(t, last, `href="/transactions?year=all&amp;page=2" aria-label="Newer"`, `<span class="arrow" aria-label="Older">`)
}

func TestTransactionsPageFallsBackToTheNearestValidPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)
	for query, want := range map[string]string{
		"": "1 / 3", "&page=0": "1 / 3", "&page=-3": "1 / 3", "&page=abc": "1 / 3", "&page=999": "3 / 3",
	} {
		r := ta.get("/transactions?year=all" + query)
		if r.Status != 200 || !strings.Contains(r.Body, want) {
			t.Errorf("%q: status %d, want 200 and %q", query, r.Status, want)
		}
	}
}

func TestTransactionsPageWithoutTransactionsHasNoArrows(t *testing.T) {
	ta := newTestApp(t)
	for _, q := range []string{"", "?page=4"} {
		r := ta.get("/transactions" + q)
		if r.Status != 200 || !strings.Contains(r.Body, "No Transactions yet.") {
			t.Errorf("%q: status %d without the empty message", q, r.Status)
		}
		if strings.Contains(r.Body, `aria-label="Newer"`) || strings.Contains(r.Body, `aria-label="Older"`) {
			t.Errorf("%q: empty page has arrows", q)
		}
	}
}

func TestTransactionsPageWithOnePageGreysBothArrows(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 5)
	body := ta.get("/transactions?year=all").Body
	wantAll(t, body, `<span class="arrow" aria-label="Newer">`, `<span class="arrow" aria-label="Older">`, "1 / 1")
}

// The test clock is 2026-10-05; addTransactions dates its Expenses in 2026-09.
func TestTransactionsPageOpensOnTheCurrentMonthNewestFirst(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 3)
	ta.post("/transactions", expenseForm("2026-10-01", "3", "October first", "1", "", "1", ""))
	ta.post("/transactions", expenseForm("2026-10-04", "3", "October fourth", "1", "", "1", ""))

	for _, path := range []string{"/transactions", "/transactions?page=1"} {
		body := ta.get(path).Body
		wantAll(t, body, "October fourth", "October first", "1 / 1",
			`<option value="2026" selected>`, `<option value="10" selected>Oct</option>`)
		if strings.Index(body, "October fourth") > strings.Index(body, "October first") {
			t.Errorf("%s is not newest first", path)
		}
		if strings.Contains(body, "Item 0") {
			t.Errorf("%s shows a September Transaction", path)
		}
	}
}

func TestAnEmptyCurrentMonthIsStillSelected(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 3)
	ta.post("/transactions", expenseForm("2025-03-01", "3", "Last year", "1", "", "1", ""))

	body := ta.get("/transactions").Body
	wantAll(t, body, "No matching Transactions.", `<option value="2026" selected>`, `<option value="9">Sep</option>`, `<option value="10" selected>Oct</option>`)

	// A year without Transactions yet is still offered with its month.
	ta = newTestApp(t)
	ta.post("/transactions", expenseForm("2025-03-01", "3", "Last year", "new", "Food", "new", "Cafe"))
	body = ta.get("/transactions").Body
	wantAll(t, body, "No matching Transactions.", `<option value="2026" selected>`, `<option value="2025">`, `<option value="10" selected>Oct</option>`)
	if strings.Index(body, `<option value="2026"`) > strings.Index(body, `<option value="2025"`) {
		t.Error("the current year is not listed newest first")
	}
	if strings.Contains(body, "Last year") {
		t.Error("the current month shows last year's Transaction")
	}
}

func TestClearShowsAllTransactionsAndPagingAndEditingKeepAll(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)
	ta.post("/transactions", expenseForm("2026-10-01", "3", "October first", "1", "", "1", ""))

	wantAll(t, ta.get("/transactions").Body, `<a class="clear" href="/transactions?year=all">Clear</a>`, `<option value="all">Year</option>`)

	all := ta.get("/transactions?year=all").Body
	wantAll(t, all, "October first", "Item 12", "1 / 3", `<option value="all">Year</option>`,
		`href="/transactions?year=all&amp;page=2" aria-label="Older"`,
		`href="/transactions?year=all&amp;page=1&amp;edit=13#tx-13"`)
	if strings.Contains(all, " selected>2026<") || strings.Contains(all, `aria-label="Month"`) {
		t.Error("Clear still narrows to a Year")
	}
	// The pager and a reload stay on all Transactions.
	wantAll(t, ta.get("/transactions?year=all&page=2").Body, "Item 08", "2 / 3", `href="/transactions?year=all&amp;page=3" aria-label="Older"`)

	// Editing a row keeps "all" in its Discard link and return value, and Apply and Delete return to it.
	row := editRow(ta.get("/transactions?year=all&page=2&edit=5").Body, "5")
	wantAll(t, row, `name="return" value="transactions:2?year=all"`, `href="/transactions?year=all&amp;page=2"`)
	r := ta.post("/transactions/5", changeForm(ta, "2026-09-05", "Item 05", "transactions:2?year=all"))
	if r.Location != "/transactions?year=all&page=2#tx-5" {
		t.Errorf("apply: location %q", r.Location)
	}
	r = ta.post("/transactions/5/delete", url.Values{"return": {"transactions:2?year=all"}})
	if r.Location != "/transactions?year=all&page=2" {
		t.Errorf("delete: location %q", r.Location)
	}
	// Page 2 of all Transactions, not of the current month (which has one page).
	wantAll(t, ta.follow(r).Body, "Item 08", "Item 03", "2 / 3")
}

func TestAnEmptyYearParameterIsAllYears(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 2)
	ta.post("/transactions", expenseForm("2026-10-01", "3", "October first", "1", "", "1", ""))
	wantAll(t, ta.get("/transactions?year=").Body, "October first", "Item 01", `href="/transactions?year=all&amp;page=1&amp;edit=1#tx-1"`)
}

func TestAddPageHasNoTransactionList(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 7)

	for _, path := range []string{"/", "/?added=1", "/?edit=1"} {
		body := ta.get(path).Body
		for _, gone := range []string{"Newest transactions", "Item 07", `class="transactions"`, `action="/transactions/`, `class="errors"`} {
			if strings.Contains(body, gone) {
				t.Errorf("%s shows %q", path, gone)
			}
		}
	}
}

func TestBackDatedTransactionIsOnTheTransactionsPageNotTheAddPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 5)
	r := ta.post("/transactions", expenseForm("2026-01-01", "3", "Back dated", "new", "Food", "new", "Cafe"))
	if r.Status != 303 {
		t.Fatalf("add status %d, want 303", r.Status)
	}
	if strings.Contains(ta.get("/").Body, "Back dated") {
		t.Error("back-dated Transaction is on the add page")
	}
	if !strings.Contains(ta.get("/transactions?year=all&page=2").Body, "Back dated") {
		t.Error("back-dated Transaction is not on the Transactions page")
	}
}

func TestAddPageValidationErrorHasNoListAndNoErrorBox(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 7)
	r := ta.post("/transactions", expenseForm("2026-09-30", "0", "Bad", "new", "Food", "new", "Cafe"))
	if r.Status != 422 {
		t.Fatalf("status %d, want 422", r.Status)
	}
	for _, gone := range []string{"Item 07", `class="errors"`, `role="alert"><ul>`} {
		if strings.Contains(r.Body, gone) {
			t.Errorf("422 page shows %q", gone)
		}
	}
}
