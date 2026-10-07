package web_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
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

	wantAll(t, ta.mustReport("/report?period=year&date=2026&amp;type=expense&amp;group=category"), "+€800", "Income €1.000 · Expense €200")
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
		"", "?period=week&date=2026-10&amp;type=expense&amp;group=category", "?period=month&date=garbage", "?period=month&date=2026-13",
		"?period=year&date=2026-10&amp;type=expense&amp;group=category", "?period=month&date=2026&amp;type=expense&amp;group=category", "?date=1999", "?period=year&date=99999", "?period=month",
	} {
		wantAll(t, ta.mustReport("/report"+q), "+€100", "2026-10")
	}
}

func TestReportNavigationLinks(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.mustReport("/report?period=month&date=2026-01"),
		`href="/report?period=month&amp;date=2025-12&amp;type=expense&amp;group=category"`, `href="/report?period=month&amp;date=2026-02&amp;type=expense&amp;group=category"`,
		`href="/report?period=year&amp;date=2026&amp;type=expense&amp;group=category"`, // toggle keeps the year
	)
	wantAll(t, ta.mustReport("/report?period=year&date=2026&amp;type=expense&amp;group=category"),
		`href="/report?period=year&amp;date=2025&amp;type=expense&amp;group=category"`, `href="/report?period=year&amp;date=2027&amp;type=expense&amp;group=category"`,
		`href="/report?period=month&amp;date=2026-10&amp;type=expense&amp;group=category"`, // current year -> current month
	)
	wantAll(t, ta.mustReport("/report?period=year&date=2024"),
		`href="/report?period=month&amp;date=2024-01&amp;type=expense&amp;group=category"`, // other year -> January
	)
}

func TestReportDirectPicker(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.mustReport("/report?period=month&date=2026-03"), `method="get" action="/report"`, `type="month"`, `value="2026-03"`)
	wantAll(t, ta.mustReport("/report?period=year&date=2026&amp;type=expense&amp;group=category"), `type="number"`, `name="date"`, `value="2026"`)
}

func TestReportAndAddPageLinkToEachOther(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/").Body, `href="/report"`)
	wantAll(t, ta.mustReport("/report"), `href="/"`)
}

// seedIn records a Transaction with the given Category and Subcategory.
func (ta *testApp) seedIn(typ, date, amount, category, subcategory string) {
	ta.t.Helper()
	f := expenseForm(date, amount, "x", "new", category, "new", subcategory)
	f.Set("type", typ)
	if r := ta.post("/transactions", f); r.Status != http.StatusSeeOther {
		ta.t.Fatalf("seed = %d\n%s", r.Status, r.Body)
	}
}

var chartJSON = regexp.MustCompile(`(?s)<script type="application/json" id="chart-data">(.*?)</script>`)

func (ta *testApp) chart(path string) (labels []string, amounts []int64) {
	ta.t.Helper()
	m := chartJSON.FindStringSubmatch(ta.mustReport(path))
	if m == nil {
		ta.t.Fatalf("no chart data on %s", path)
	}
	var d struct {
		Labels  []string `json:"labels"`
		Amounts []int64  `json:"amounts"`
	}
	if err := json.Unmarshal([]byte(m[1]), &d); err != nil {
		ta.t.Fatalf("chart data %q: %v", m[1], err)
	}
	return d.Labels, d.Amounts
}

func wantChart(t *testing.T, labels []string, amounts []int64, wantLabels []string, wantAmounts []int64) {
	t.Helper()
	if strings.Join(labels, "|") != strings.Join(wantLabels, "|") || fmt.Sprint(amounts) != fmt.Sprint(wantAmounts) {
		t.Errorf("chart = %v %v, want %v %v", labels, amounts, wantLabels, wantAmounts)
	}
}

func TestReportChartTypeAndGroupCombinations(t *testing.T) {
	ta := newTestApp(t)
	ta.seedIn("expense", "2026-10-02", "300", "Food", "Restaurant")
	ta.seedIn("expense", "2026-10-03", "100", "Food", "Groceries")
	ta.seedIn("expense", "2026-10-04", "200", "Rent", "Groceries")
	ta.seedIn("income", "2026-10-05", "900", "Salary", "Work")
	ta.seedIn("income", "2026-10-06", "100", "Gift", "Work")
	ta.seedIn("expense", "2026-09-06", "999", "Elsewhere", "Elsewhere") // other Period

	cases := []struct {
		query   string
		labels  []string
		amounts []int64
	}{
		{"", []string{"Food", "Rent"}, []int64{400, 200}}, // defaults: expense by category
		{"&type=expense&group=category", []string{"Food", "Rent"}, []int64{400, 200}},
		{"&type=expense&group=subcategory", []string{"Groceries", "Restaurant"}, []int64{300, 300}},
		{"&type=income&group=category", []string{"Salary", "Gift"}, []int64{900, 100}},
		{"&type=income&group=subcategory", []string{"Work"}, []int64{1000}},
		{"&type=bogus&group=bogus", []string{"Food", "Rent"}, []int64{400, 200}},
	}
	for _, c := range cases {
		l, a := ta.chart("/report?period=month&date=2026-10" + c.query)
		wantChart(t, l, a, c.labels, c.amounts)
	}
}

func TestReportChartGroupsSmallSlicesAsOther(t *testing.T) {
	ta := newTestApp(t)
	ta.seedIn("expense", "2026-10-02", "940", "Big", "S")
	ta.seedIn("expense", "2026-10-03", "30", "Exactly3", "S") // 3% stays
	ta.seedIn("expense", "2026-10-04", "20", "Small1", "S")
	ta.seedIn("expense", "2026-10-05", "9", "Small2", "S")
	ta.seedIn("expense", "2026-10-06", "1", "Small3", "S")

	body := ta.mustReport("/report?period=month&date=2026-10")
	l, a := ta.chart("/report?period=month&date=2026-10")
	wantChart(t, l, a, []string{"Big", "Exactly3", "Other"}, []int64{940, 30, 30})
	wantAll(t, body, ">Big<", ">€940<", ">94.0%<", ">Other<", ">€30<", ">3.0%<")
}

func TestReportChartExcludesInvestmentsAndDivestments(t *testing.T) {
	ta := newTestApp(t)
	ta.seedIn("expense", "2026-10-02", "100", "Food", "S")
	ta.post("/transactions", holdingForm("investment", "2026-10-03", "400", "new", "ETF"))
	ta.post("/transactions", holdingForm("divestment", "2026-10-04", "300", "new", "ETF"))
	for _, typ := range []string{"expense", "income"} {
		l, a := ta.chart("/report?type=" + typ)
		if typ == "expense" {
			wantChart(t, l, a, []string{"Food"}, []int64{100})
		} else {
			wantChart(t, l, a, []string{}, []int64{})
		}
	}
}

func TestReportChartEmptyPeriod(t *testing.T) {
	ta := newTestApp(t)
	l, a := ta.chart("/report?period=year&date=2020")
	wantChart(t, l, a, []string{}, []int64{})
}

func TestReportChartSelectionPersistsAcrossPeriodNavigation(t *testing.T) {
	ta := newTestApp(t)
	body := ta.mustReport("/report?period=month&date=2026-05&type=income&group=subcategory")
	wantAll(t, body,
		`date=2026-04&amp;type=income&amp;group=subcategory"`, `date=2026-06&amp;type=income&amp;group=subcategory"`,
		`period=year&amp;date=2026&amp;type=income&amp;group=subcategory"`,
		`name="type" value="income"`, `name="group" value="subcategory"`, // direct picker
		`<option value="income" selected>`, `<option value="subcategory" selected>`,
	)
}
