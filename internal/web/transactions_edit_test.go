package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// changeForm is the edit form of Expense id as the browser submits it: the
// Category and Subcategory of addTransactions' Expenses, and the hidden return value.
func changeForm(ta *testApp, date, desc, ret string) url.Values {
	body := ta.get("/").Body
	f := expenseForm(date, "3", desc, optionValue(ta.t, body, "category", "Food"), "", optionValue(ta.t, body, "subcategory", "Cafe"), "")
	if ret != "" {
		f.Set("return", ret)
	}
	return f
}

func TestClickingARowOnTheTransactionsPageOpensItsEditFormOnTheSamePage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)

	wantAll(t, editRow(ta.get("/transactions?page=2").Body, "5"), `href="/transactions?page=2&amp;edit=5#tx-5"`)

	r := ta.get("/transactions?page=2&edit=5")
	row := editRow(r.Body, "5")
	wantAll(t, row, `action="/transactions/5#tx-5"`, `name="return" value="transactions:2"`, `value="Item 05"`,
		`href="/transactions?page=2"`, "Discard", "Change", "Delete", `formaction="/transactions/5/delete"`)
	if !(strings.Index(row, "Change") < strings.Index(row, "Discard") && strings.Index(row, "Discard") < strings.Index(row, "Delete")) {
		t.Errorf("buttons are not Change, Discard, Delete in the markup:\n%s", row)
	}
	wantAll(t, r.Body, "2 / 3", "function syncGroups")
	if other := editRow(r.Body, "6"); strings.Contains(other, "<form") {
		t.Errorf("other row is a form: %s", other)
	}
}

func TestEditingARowOnAnotherPageOfTheTransactionsPageShowsNoForm(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)
	r := ta.get("/transactions?page=1&edit=1")
	if r.Status != http.StatusOK || strings.Contains(r.Body, `class="editing"`) {
		t.Errorf("status %d, edit form for a row outside the page", r.Status)
	}
}

func TestAddPageEditFormReturnsToTheAddPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 7)
	wantAll(t, editRow(ta.get("/?edit=7").Body, "7"), `name="return" value="add"`, `href="/"`)
}

func TestChangeOnTheTransactionsPageFollowsTheRowToItsNewPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)

	// Same date: back to the same page, at the row.
	r := ta.post("/transactions/5", changeForm(ta, "2026-09-05", "Item 05 changed", "transactions:2"))
	if r.Status != http.StatusSeeOther || r.Location != "/transactions?page=2#tx-5" {
		t.Fatalf("status %d, location %q; want 303 /transactions?page=2#tx-5\n%s", r.Status, r.Location, r.Body)
	}
	wantAll(t, ta.follow(r).Body, "Item 05 changed")

	// A new date moves it: from page 1 to the oldest, page 3.
	r = ta.post("/transactions/12", changeForm(ta, "2026-08-01", "Item 12", "transactions:1"))
	if r.Status != http.StatusSeeOther || r.Location != "/transactions?page=3#tx-12" {
		t.Fatalf("status %d, location %q; want 303 /transactions?page=3#tx-12", r.Status, r.Location)
	}
	wantAll(t, editRow(ta.follow(r).Body, "12"), "2026-08-01")
}

func TestChangeFromTheAddPageStillReturnsToTheAddPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 7)
	// The date change pushes the row out of the 5 shown: the add page does not follow it.
	r := ta.post("/transactions/7", changeForm(ta, "2026-01-01", "Item 07", "add"))
	if r.Status != http.StatusSeeOther || r.Location != "/#tx-7" {
		t.Fatalf("status %d, location %q; want 303 /#tx-7", r.Status, r.Location)
	}
}

func TestDeleteFromTheTransactionsPageReturnsToTheSamePage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)
	r := ta.post("/transactions/5/delete", url.Values{"return": {"transactions:2"}})
	if r.Status != http.StatusSeeOther || r.Location != "/transactions?page=2" {
		t.Fatalf("status %d, location %q; want 303 /transactions?page=2", r.Status, r.Location)
	}
	body := ta.follow(r).Body
	wantAll(t, body, "2 / 3", "Item 02")
	if strings.Contains(body, "Item 05") {
		t.Error("deleted Transaction still listed")
	}
}

func TestDeletingTheOnlyRowOfTheLastPageLandsOnTheNewLastPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 11)
	r := ta.post("/transactions/1/delete", url.Values{"return": {"transactions:3"}})
	if r.Status != http.StatusSeeOther || r.Location != "/transactions?page=2" {
		t.Fatalf("status %d, location %q; want 303 /transactions?page=2", r.Status, r.Location)
	}
	wantAll(t, ta.follow(r).Body, "Item 06", "2 / 2")
}

func TestDeletingTheLastTransactionShowsTheEmptyState(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 1)
	r := ta.post("/transactions/1/delete", url.Values{"return": {"transactions:1"}})
	if r.Status != http.StatusSeeOther || r.Location != "/transactions?page=1" {
		t.Fatalf("status %d, location %q; want 303 /transactions?page=1", r.Status, r.Location)
	}
	wantAll(t, ta.follow(r).Body, "No Transactions yet.")
}

func TestDeleteFromTheAddPageReturnsToTheAddPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 2)
	r := ta.post("/transactions/1/delete", url.Values{"return": {"add"}})
	if r.Status != http.StatusSeeOther || r.Location != "/" {
		t.Fatalf("status %d, location %q; want 303 /", r.Status, r.Location)
	}
}

func TestInvalidChangeFromTheTransactionsPageRerendersThatPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)

	bad := changeForm(ta, "2026-09-05", "Item 05", "transactions:2")
	bad.Set("amount", "0")
	r := ta.post("/transactions/5", bad)
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	wantAll(t, r.Body, "2 / 3", "Item 03")
	wantAll(t, editRow(r.Body, "5"), `role="alert"`, `name="return" value="transactions:2"`, "Amount must be a whole number")

	// A Duplicate of another Transaction.
	dup := changeForm(ta, "2026-09-06", "Item 06", "transactions:2")
	r = ta.post("/transactions/5", dup)
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate: status %d, want 422", r.Status)
	}
	wantAll(t, editRow(r.Body, "5"), errDuplicateText)
	wantAll(t, r.Body, "2 / 3")
}

func TestATamperedReturnValueFallsBackToTheAddPage(t *testing.T) {
	for _, ret := range []string{
		"", "nonsense", "https://evil.example/", "//evil.example", "/transactions?page=2", "javascript:alert(1)",
		"transactions:", "transactions:0", "transactions:-1", "transactions:abc", "transactions:2/../..", "transactions:99999999999999999999",
	} {
		ta := newTestApp(t)
		addTransactions(ta, 2)
		if r := ta.post("/transactions/1", changeForm(ta, "2026-09-01", "Item 01", ret)); r.Location != "/#tx-1" {
			t.Errorf("change with return %q: status %d, location %q; want /#tx-1", ret, r.Status, r.Location)
		}
		if r := ta.post("/transactions/1/delete", url.Values{"return": {ret}}); r.Location != "/" {
			t.Errorf("delete with return %q: status %d, location %q; want /", ret, r.Status, r.Location)
		}
	}
}

func TestAReturnValuePastTheEndIsClampedToTheLastPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)
	r := ta.post("/transactions/12/delete", url.Values{"return": {"transactions:999"}})
	if r.Location != "/transactions?page=3" {
		t.Errorf("location %q, want /transactions?page=3", r.Location)
	}
}
