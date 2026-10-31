package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// postFrom POSTs a form as a browser would, sending the given Sec-Fetch-Site.
func (ta *testApp) postFrom(site, path string, form url.Values) response {
	ta.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", site)
	return ta.do(req)
}

func TestACrossSiteWriteIsRejectedAndChangesNothing(t *testing.T) {
	ta := newTestApp(t)
	r := ta.postFrom("cross-site", "/categories", url.Values{"list": {"category"}, "name": {"Forged"}})
	if r.Status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", r.Status)
	}
	if strings.Contains(ta.get("/categories").Body, "Forged") {
		t.Error("a cross-site write added a category")
	}
}

func TestNoResponseCanBeFramed(t *testing.T) {
	ta := newTestApp(t)
	responses := map[string]response{
		"page":                ta.get("/transactions"),
		"static file":         ta.get("/static/chart.umd.js"),
		"not found":           ta.get("/nope"),
		"write":               ta.post("/categories", url.Values{"list": {"category"}, "name": {"Groceries"}}),
		"rejected cross-site": ta.postFrom("cross-site", "/categories", url.Values{"list": {"category"}, "name": {"Forged"}}),
	}
	for name, r := range responses {
		if got := r.Header.Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
			t.Errorf("%s: Content-Security-Policy = %q, want frame-ancestors 'none'", name, got)
		}
	}
}

func TestASameOriginWriteSucceeds(t *testing.T) {
	ta := newTestApp(t)
	r := ta.postFrom("same-origin", "/categories", url.Values{"list": {"category"}, "name": {"Groceries"}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", r.Status)
	}
	if !strings.Contains(ta.get("/categories").Body, "Groceries") {
		t.Error("a same-origin write did not add the category")
	}
}

func TestAWriteWithoutBrowserFetchHeadersSucceeds(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/categories", url.Values{"list": {"category"}, "name": {"Groceries"}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", r.Status)
	}
	if !strings.Contains(ta.get("/categories").Body, "Groceries") {
		t.Error("a write without fetch headers did not add the category")
	}
}
