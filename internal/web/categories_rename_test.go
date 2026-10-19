package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"
)

func renameEntry(ta *testApp, list, id, name string) response {
	ta.t.Helper()
	return ta.post("/categories/"+id, url.Values{"list": {list}, "name": {name}})
}

// openRow is the entry row being edited on the Edit Categories page.
func openRow(body string) string {
	return regexp.MustCompile(`(?s)<li class="editing">.*?</li>`).FindString(body)
}

func TestTappingAnEntryOpensItInlineWithTheRenameLayout(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	addEntry(ta, "category", "Rent")
	list := ta.get("/categories").Body
	wantAll(t, list, `href="/categories?list=category&amp;page=1&amp;edit=1"`, `href="/categories?list=category&amp;page=1&amp;edit=2"`)
	if openRow(list) != "" {
		t.Error("a row is open without being tapped")
	}

	body := ta.get("/categories?list=category&page=1&edit=1").Body
	row := openRow(body)
	wantAll(t, row, `action="/categories/1"`, `name="list" value="category"`, `name="name"`, `value="Food"`, `required`, `maxlength="50"`,
		"Delete", `href="/categories?list=category&amp;page=1"`, "Discard", "Apply")
	// Apply is first in the markup so Enter in the name presses it; CSS shows it last.
	order := []string{"Apply", "Delete", `name="name"`, "Discard"}
	for i := 1; i < len(order); i++ {
		if strings.Index(row, order[i-1]) >= strings.Index(row, order[i]) {
			t.Errorf("%q does not come before %q in the row:\n%s", order[i-1], order[i], row)
		}
	}
	if !strings.Contains(row, `data-arm`) || !strings.Contains(body, "function disarm") {
		t.Error("Apply is not a double-tap button")
	}
	if strings.Contains(body, `href="/categories?list=category&amp;page=1&amp;edit=1"`) || !strings.Contains(body, `edit=2"`) {
		t.Error("only the other row should stay a closed link")
	}
	// Discard is a plain link back: the page without the edit.
	if strings.Contains(ta.get("/categories?list=category&page=1").Body, `class="editing"`) {
		t.Error("Discard link leaves the row open")
	}
}

func TestEditingAnEntryNotOnThePageOrNotExistingOpensNothing(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "category", 7)
	// Entry 7 sits on page 2.
	if openRow(ta.get("/categories?list=category&page=1&edit=7").Body) != "" {
		t.Error("opened a row that is not on the page")
	}
	if openRow(ta.get("/categories?list=category&edit=99").Body) != "" {
		t.Error("opened a missing entry")
	}
}

func TestRenameChangesTheNameOnTransactionsAndRepeatingEntries(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-09-01", "3", "Lunch", "new", "Food", "new", "Cafe"))
	mustSave(t, ta, repeatingForm([7]string{"expense", "5", "Rent", "1", "", "1", ""}))
	r := renameEntry(ta, "category", "1", "Groceries")
	if r.Status != 303 || r.Location != "/categories?list=category&page=1" {
		t.Fatalf("status %d, Location %q", r.Status, r.Location)
	}
	wantAll(t, ta.follow(r).Body, "<span>Groceries</span>", "1 Trans. + 1 Rep.</span>")
	for _, path := range []string{"/transactions", "/", "/repeating"} {
		body := ta.get(path).Body
		wantAll(t, body, "Groceries")
		if strings.Contains(body, "Food") {
			t.Errorf("%s still shows the old name", path)
		}
	}
}

func TestACaseOnlyRenameGoesThrough(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "food")
	r := renameEntry(ta, "category", "1", "Food")
	if r.Status != 303 || strings.Contains(r.Location, "taken") {
		t.Fatalf("status %d, Location %q", r.Status, r.Location)
	}
	wantAll(t, ta.follow(r).Body, "<span>Food</span>")
}

func TestRenameIntoAnotherEntrysNameIsRefusedAndKeepsTheRowOpen(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	addEntry(ta, "category", "Rent")
	r := renameEntry(ta, "category", "1", "rENT")
	if r.Status != 303 {
		t.Fatalf("status %d", r.Status)
	}
	body := ta.follow(r).Body
	row := openRow(body)
	wantAll(t, row, `value="rENT"`, ">Taken</button>", `action="/categories/1"`)
	wantAll(t, body, "<span>Rent</span>", "history.replaceState", "2000")
	// Both entries are unchanged.
	closed := ta.get("/categories").Body
	wantAll(t, closed, "<span>Food</span>", "<span>Rent</span>")
	if strings.Contains(closed, "Taken") {
		t.Error("Taken is not one-off")
	}
}

func TestTakenRefusalKeepsTheListAndThePageOfTheEntry(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "holding", 7)
	// Entry 07 sits on page 2.
	body := ta.follow(renameEntry(ta, "holding", "7", "entry 01")).Body
	wantAll(t, body, `value="holding" selected`, "2 / 2", ">Taken</button>", `value="entry 01"`)
}

func TestRenamingToANameUsedOnlyInAnotherListIsAllowed(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	addEntry(ta, "subcategory", "Rent")
	r := renameEntry(ta, "category", "1", "Rent")
	if strings.Contains(r.Location, "taken") {
		t.Fatalf("refused: %q", r.Location)
	}
	wantAll(t, ta.follow(r).Body, "<span>Rent</span>")
	wantAll(t, ta.get("/categories?list=subcategory").Body, "<span>Rent</span>")
}

func TestRenameTrimsAndIgnoresBlankNames(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	wantAll(t, ta.follow(renameEntry(ta, "category", "1", "  Meals \t")).Body, "<span>Meals</span>")
	r := renameEntry(ta, "category", "1", "   ")
	if r.Status != 303 || strings.Contains(r.Location, "taken") {
		t.Fatalf("status %d, Location %q", r.Status, r.Location)
	}
	wantAll(t, ta.follow(r).Body, "Meals")
}

func TestRenameRejectsTooLongNamesAndMissingEntries(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	if r := renameEntry(ta, "category", "1", strings.Repeat("x", 51)); r.Status != 400 {
		t.Errorf("51 characters: status %d, want 400", r.Status)
	}
	if r := renameEntry(ta, "category", "1", strings.Repeat("ä", 50)); r.Status != 303 {
		t.Errorf("50 characters: status %d, want 303", r.Status)
	}
	if r := renameEntry(ta, "category", "99", "Other"); r.Status != 404 {
		t.Errorf("missing entry: status %d, want 404", r.Status)
	}
	if r := renameEntry(ta, "category", "abc", "Other"); r.Status != 404 {
		t.Errorf("bad id: status %d, want 404", r.Status)
	}
}

func TestAfterARenameTheUserLandsOnThePageWhereTheEntrySorts(t *testing.T) {
	ta := newTestApp(t)
	addEntries(ta, "levy-category", 12)
	// Entry 01 moves behind Entry 12, onto page 3.
	r := renameEntry(ta, "levy-category", "1", "Zebra")
	if want := "/categories?list=levy-category&page=3"; r.Location != want {
		t.Errorf("Location = %q, want %q", r.Location, want)
	}
	wantAll(t, ta.follow(r).Body, "Zebra", `value="levy-category" selected`, "3 / 3")
}

func TestTheTransactionEditRowsButtonIsLabelledApply(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	row := editRow(ta.get("/transactions?edit=1").Body, "1")
	wantAll(t, row, ">Apply</button>")
	if strings.Contains(row, "Change") {
		t.Error("the edit row still labels the button Change")
	}
}
