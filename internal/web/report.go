package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"github.com/VakZok/money_manager_app/internal/store"
)

const (
	periodMonth = "month"
	periodYear  = "year"

	monthLayout = "2006-01"
	yearLayout  = "2006"

	minYear = 2000
	maxYear = 2200
)

// period is a calendar month or year (Start is its first day, UTC).
type period struct {
	Kind  string
	Start time.Time
}

func (p period) layout() string {
	if p.Kind == periodYear {
		return yearLayout
	}
	return monthLayout
}

// Value is the period's date as used in the URL and the picker.
func (p period) Value() string { return p.Start.Format(p.layout()) }

func (p period) end() time.Time {
	if p.Kind == periodYear {
		return p.Start.AddDate(1, 0, 0)
	}
	return p.Start.AddDate(0, 1, 0)
}

func (p period) shifted(n int) period {
	if p.Kind == periodYear {
		return period{p.Kind, p.Start.AddDate(n, 0, 0)}
	}
	return period{p.Kind, p.Start.AddDate(0, n, 0)}
}

func (p period) valid() bool {
	y := p.Start.Year()
	return y >= minYear && y <= maxYear
}

// query is the Report URL for the period, keeping the chart selection.
func (p period) query(c chartSel) string {
	return "/report?period=" + p.Kind + "&date=" + p.Value() + "&type=" + c.Type + "&group=" + c.Group
}

// parsePeriod reads ?period=&date=; anything missing or invalid yields the
// month containing now.
func parsePeriod(q url.Values, now time.Time) period {
	current := period{periodMonth, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)}
	kind := q.Get("period")
	layout := monthLayout
	switch kind {
	case periodMonth:
	case periodYear:
		layout = yearLayout
	default:
		return current
	}
	if len(q.Get("date")) != len(layout) {
		return current
	}
	d, err := time.Parse(layout, q.Get("date"))
	if err != nil {
		return current
	}
	p := period{kind, d}
	if !p.valid() {
		return current
	}
	return p
}

const (
	groupCategory    = "category"
	groupSubcategory = "subcategory"

	// otherThresholdPct: slices below this share of the total become "Other".
	otherThresholdPct = 3
)

// chartSel is the pie chart's Type (one with a Category pair, not a Levy) and Group-by.
type chartSel struct{ Type, Group string }

// chartTypes lists the Types the pie chart can break down: those with a
// Category and Subcategory pair; Levy is not part of the Report (ADR 0007).
func chartTypes() []store.TypeSpec {
	var out []store.TypeSpec
	for _, s := range store.Types {
		if s.TakesDefaultLists() {
			out = append(out, s)
		}
	}
	return out
}

// parseChartSel reads ?type=&group=; anything missing or invalid yields the
// defaults (expense, category).
func parseChartSel(q url.Values) chartSel {
	c := chartSel{Type: store.TypeExpense, Group: groupCategory}
	if s, ok := store.SpecFor(q.Get("type")); ok && s.TakesDefaultLists() {
		c.Type = s.Type
	}
	if q.Get("group") == groupSubcategory {
		c.Group = groupSubcategory
	}
	return c
}

// list is the list of the Type's Category pair the chart groups by.
func (c chartSel) list() store.List {
	spec, _ := store.SpecFor(c.Type)
	if c.Group == groupSubcategory {
		return spec.Lists.Subcategory
	}
	return spec.Lists.Category
}

// chartData is emitted as JSON for Chart.js.
type chartData struct {
	Labels  []string `json:"labels"`
	Amounts []int64  `json:"amounts"`
}

type legendRow struct{ Label, Amount, Share string }

// buildChart groups slices below the threshold into "Other" and returns the
// chart data and legend. rows must be ordered by amount, descending.
func buildChart(rows []store.Slice) (chartData, []legendRow) {
	cd := chartData{Labels: []string{}, Amounts: []int64{}}
	var total, other int64
	for _, r := range rows {
		total += r.Amount
	}
	for _, r := range rows {
		if r.Amount*100 < otherThresholdPct*total {
			other += r.Amount
			continue
		}
		cd.Labels = append(cd.Labels, r.Name)
		cd.Amounts = append(cd.Amounts, r.Amount)
	}
	if other > 0 {
		cd.Labels = append(cd.Labels, "Other")
		cd.Amounts = append(cd.Amounts, other)
	}
	legend := make([]legendRow, len(cd.Labels))
	for i, l := range cd.Labels {
		legend[i] = legendRow{l, formatEuro(cd.Amounts[i]), fmt.Sprintf("%.1f%%", float64(cd.Amounts[i])*100/float64(total))}
	}
	return cd, legend
}

type reportData struct {
	Chart       chartSel
	ChartTypes  []store.TypeSpec
	ChartData   chartData
	Legend      []legendRow
	Period      period
	Prev, Next  string // empty when out of range
	ToggleLabel string
	ToggleLink  string
	Net         template.HTML // built from a fixed sign and a formatted number
	NetClass    string
	Income      string
	Expense     string
	PickerYear  bool
	PickerMin   int
	PickerMax   int
}

func (a *App) report(w http.ResponseWriter, r *http.Request) {
	now := a.now()
	p := parsePeriod(r.URL.Query(), now)

	sel := parseChartSel(r.URL.Query())
	from, to := p.Start.Format(dateLayout), p.end().Format(dateLayout)

	income, expense, err := a.store.Totals(from, to)
	if err != nil {
		a.serverError(w, err)
		return
	}
	net := income - expense

	slices, err := a.store.Breakdown(sel.Type, sel.list(), from, to)
	if err != nil {
		a.serverError(w, err)
		return
	}
	d := reportData{Chart: sel, ChartTypes: chartTypes(), Period: p, Income: formatEuro(income), Expense: formatEuro(expense)}
	switch {
	case net > 0:
		text, class := signedEuro(net, true)
		d.Net, d.NetClass = template.HTML(text), class
	case net < 0:
		text, class := signedEuro(-net, false)
		d.Net, d.NetClass = template.HTML(text), class
	default:
		d.Net, d.NetClass = template.HTML(formatEuro(0)), "net-zero"
	}
	d.ChartData, d.Legend = buildChart(slices)
	if prev := p.shifted(-1); prev.valid() {
		d.Prev = prev.query(sel)
	}
	if next := p.shifted(1); next.valid() {
		d.Next = next.query(sel)
	}

	// Switching Month -> Year keeps the year; Year -> Month picks the current
	// month if the year is the current one, otherwise January.
	if p.Kind == periodMonth {
		d.ToggleLabel = "Year"
		d.ToggleLink = period{periodYear, time.Date(p.Start.Year(), 1, 1, 0, 0, 0, 0, time.UTC)}.query(sel)
	} else {
		month := time.January
		if p.Start.Year() == now.Year() {
			month = now.Month()
		}
		d.ToggleLabel = "Month"
		d.ToggleLink = period{periodMonth, time.Date(p.Start.Year(), month, 1, 0, 0, 0, 0, time.UTC)}.query(sel)
	}
	d.PickerYear = p.Kind == periodYear
	d.PickerMin, d.PickerMax = minYear, maxYear
	a.render(w, http.StatusOK, "report.html", d)
}
