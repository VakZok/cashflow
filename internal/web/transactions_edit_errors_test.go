package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// applyButton is the label of the Apply button of an edit row.
func applyButton(t *testing.T, row string) string {
	t.Helper()
	m := regexp.MustCompile(`<button class="primary" type="submit"[^>]* data-resting="Apply"[^>]*>([^<]*)</button>`).FindStringSubmatch(row)
	if m == nil {
		t.Fatalf("no Apply button in:\n%s", row)
	}
	return m[1]
}

func TestAClearedDescriptionIsHighlightedAndApplySaysMissingFieldsOnTheSameFilteredPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)

	r := ta.post("/transactions/5", changeForm(ta, "2026-09-05", "", "transactions:2?search=Item"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	row := editRow(r.Body, "5")
	if got := strings.Join(highlighted(row), ","); got != "edit-description" {
		t.Errorf("highlighted %q, want edit-description", got)
	}
	if b := applyButton(t, row); b != "Missing fields" {
		t.Errorf("Apply reads %q, want Missing fields", b)
	}
	// Disarming after an "Approve" tap goes back to Apply, not to the message.
	wantAll(t, row, `data-arm data-label="Apply"`)
	// The row stays open in place, on the same filtered page.
	wantAll(t, row, `action="/transactions/5#tx-5"`, `name="return" value="transactions:2?search=Item"`)
	wantAll(t, r.Body, "2 / 3", `value="Item"`)
}

func TestAClearedDateAndAnUnnamedNewCategoryAreHighlighted(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 2)
	f := changeForm(ta, "", "Item 01", "transactions:1")
	f.Set("category", "new")
	r := ta.post("/transactions/1", f)
	row := editRow(r.Body, "1")
	if got := strings.Join(highlighted(row), ","); got != "edit-date,edit-new_category" {
		t.Errorf("highlighted %q, want edit-date and edit-new_category", got)
	}
	if b := applyButton(t, row); b != "Missing fields" {
		t.Errorf("Apply reads %q", b)
	}
}

func TestAnUnselectedSubcategoryIsHighlighted(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 2)
	f := changeForm(ta, "2026-09-01", "Item 01", "transactions:1")
	f.Set("subcategory", "")
	row := editRow(ta.post("/transactions/1", f).Body, "1")
	if got := strings.Join(highlighted(row), ","); got != "edit-subcategory" {
		t.Errorf("highlighted %q, want edit-subcategory", got)
	}
	if b := applyButton(t, row); b != "Missing fields" {
		t.Errorf("Apply reads %q", b)
	}
}

func TestEditingARowIntoADuplicateShowsAlreadyExistsOnApply(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 12)
	r := ta.post("/transactions/5", changeForm(ta, "2026-09-06", "item 06", "transactions:2"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	row := editRow(r.Body, "5")
	if b := applyButton(t, row); b != "Already exists" {
		t.Errorf("Apply reads %q, want Already exists", b)
	}
	if len(highlighted(row)) != 0 {
		t.Errorf("a Duplicate highlights %v", highlighted(row))
	}
}

func TestAnInvalidAmountInAnEditRowShowsTheBubbleAndLeavesApplyAlone(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 2)
	f := changeForm(ta, "2026-09-01", "Item 01", "transactions:1")
	f.Set("amount", "0")
	row := editRow(ta.post("/transactions/1", f).Body, "1")
	wantAll(t, row, `<span class="bubble" role="alert">1 to 100.000.000</span>`)
	if b := applyButton(t, row); b != "Apply" {
		t.Errorf("Apply reads %q, want Apply", b)
	}
	if strings.Contains(editRow(ta.get("/transactions?edit=1").Body, "1"), `class="bubble"`) {
		t.Error("a freshly opened edit row shows the bubble")
	}
}

func TestAHandCraftedEditShowsInvalidInputOnApply(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 2)
	f := changeForm(ta, "2026-09-01", "Item 01", "transactions:1")
	f.Set("type", "bogus")
	row := editRow(ta.post("/transactions/1", f).Body, "1")
	if b := applyButton(t, row); b != "Invalid input" {
		t.Errorf("Apply reads %q, want Invalid input", b)
	}
}

func TestNoErrorBoxIsLeftOnTheTransactionsPage(t *testing.T) {
	ta := newTestApp(t)
	addTransactions(ta, 2)
	f := changeForm(ta, "", "", "transactions:1")
	f.Set("amount", "0")
	f.Set("type", "bogus")
	body := ta.post("/transactions/1", f).Body
	for _, gone := range []string{`class="errors"`, `.errors`, "is required", "Amount must be"} {
		if strings.Contains(body, gone) {
			t.Errorf("the page still has %q", gone)
		}
	}
	if b := applyButton(t, editRow(ta.get("/transactions?edit=1").Body, "1")); b != "Apply" {
		t.Errorf("a freshly opened row's Apply reads %q", b)
	}
}
