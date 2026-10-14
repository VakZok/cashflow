package web_test

import (
	"strings"
	"testing"
)

func TestEveryPageHasTheBottomNavWithItsOwnItemActive(t *testing.T) {
	ta := newTestApp(t)
	pages := []struct{ path, active string }{
		{"/", "Add transaction"},
		{"/repeating", "Repeating"},
		{"/report", "Report"},
		{"/report?period=year", "Report"},
	}
	labels := []string{"Repeating", "Add transaction", "Report"}
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
