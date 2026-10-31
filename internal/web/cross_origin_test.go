package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// newCategory is the form that adds a category with the given name.
func newCategory(name string) url.Values {
	return url.Values{"list": {"category"}, "name": {name}}
}

// fetchSite is the request header a browser sends for a request from site.
func fetchSite(site string) http.Header {
	return http.Header{"Sec-Fetch-Site": {site}}
}

func TestACrossSiteWriteIsRejectedAndChangesNothing(t *testing.T) {
	ta := newTestApp(t)
	r := ta.postWithHeader("/categories", newCategory("Forged"), fetchSite("cross-site"))
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
		"write":               ta.post("/categories", newCategory("Groceries")),
		"rejected cross-site": ta.postWithHeader("/categories", newCategory("Forged"), fetchSite("cross-site")),
	}
	for name, r := range responses {
		if got := r.Header.Get("Content-Security-Policy"); got != "frame-ancestors 'none'" {
			t.Errorf("%s: Content-Security-Policy = %q, want frame-ancestors 'none'", name, got)
		}
	}
}

func TestAWriteNotFromAnotherSiteSucceeds(t *testing.T) {
	for name, header := range map[string]http.Header{
		"same-origin":             fetchSite("same-origin"),
		"no browser fetch header": nil,
	} {
		t.Run(name, func(t *testing.T) {
			ta := newTestApp(t)
			r := ta.postWithHeader("/categories", newCategory("Groceries"), header)
			if r.Status != http.StatusSeeOther {
				t.Fatalf("status = %d, want 303", r.Status)
			}
			if !strings.Contains(ta.get("/categories").Body, "Groceries") {
				t.Error("the write did not add the category")
			}
		})
	}
}
