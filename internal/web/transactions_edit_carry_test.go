package web_test

import (
	"strings"
	"testing"
)

func TestAnEditFormIsFilledFromTheAddressAndTheFilterIsKept(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	body := ta.get("/transactions?year=2026&month=10&page=1&edit=1&e_type=expense&e_date=2026-10-03&e_amount=99&e_description=Dinner&e_category=1&e_subcategory=1").Body
	if inputValue(t, body, "edit-amount") != "99" || inputValue(t, body, "edit-description") != "Dinner" || inputValue(t, body, "edit-date") != "2026-10-03" {
		t.Errorf("the changes are not carried:\n%s", editRow(body, "1"))
	}
	if got := selectedLabel(t, body, "edit-category"); got != "Food" {
		t.Errorf("category %q", got)
	}
	// The stored values of the row are not shown instead.
	if strings.Contains(editRow(body, "1"), `value="12"`) {
		t.Error("the stored amount is shown")
	}
	// Discard goes back to the list without the carried changes.
	wantAll(t, editRow(body, "1"), `href="/transactions?year=2026&amp;month=10&amp;page=1"`)
}

func TestACarriedTypeChangeBringsBackThatTypesFields(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	ta.post("/transactions", validForm("investment", "2026-10-03", "90", "ETF World"))
	body := ta.get("/transactions?edit=1&e_type=investment&e_date=2026-10-02&e_amount=12&e_holding=new&e_new_holding=Fund").Body
	if got := selectedLabel(t, body, "edit-type"); got != "Investment" {
		t.Errorf("type %q", got)
	}
	if got := selectedLabel(t, body, "edit-holding"); got != "new…" {
		t.Errorf("holding %q", got)
	}
	if inputValue(t, body, "edit-new_holding") != "Fund" || inputHidden(t, body, "edit-new_holding") {
		t.Error("the new… name is not filled and shown")
	}
}

func TestCarriedEditValuesThatNoLongerExistFallBackToEmpty(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", validForm("investment", "2026-10-03", "90", "ETF World"))
	body := ta.get("/transactions?edit=1&e_type=investment&e_date=2026-10-03&e_amount=90&e_holding=999").Body
	if got := selectedLabel(t, body, "edit-holding"); got != "" {
		t.Errorf("holding %q, want empty", got)
	}
	if h := highlighted(body); len(h) != 0 {
		t.Errorf("highlighted %v", h)
	}
	body = ta.get("/transactions?edit=1&e_type=investment&e_category=999&e_subcategory=abc").Body
	for _, id := range []string{"edit-category", "edit-subcategory"} {
		if got := selectedLabel(t, body, id); got != "" {
			t.Errorf("%s = %q, want empty", id, got)
		}
	}
	body = ta.get("/transactions?edit=1&e_type=bogus&e_amount=5").Body
	if got := selectedLabel(t, body, "edit-type"); got != "Investment" {
		t.Errorf("type %q, want the stored Investment", got)
	}
}

func TestACarriedEditOfADeletedTransactionShowsNoEdit(t *testing.T) {
	ta := newTestApp(t)
	res := ta.get("/transactions?edit=42&e_type=expense&e_amount=5")
	if res.Status != 200 || strings.Contains(res.Body, `class="editing"`) || strings.Contains(res.Body, `class="bubble"`) {
		t.Errorf("status %d, body:\n%s", res.Status, res.Body)
	}
}

func TestOnlyTheEditFormCarriesItsFieldsOnTheTransactionsPage(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-02", "12", "Lunch", "new", "Food", "new", "Cafe"))
	wantAll(t, ta.get("/transactions?edit=1").Body, `<form method="post" action="/transactions/1#tx-1" data-carry="e_">`)
	if body := ta.get("/transactions").Body; strings.Contains(body, "<form method=\"post\"") && strings.Contains(body, " data-carry") {
		t.Error("a form is carried without an edit")
	}
}
