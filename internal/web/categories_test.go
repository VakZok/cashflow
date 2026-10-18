package web_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func addEntry(ta *testApp, list, name string) response {
	ta.t.Helper()
	return ta.post("/categories", url.Values{"list": {list}, "name": {name}})
}

func addEntries(ta *testApp, list string, n int) {
	ta.t.Helper()
	for i := 1; i <= n; i++ {
		addEntry(ta, list, fmt.Sprintf("Entry %02d", i))
	}
}

func TestEditCategoriesPageShowsTheDropdownAndDefaultsToCategory(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/categories").Body
	wantAll(t, body, `<option value="category" selected>Category</option>`, `>Subcategory</option>`, `>Levy Category</option>`, `>Levy Subcategory</option>`, `>Holding</option>`,
		`required maxlength="50"`, ">Add</button>")
	if strings.Contains(ta.get("/categories?list=nope").Body, `value="holding" selected`) {
		t.Error("an unknown list is not Category")
	}
}

func TestEachListIsShownSeparately(t *testing.T) {
	ta := newTestApp(t)
	lists := map[string]string{"category": "Alpha", "subcategory": "Bravo", "levy-category": "Charlie", "levy-subcategory": "Delta", "holding": "Echo"}
	for list, name := range lists {
		addEntry(ta, list, name)
	}
	for list, name := range lists {
		body := ta.get("/categories?list=" + list).Body
		wantAll(t, body, name, fmt.Sprintf(`value="%s" selected`, list))
		for other, otherName := range lists {
			if other != list && strings.Contains(body, otherName) {
				t.Errorf("list %s shows %q of %s", list, otherName, other)
			}
		}
	}
}

func TestEntriesAreSortedIgnoringCase(t *testing.T) {
	ta := newTestApp(t)
	for _, n := range []string{"banana", "Cherry", "apple"} {
		addEntry(ta, "category", n)
	}
	body := ta.get("/categories").Body
	if strings.Index(body, "apple") > strings.Index(body, "banana") || strings.Index(body, "banana") > strings.Index(body, "Cherry") {
		t.Error("entries are not sorted ignoring case")
	}
}

func TestEntriesPageFivePerPageWithThePager(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "category", 12)
	p1 := ta.get("/categories").Body
	wantAll(t, p1, "Entry 01", "Entry 05", "1 / 3", `href="/categories?list=category&amp;page=2" aria-label="Older"`, `<span class="arrow" aria-label="Newer">`)
	if strings.Contains(p1, "Entry 06") {
		t.Error("page 1 shows a sixth entry")
	}
	wantAll(t, ta.get("/categories?list=category&page=3").Body, "Entry 11", "Entry 12", "3 / 3")
	wantAll(t, ta.get("/categories?page=99").Body, "3 / 3")
}

func TestSwitchingListsStartsOnPageOne(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "category", 12)
	addEntries(ta, "holding", 7)
	// The dropdown submits only "list", so no page is carried over.
	wantAll(t, ta.get("/categories?list=holding").Body, "1 / 2", "Entry 01")
}

func TestEntriesShowTransactionCountsAndRepeatingOnlyWhenUsed(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Unused")
	ta.post("/transactions", expenseForm("2026-09-01", "3", "One", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", expenseForm("2026-09-02", "3", "Two", "new", "Food", "new", "Cafe"))
	body := ta.get("/categories").Body
	wantAll(t, body, "0 Trans.</span>", "2 Trans.</span>")
	if strings.Contains(body, "Rep.") {
		t.Error("Rep. shown without Repeating entries")
	}
}

func TestCountsIncludeRepeating(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-09-01", "3", "One", "new", "Food", "new", "Cafe"))
	// The helper's Repeating row uses a Category named "Cat", with no Transactions.
	mustSave(t, ta, repeatingForm(repRow("expense", "5", "Rent")))
	body := ta.get("/categories").Body
	wantAll(t, body, "1 Trans.</span>", "0 Trans. + 1 Rep.</span>")
	// A Repeating entry using an entry that also has Transactions shows both counts.
	mustSave(t, ta, repeatingForm([7]string{"expense", "5", "Rent", "1", "", "1", ""}))
	wantAll(t, ta.get("/categories").Body, "1 Trans. + 1 Rep.</span>")
}

func TestAddCreatesTheEntryAndLandsOnItsPage(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "category", 12)
	r := addEntry(ta, "category", "Entry 075")
	if r.Status != 303 {
		t.Fatalf("status = %d", r.Status)
	}
	if want := "/categories?list=category&page=2&added=new"; r.Location != want {
		t.Errorf("Location = %q, want %q", r.Location, want)
	}
	body := ta.follow(r).Body
	wantAll(t, body, "Entry 075", ">Added!</button>", "2 / 3")
	if strings.Contains(ta.get("/categories?list=category&page=2").Body, "Added!") {
		t.Error("the flag is not one-off")
	}
}

func TestAddedFlagIsRemovedFromTheURLAndResetAfterTwoSeconds(t *testing.T) {
	ta := newTestApp(t)
	body := ta.follow(addEntry(ta, "category", "Food")).Body
	wantAll(t, body, "history.replaceState", "searchParams.delete('added')", "2000")
	if strings.Contains(ta.get("/categories").Body, "replaceState") {
		t.Error("script present without the flag")
	}
}

func TestAddTrimsAndIgnoresBlankNames(t *testing.T) {
	ta := newTestApp(t)
	body := ta.follow(addEntry(ta, "category", "  Food \t")).Body
	wantAll(t, body, "<span>Food</span>")
	r := addEntry(ta, "category", "   ")
	if strings.Contains(r.Location, "added") {
		t.Errorf("blank name flagged: %s", r.Location)
	}
	if n := strings.Count(ta.follow(r).Body, "<li>"); n != 1 {
		t.Errorf("entries = %d, want 1", n)
	}
}

func TestAddExistingNameIgnoringCaseCreatesNothingAndLandsOnIt(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "category", 7)
	addEntry(ta, "category", "Zebra")
	r := addEntry(ta, "category", "zEBRA")
	if want := "/categories?list=category&page=2&added=exists"; r.Location != want {
		t.Errorf("Location = %q, want %q", r.Location, want)
	}
	body := ta.follow(r).Body
	wantAll(t, body, ">Exists!</button>", "Zebra", "2 / 2")
	if strings.Contains(body, "zEBRA") || strings.Count(body, "<li>") != 3 {
		t.Error("a duplicate was created")
	}
}

func TestTheSameNameMayExistInDifferentLists(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Other")
	r := addEntry(ta, "levy-category", "Other")
	if !strings.HasSuffix(r.Location, "added=new") {
		t.Errorf("Location = %q, want a new entry", r.Location)
	}
	wantAll(t, ta.get("/categories?list=levy-category").Body, "Other")
}

func TestAddRejectsTooLongNamesAndUsesTheDefaultListWhenUnknown(t *testing.T) {
	ta := newTestApp(t)
	if r := addEntry(ta, "category", strings.Repeat("x", 51)); r.Status != 400 {
		t.Errorf("51 characters: status %d, want 400", r.Status)
	}
	if r := addEntry(ta, "category", strings.Repeat("ä", 50)); r.Status != 303 {
		t.Errorf("50 characters: status %d, want 303", r.Status)
	}
}

func TestCategoriesPageEscapesNames(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "<script>x</script>")
	if strings.Contains(ta.get("/categories").Body, "<script>x") {
		t.Error("name is not escaped")
	}
}
