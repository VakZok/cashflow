package web_test

import (
	"regexp"
	"strings"
	"testing"
)

func TestTheAddFieldIsFilledFromTheAddressInTheSameList(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/categories?list=holding&page=1&a_name=Pet").Body
	wantAll(t, body, `name="name"`, `value="Pet"`, `<option value="holding" selected>`)
	if m := regexp.MustCompile(`<input type="text" name="name"[^>]*>`).FindString(body); !strings.Contains(m, `value="Pet"`) {
		t.Errorf("the add field is not filled: %q", m)
	}
	// Nothing carried: the field is empty.
	if strings.Contains(ta.get("/categories?list=holding").Body, `value="Pet"`) {
		t.Error("Pet comes back without being carried")
	}
}

func TestAnOpenRenameIsFilledWithTheCarriedNameInsteadOfTheStoredOne(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	row := openRow(ta.get("/categories?list=category&page=1&edit=1&e_name=Groceries").Body)
	wantAll(t, row, `value="Groceries"`)
	if strings.Contains(row, `value="Food"`) {
		t.Error("the stored name is shown")
	}
	// Discard goes back to the list without the carried name.
	wantAll(t, row, `href="/categories?list=category&amp;page=1"`)
}

func TestACarriedRenameOfADeletedEntryShowsNoRename(t *testing.T) {
	ta := newTestApp(t)
	res := ta.get("/categories?list=category&page=1&edit=42&e_name=Groceries")
	if res.Status != 200 || openRow(res.Body) != "" || strings.Contains(res.Body, "Groceries") {
		t.Errorf("status %d, body:\n%s", res.Status, res.Body)
	}
}

func TestTheAddAndRenameFormsAreCarried(t *testing.T) {
	ta := newTestApp(t)
	addEntry(ta, "category", "Food")
	wantAll(t, ta.get("/categories").Body, `<form class="add" method="post" action="/categories" data-carry="a_">`)
	wantAll(t, ta.get("/categories?edit=1").Body, `data-carry="e_"`)
}

func TestAddingAfterReturningClearsTheFieldAndKeepsTheList(t *testing.T) {
	ta := newTestApp(t)
	res := addEntry(ta, "holding", "Pet")
	if res.Status != 303 || strings.Contains(res.Location, "a_name") || !strings.Contains(res.Location, "list=holding") {
		t.Errorf("redirect %d %q", res.Status, res.Location)
	}
	body := ta.get(res.Location).Body
	if strings.Contains(regexp.MustCompile(`<input type="text" name="name"[^>]*>`).FindString(body), `value="Pet"`) {
		t.Error("the add field is not cleared")
	}
	wantAll(t, body, `<option value="holding" selected>`)
}
