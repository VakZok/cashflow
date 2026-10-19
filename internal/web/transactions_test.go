package web_test

import (
	"fmt"
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

	p1 := ta.get("/transactions").Body
	wantAll(t, p1, "Item 12", "Item 11", "Item 10", "Item 09", "Item 08", "1 / 3")
	if strings.Contains(p1, "Item 07") {
		t.Error("page 1 shows a sixth Transaction")
	}
	if strings.Index(p1, "Item 12") > strings.Index(p1, "Item 11") {
		t.Error("page 1 is not newest first")
	}
	p2 := ta.get("/transactions?page=2").Body
	wantAll(t, p2, "Item 07", "Item 03", "2 / 3")
	p3 := ta.get("/transactions?page=3").Body
	wantAll(t, p3, "Item 02", "Item 01", "3 / 3")
}

func TestTransactionsPageOrdersSameDateByLatestEntered(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-09-01", "3", "First entered", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", expenseForm("2026-09-01", "3", "Last entered", "", "", "", ""))
	body := ta.get("/transactions").Body
	if strings.Index(body, "Last entered") > strings.Index(body, "First entered") {
		t.Error("same-date Transactions are not ordered last entered first")
	}
}

func TestTransactionsPageArrows(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)

	first := ta.get("/transactions").Body
	wantAll(t, first, `href="/transactions?page=2" aria-label="Older"`, `<span class="arrow" aria-label="Newer">`)
	middle := ta.get("/transactions?page=2").Body
	wantAll(t, middle, `href="/transactions?page=1" aria-label="Newer"`, `href="/transactions?page=3" aria-label="Older"`)
	last := ta.get("/transactions?page=3").Body
	wantAll(t, last, `href="/transactions?page=2" aria-label="Newer"`, `<span class="arrow" aria-label="Older">`)
}

func TestTransactionsPageFallsBackToTheNearestValidPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)
	for query, want := range map[string]string{
		"": "1 / 3", "?page=0": "1 / 3", "?page=-3": "1 / 3", "?page=abc": "1 / 3", "?page=999": "3 / 3",
	} {
		r := ta.get("/transactions" + query)
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
	body := ta.get("/transactions").Body
	wantAll(t, body, `<span class="arrow" aria-label="Newer">`, `<span class="arrow" aria-label="Older">`, "1 / 1")
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
	if !strings.Contains(ta.get("/transactions?page=2").Body, "Back dated") {
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
