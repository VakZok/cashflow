package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func deleteEntry(ta *testApp, list, id string) response {
	ta.t.Helper()
	return ta.post("/categories/"+id+"/delete", url.Values{"list": {list}})
}

func TestDeletingAnUnusedEntryRemovesItAndStaysOnThePage(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "category", 7)
	row := openRow(ta.get("/categories?list=category&page=2&edit=7").Body)
	if !regexp.MustCompile(`<button class="primary" type="submit" data-arm formaction="/categories/7/delete" formnovalidate>Delete</button>`).MatchString(row) {
		t.Errorf("Delete is not a double-tap submit button:\n%s", row)
	}
	r := deleteEntry(ta, "category", "6")
	if r.Status != 303 || r.Location != "/categories?list=category&page=2" {
		t.Fatalf("status %d, Location %q", r.Status, r.Location)
	}
	body := ta.follow(r).Body
	if strings.Contains(body, "Entry 06") || !strings.Contains(body, "Entry 07") || !strings.Contains(body, "2 / 2") {
		t.Errorf("entry not deleted or wrong page:\n%s", body)
	}
}

func TestDeletingTheLastEntryOnAPageFallsBackToThePageBefore(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "category", 6)
	r := deleteEntry(ta, "category", "6")
	if r.Location != "/categories?list=category&page=2" {
		t.Fatalf("Location %q", r.Location)
	}
	wantAll(t, ta.follow(r).Body, "Entry 05", "1 / 1")
}

func TestDeletingAnEntryUsedByTransactionsIsRefused(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-09-01", "3", "Lunch", "new", "Food", "new", "Cafe"))
	row := openRow(ta.get("/categories?list=category&edit=1").Body)
	if !regexp.MustCompile(`<button type="button"[^>]*>Delete</button>`).MatchString(row) || strings.Contains(row, "formaction") {
		t.Errorf("Delete of an entry in use submits:\n%s", row)
	}
	wantAll(t, ta.get("/categories?list=category&edit=1").Body, "'In Use'", "3000")
	if r := deleteEntry(ta, "category", "1"); r.Status != 409 {
		t.Errorf("status %d, want 409", r.Status)
	}
	wantAll(t, ta.get("/categories").Body, "Food", "1 Trans.")
}

func TestDeletingAnEntryUsedOnlyByRepeatingEntriesIsRefused(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("expense", "5", "Rent")))
	row := openRow(ta.get("/categories?list=category&edit=1").Body)
	if strings.Contains(row, "formaction") {
		t.Errorf("Delete of an entry in use submits:\n%s", row)
	}
	if r := deleteEntry(ta, "category", "1"); r.Status != 409 {
		t.Errorf("status %d, want 409", r.Status)
	}
	wantAll(t, ta.get("/categories").Body, "Cat", "0 Trans. + 1 Rep.")
}

func TestDeletingAMissingEntryIsNotFound(t *testing.T) {
	ta := newTestApp(t)
	if r := deleteEntry(ta, "category", "99"); r.Status != 404 {
		t.Errorf("status %d, want 404", r.Status)
	}
	if r := deleteEntry(ta, "category", "x"); r.Status != 404 {
		t.Errorf("status %d, want 404", r.Status)
	}
}
