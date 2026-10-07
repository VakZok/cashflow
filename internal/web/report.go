package web

import (
	"html/template"
	"net/http"
	"net/url"
	"time"
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

func (p period) query() string {
	return "/report?period=" + p.Kind + "&date=" + p.Value()
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

type reportData struct {
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

	income, expense, err := a.store.Totals(p.Start.Format(dateLayout), p.end().Format(dateLayout))
	if err != nil {
		a.serverError(w, err)
		return
	}
	net := income - expense

	d := reportData{Period: p, Income: formatEuro(income), Expense: formatEuro(expense)}
	switch {
	case net > 0:
		d.Net, d.NetClass = template.HTML("+"+formatEuro(net)), "net-positive"
	case net < 0:
		d.Net, d.NetClass = template.HTML("−"+formatEuro(-net)), "net-negative"
	default:
		d.Net, d.NetClass = template.HTML(formatEuro(0)), "net-zero"
	}
	if prev := p.shifted(-1); prev.valid() {
		d.Prev = prev.query()
	}
	if next := p.shifted(1); next.valid() {
		d.Next = next.query()
	}

	// Switching Month -> Year keeps the year; Year -> Month picks the current
	// month if the year is the current one, otherwise January.
	if p.Kind == periodMonth {
		d.ToggleLabel = "Year"
		d.ToggleLink = period{periodYear, time.Date(p.Start.Year(), 1, 1, 0, 0, 0, 0, time.UTC)}.query()
	} else {
		month := time.January
		if p.Start.Year() == now.Year() {
			month = now.Month()
		}
		d.ToggleLabel = "Month"
		d.ToggleLink = period{periodMonth, time.Date(p.Start.Year(), month, 1, 0, 0, 0, 0, time.UTC)}.query()
	}
	d.PickerYear = p.Kind == periodYear
	d.PickerMin, d.PickerMax = minYear, maxYear
	a.render(w, http.StatusOK, "report.html", d)
}
