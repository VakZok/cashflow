package web_test

import (
	"net/http"
	"strings"
	"testing"
)

// seed records an Income/Expense Transaction with a throwaway Category.
func (ta *testApp) seed(typ, date, amount string) {
	ta.t.Helper()
	f := validForm(typ, date, amount, "x")
	if r := ta.post("/transactions", f); r.Status != http.StatusSeeOther {
		ta.t.Fatalf("seed %s %s %s = %d\n%s", typ, date, amount, r.Status, r.Body)
	}
}

func (ta *testApp) mustReport(path string) string {
	ta.t.Helper()
	r := ta.get(path)
	if r.Status != 200 {
		ta.t.Fatalf("GET %s = %d, want 200", path, r.Status)
	}
	return r.Body
}

func wantAll(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("missing %q in:\n%s", w, body)
		}
	}
}

func TestReportDefaultsToCurrentMonthAndExcludesOtherPeriods(t *testing.T) {
	ta := newTestApp(t)
	ta.seed("income", "2026-10-01", "3000")
	ta.seed("expense", "2026-10-31", "1000")
	ta.seed("expense", "2026-09-30", "500") // outside
	ta.seed("income", "2026-11-01", "700")  // outside

	body := ta.mustReport("/report")
	wantAll(t, body, `class="net net-positive"`, "+€2.000", "Income €3.000 · Expense €1.000", "2026-10")
}

func TestReportNetSignAndColour(t *testing.T) {
	ta := newTestApp(t)
	ta.seed("income", "2026-08-10", "100")
	ta.seed("expense", "2026-08-11", "667")
	ta.seed("income", "2026-07-10", "50")
	ta.seed("expense", "2026-07-11", "50")

	wantAll(t, ta.mustReport("/report?period=month&date=2026-08"), `class="net net-negative"`, "−€567", "Income €100 · Expense €667")

	body := ta.mustReport("/report?period=month&date=2026-07")
	wantAll(t, body, `class="net net-zero"`, ">€0<")
	if strings.Contains(body, "+€0") || strings.Contains(body, "−€0") {
		t.Errorf("zero net must have no sign:\n%s", body)
	}
}

func TestReportYearPeriod(t *testing.T) {
	ta := newTestApp(t)
	ta.seed("income", "2025-01-01", "10")
	ta.seed("income", "2026-01-01", "1000")
	ta.seed("expense", "2026-12-31", "200")
	ta.seed("income", "2027-01-01", "5")

	wantAll(t, ta.mustReport("/report?period=year&date=2026"), "+€800", "Income €1.000 · Expense €200")
}

func TestReportIgnoresInvestmentsAndDivestments(t *testing.T) {
	ta := newTestApp(t)
	ta.seed("income", "2026-10-02", "100")
	if r := ta.post("/transactions", holdingForm("investment", "2026-10-03", "40", "new", "ETF")); r.Status != http.StatusSeeOther {
		t.Fatalf("invest = %d", r.Status)
	}
	if r := ta.post("/transactions", holdingForm("divestment", "2026-10-04", "30", "new", "ETF")); r.Status != http.StatusSeeOther {
		t.Fatalf("divest = %d", r.Status)
	}
	wantAll(t, ta.mustReport("/report"), "+€100", "Income €100 · Expense €0")
}

func TestReportEmptyPeriod(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.mustReport("/report"), `class="net net-zero"`, ">€0<", "Income €0 · Expense €0")
}

func TestReportInvalidParamsFallBackToCurrentMonth(t *testing.T) {
	ta := newTestApp(t)
	ta.seed("income", "2026-10-02", "100")
	for _, q := range []string{
		"", "?period=week&date=2026-10", "?period=month&date=garbage", "?period=month&date=2026-13",
		"?period=year&date=2026-10", "?period=month&date=2026", "?date=1999", "?period=year&date=99999", "?period=month",
	} {
		wantAll(t, ta.mustReport("/report"+q), "+€100", "2026-10")
	}
}

func TestReportNavigationLinks(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.mustReport("/report?period=month&date=2026-01"),
		`href="/report?period=month&amp;date=2025-12"`, `href="/report?period=month&amp;date=2026-02"`,
		`href="/report?period=year&amp;date=2026"`, // toggle keeps the year
	)
	wantAll(t, ta.mustReport("/report?period=year&date=2026"),
		`href="/report?period=year&amp;date=2025"`, `href="/report?period=year&amp;date=2027"`,
		`href="/report?period=month&amp;date=2026-10"`, // current year -> current month
	)
	wantAll(t, ta.mustReport("/report?period=year&date=2024"),
		`href="/report?period=month&amp;date=2024-01"`, // other year -> January
	)
}

func TestReportDirectPicker(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.mustReport("/report?period=month&date=2026-03"), `method="get" action="/report"`, `type="month"`, `value="2026-03"`)
	wantAll(t, ta.mustReport("/report?period=year&date=2026"), `type="number"`, `name="date"`, `value="2026"`)
}

func TestReportAndAddPageLinkToEachOther(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/").Body, `href="/report"`)
	wantAll(t, ta.mustReport("/report"), `href="/"`)
}
