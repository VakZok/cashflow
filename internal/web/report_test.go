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
		"?period=year&date=2026-10&amp;type=expense&amp;group=category", "?period=month&date=2026&amp;type=expense&amp;group=category", "?date=1999", "?period=year&date=0000", "?period=month&date=0000-12", "?period=year&date=99999", "?period=month",
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

func TestReportPeriodArrowsUseThePagerChevrons(t *testing.T) {
	ta := newTestApp(t)
	body := ta.mustReport("/report?period=month&date=2026-01")
	wantAll(t, body,
		`aria-label="Previous"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 18-6-6 6-6"/></svg></a>`,
		`aria-label="Next"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18 6-6-6-6"/></svg></a>`,
	)
	if strings.ContainsAny(body, "‹›") {
		t.Errorf("Report still has text arrows:\n%s", body)
	}
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

type chartPayload struct {
	Labels  []string `json:"labels"`
	Amounts []int64  `json:"amounts"`
	Colours []string `json:"colours"`
}

func (ta *testApp) chartData(path string) chartPayload {
	ta.t.Helper()
	m := chartJSON.FindStringSubmatch(ta.mustReport(path))
	if m == nil {
		ta.t.Fatalf("no chart data on %s", path)
	}
	var d chartPayload
	if err := json.Unmarshal([]byte(m[1]), &d); err != nil {
		ta.t.Fatalf("chart data %q: %v", m[1], err)
	}
	return d
}

func (ta *testApp) chart(path string) (labels []string, amounts []int64) {
	ta.t.Helper()
	d := ta.chartData(path)
	return d.Labels, d.Amounts
}

func (ta *testApp) chartColours(path string) []string {
	ta.t.Helper()
	return ta.chartData(path).Colours
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

// seedFold records, in October 2026, one big Expense, one at exactly 3% (which
// stays named) and three below 3% (grouped as Other).
func (ta *testApp) seedFold() {
	ta.t.Helper()
	ta.seedIn("expense", "2026-10-02", "940", "Big", "S")
	ta.seedIn("expense", "2026-10-03", "30", "Exactly3", "S")
	ta.seedIn("expense", "2026-10-04", "20", "Small1", "S")
	ta.seedIn("expense", "2026-10-05", "9", "Small2", "S")
	ta.seedIn("expense", "2026-10-06", "1", "Small3", "S")
}

// seedTen records ten equal Expenses in October 2026, filling the palette.
func (ta *testApp) seedTen() {
	ta.t.Helper()
	for i := range 10 {
		ta.seedIn("expense", fmt.Sprintf("2026-10-%02d", i+1), "1000", fmt.Sprintf("Cat%d", i), "S")
	}
}

func TestReportChartGroupsSmallSlicesAsOther(t *testing.T) {
	ta := newTestApp(t)
	ta.seedFold()

	l, a := ta.chart("/report?period=month&date=2026-10")
	wantChart(t, l, a, []string{"Big", "Exactly3", "Other"}, []int64{940, 30, 30})
}

var (
	breakdownTable = regexp.MustCompile(`(?s)<table class="breakdown">(.*?)</table>`)
	tableRow       = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
	tableCell      = regexp.MustCompile(`(?s)<td>(.*?)</td>`)
	htmlTag        = regexp.MustCompile(`<[^>]*>`)
)

// breakdownHTML returns the inside of the breakdown table.
func (ta *testApp) breakdownHTML(path string) string {
	ta.t.Helper()
	m := breakdownTable.FindStringSubmatch(ta.mustReport(path))
	if m == nil {
		ta.t.Fatalf("no breakdown table on %s", path)
	}
	return m[1]
}

// table returns the breakdown table's rows as "Label Amount Share".
func (ta *testApp) table(path string) []string {
	ta.t.Helper()
	rows := []string{}
	for _, r := range tableRow.FindAllStringSubmatch(ta.breakdownHTML(path), -1) {
		var cells []string
		for _, c := range tableCell.FindAllStringSubmatch(r[1], -1) {
			cells = append(cells, strings.TrimSpace(htmlTag.ReplaceAllString(c[1], "")))
		}
		rows = append(rows, strings.Join(cells, " "))
	}
	return rows
}

func TestReportTableListsEveryItem(t *testing.T) {
	ta := newTestApp(t)
	ta.seedFold()

	got := ta.table("/report?period=month&date=2026-10")
	want := []string{"Big €940 94.0%", "Exactly3 €30 3.0%", "Small1 €20 2.0%", "Small2 €9 0.9%", "Small3 €1 0.1%"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("table = %q, want %q", got, want)
	}
}

// otherGrey is the colour reserved for the "Other" slice.
const otherGrey = "#bab0ac"

func TestReportChartColoursNamedSlicesDistinctlyAndOtherGrey(t *testing.T) {
	ta := newTestApp(t)
	ta.seedTen()
	ta.seedIn("expense", "2026-10-20", "1", "Tiny1", "S")
	ta.seedIn("expense", "2026-10-21", "1", "Tiny2", "S")

	c := ta.chartColours("/report?period=month&date=2026-10")
	if len(c) != 11 {
		t.Fatalf("colours = %q, want 10 named + Other", c)
	}
	if c[10] != otherGrey {
		t.Errorf("Other colour = %q, want %q", c[10], otherGrey)
	}
	seen := map[string]bool{otherGrey: true}
	for _, col := range c[:10] {
		if seen[col] {
			t.Errorf("named colour %q repeats or is the Other grey: %q", col, c)
		}
		seen[col] = true
	}
}

var swatch = regexp.MustCompile(`<span class="swatch" style="background:\s*([^";]*)"`)

// tableColours returns the breakdown table's swatch colours, row by row.
func (ta *testApp) tableColours(path string) []string {
	ta.t.Helper()
	out := []string{}
	for _, s := range swatch.FindAllStringSubmatch(ta.breakdownHTML(path), -1) {
		out = append(out, s[1])
	}
	return out
}

func TestReportTableSwatchesMatchTheirSlice(t *testing.T) {
	ta := newTestApp(t)
	ta.seedFold()

	path := "/report?period=month&date=2026-10"
	chart := ta.chartColours(path) // Big, Exactly3, Other
	want := []string{chart[0], chart[1], otherGrey, otherGrey, otherGrey}
	if got := ta.tableColours(path); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("table colours = %q, want %q", got, want)
	}
}

func TestReportChartFirstAndLastSlicesNeverShareAColour(t *testing.T) {
	ta := newTestApp(t)
	ta.seedTen()
	ta.seedIn("expense", "2026-10-20", "1", "Lone", "S") // keeps its name: 11 named, no Other

	c := ta.chartColours("/report?period=month&date=2026-10")
	if len(c) != 11 {
		t.Fatalf("colours = %q, want 11 named", c)
	}
	if c[10] == c[0] || c[10] == c[9] {
		t.Errorf("last colour %q touches an equal neighbour: %q", c[10], c)
	}
}

func TestReportChartPutsOtherLastEvenWhenLargest(t *testing.T) {
	ta := newTestApp(t)
	ta.seedIn("expense", "2026-10-01", "30", "Named", "S")
	for i := range 20 {
		ta.seedIn("expense", fmt.Sprintf("2026-10-%02d", i+2), "2", fmt.Sprintf("Small%d", i), "S") // 2/70 < 3%
	}

	l, a := ta.chart("/report?period=month&date=2026-10")
	wantChart(t, l, a, []string{"Named", "Other"}, []int64{30, 40})
}

func TestReportTableEmptyPeriod(t *testing.T) {
	ta := newTestApp(t)
	if got := ta.table("/report?period=year&date=2020"); len(got) != 0 {
		t.Errorf("table = %q, want no rows", got)
	}
}

func TestReportChartKeepsLoneSmallSliceNamed(t *testing.T) {
	ta := newTestApp(t)
	ta.seedIn("expense", "2026-10-02", "980", "Big", "S")
	ta.seedIn("expense", "2026-10-03", "20", "Small", "S")

	l, a := ta.chart("/report?period=month&date=2026-10")
	wantChart(t, l, a, []string{"Big", "Small"}, []int64{980, 20})
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

func TestReportControlsSubmitOnChangeWithoutButtons(t *testing.T) {
	ta := newTestApp(t)
	body := ta.mustReport("/report?period=month&date=2026-03&type=expense&group=subcategory")
	if strings.Contains(body, "<button") {
		t.Errorf("Report has a submit button:\n%s", body)
	}
	if n := strings.Count(body, `onchange="this.form.submit()"`); n != 3 {
		t.Errorf("auto-submitting controls = %d, want 3 (picker, Type, Group by)", n)
	}
	// Each form carries the other controls' values so changing one keeps them.
	wantAll(t, body,
		`<input type="hidden" name="type" value="expense">`,
		`<input type="hidden" name="group" value="subcategory">`,
		`<input type="hidden" name="date" value="2026-03">`,
	)
}

func TestReportAndFilterAcceptTheFullYearRange(t *testing.T) {
	ta := newTestApp(t)
	ta.seed("income", "0001-01-05", "100")
	ta.seed("income", "9999-12-31", "200")
	wantAll(t, ta.mustReport("/report?period=year&date=0001"), "Income €100")
	wantAll(t, ta.mustReport("/report?period=month&date=9999-12"), "Income €200")
	wantAll(t, ta.mustReport("/report"), `min="0001-01"`, `max="9999-12"`)
	wantAll(t, ta.get("/transactions?year=0001").Body, "0001-01-05")
	wantAll(t, ta.get("/transactions?year=9999").Body, "9999-12-31")
}
