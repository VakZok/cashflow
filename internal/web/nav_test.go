package web_test

import (
	"strings"
	"testing"
)

func TestEveryPageHasTheBottomNavWithItsOwnItemActive(t *testing.T) {
	ta := newTestApp(t)
	pages := []struct{ path, active string }{
		{"/", "Add transaction"},
		{"/transactions", "Transactions"},
		{"/transactions?page=9", "Transactions"},
		{"/repeating", "Repeating"},
		{"/report", "Report"},
		{"/report?period=year", "Report"},
		{"/categories", "Edit Categories"},
	}
	labels := []string{"Repeating", "Add transaction", "Transactions", "Report", "Edit Categories"}
	for _, p := range pages {
		t.Run(p.path, func(t *testing.T) {
			body := ta.get(p.path).Body
			if !strings.Contains(body, `<nav class="bottom-nav"`) {
				t.Fatal("page has no bottom nav")
			}
			if n := strings.Count(body, `aria-current="page"><svg`); n != 1 {
				t.Fatalf("want exactly one active item, got %d", n)
			}
			for _, l := range labels {
				active := strings.Contains(body, `aria-label="`+l+`" aria-current="page"`)
				if active != (l == p.active) {
					t.Errorf("%q active = %v, want %v", l, active, l == p.active)
				}
			}
		})
	}
}

func TestPagesNoLongerCarryTextNavLinks(t *testing.T) {
	ta := newTestApp(t)
	for _, path := range []string{"/", "/repeating", "/report"} {
		body := ta.get(path).Body
		for _, old := range []string{`<a href="/report">Report</a>`, `<a href="/repeating">Repeating</a>`, `<a href="/">Home</a>`, `<a href="/">Add transaction</a>`} {
			if strings.Contains(body, old) {
				t.Errorf("%s still has text link %s", path, old)
			}
		}
	}
}

func TestNavHidesOnlyForTextFieldsAndFocusIsShortLived(t *testing.T) {
	ta := newTestApp(t)
	for _, path := range []string{"/", "/transactions", "/repeating", "/report"} {
		body := ta.get(path).Body
		if strings.Contains(body, "select:focus") {
			t.Errorf("%s hides the nav while a dropdown has focus", path)
		}
		for _, want := range []string{`input:is([type="text"], [type="search"], [type="number"]):focus`, `addEventListener('change'`, `addEventListener('pointerdown'`, "safe-area-inset-bottom"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s is missing %q", path, want)
			}
		}
	}
}

func TestPagesThatKeepStateMarkTheirNavItem(t *testing.T) {
	ta := newTestApp(t)
	for _, path := range []string{"/", "/transactions", "/repeating", "/report", "/categories"} {
		body := ta.get(path).Body
		for _, want := range []string{`<a href="/" data-page="add"`, `<a href="/transactions" data-page="transactions"`, `<a href="/report" data-page="report"`, `<a href="/categories" data-page="categories"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s is missing %q", path, want)
			}
		}
		for _, none := range []string{`<a href="/repeating" data-page`} {
			if strings.Contains(body, none) {
				t.Errorf("%s keeps state for %q", path, none)
			}
		}
	}
}

func TestOnlyTheAddFormIsCarriedAcrossPages(t *testing.T) {
	ta := newTestApp(t)
	if n := strings.Count(ta.get("/").Body, "data-keep-scroll data-carry>"); n != 1 {
		t.Errorf("the Add page has %d carried forms, want 1", n)
	}
	if !strings.Contains(ta.get("/").Body, `data-page="add" aria-label="Add transaction"`) {
		t.Error("the Add icon does not remember its page")
	}
	for _, path := range []string{"/transactions", "/repeating", "/report", "/categories"} {
		if strings.Contains(ta.get(path).Body, "<form method=\"post\" action=\"/transactions\" data-keep-scroll data-carry>") {
			t.Errorf("%s carries the Add form", path)
		}
	}
}
