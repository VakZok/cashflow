package web

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VakZok/money_manager_app/internal/store"
)

type transactionsData struct {
	Transactions []store.TransactionRow
	Page, Pages  int
	// PageURL is this page with its filter, which the rows' edit links extend.
	PageURL string
	// Newer and Older are the links to the neighbouring pages, empty on the first and last page.
	Newer, Older string
	Edit         *editView // the row being edited, nil if none

	Filter filter
	Years  []int // the Year dropdown's choices besides All; none means there are no Transactions
	Months []monthChoice
}

// filter is the narrowing of the Transactions page, as carried in its URL.
// The zero filter narrows nothing.
type filter struct {
	Year  int // 0 is All
	Month int // 1-12, 0 is the whole Year; always 0 without a Year
}

// parseFilter reads ?year=&month=; anything missing or invalid is All.
func parseFilter(q url.Values) filter {
	var f filter
	if y := q.Get("year"); len(y) == len(yearLayout) && wholeNumber.MatchString(y) {
		f.Year, _ = strconv.Atoi(y)
	}
	if f.Year < minYear || f.Year > maxYear {
		return filter{}
	}
	if m := q.Get("month"); wholeNumber.MatchString(m) && len(m) <= 2 {
		if n, _ := strconv.Atoi(m); n >= 1 && n <= 12 {
			f.Month = n
		}
	}
	return f
}

// among drops a Year that no Transaction is dated in (with its Month): it is
// not a choice of the Year dropdown.
func (f filter) among(years []int) filter {
	if slices.Contains(years, f.Year) {
		return f
	}
	return filter{}
}

// store is the filter as the store applies it.
func (f filter) store() store.Filter {
	if f.Year == 0 {
		return store.Filter{}
	}
	from := time.Date(f.Year, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, 0)
	if f.Month != 0 {
		from = from.AddDate(0, f.Month-1, 0)
		to = from.AddDate(0, 1, 0)
	}
	return store.Filter{From: from.Format(dateLayout), To: to.Format(dateLayout)}
}

// query is the filter's URL parameters, "" when it narrows nothing.
func (f filter) query() string {
	if f.Year == 0 {
		return ""
	}
	q := "year=" + strconv.Itoa(f.Year)
	if f.Month != 0 {
		q += "&month=" + strconv.Itoa(f.Month)
	}
	return q
}

// Active reports whether the filter narrows anything.
func (f filter) Active() bool { return f.Year != 0 }

// transactionsURL is the page of the Transactions page narrowed by the filter.
func transactionsURL(f filter, page int) string {
	q := f.query()
	if q != "" {
		q += "&"
	}
	return "/transactions?" + q + "page=" + strconv.Itoa(page)
}

// monthChoice is a choice of the Month dropdown.
type monthChoice struct {
	Number int
	Label  string
}

// monthChoices are the Month dropdown's choices besides All, January first.
var monthChoices = func() []monthChoice {
	out := make([]monthChoice, 12)
	for i := range out {
		m := time.Month(i + 1)
		out[i] = monthChoice{int(m), m.String()[:3]}
	}
	return out
}()

// returnTo is where Change, Delete and Discard take the user back to: the add
// page, or a page of the Transactions page with its filter. It is a closed set
// so the hidden form value can never redirect anywhere else.
type returnTo struct {
	Page   int // 0 is the add page, otherwise the page of the Transactions page
	Filter filter
}

const returnAdd, returnTransactionsPrefix = "add", "transactions:"

// parseReturn reads the hidden "return" value back; anything unrecognised is
// the add page, and an unrecognised filter narrows nothing.
func parseReturn(s string) returnTo {
	rest, ok := strings.CutPrefix(s, returnTransactionsPrefix)
	if !ok {
		return returnTo{}
	}
	p, query, _ := strings.Cut(rest, "?")
	if !wholeNumber.MatchString(p) {
		return returnTo{}
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 1 {
		return returnTo{}
	}
	q, _ := url.ParseQuery(query)
	return returnTo{Page: n, Filter: parseFilter(q)}
}

// String is the value of the hidden "return" input.
func (r returnTo) String() string {
	if r.Page == 0 {
		return returnAdd
	}
	s := returnTransactionsPrefix + strconv.Itoa(r.Page)
	if q := r.Filter.query(); q != "" {
		s += "?" + q
	}
	return s
}

// URL is the page itself, which is where Discard and Delete go.
func (r returnTo) URL() string {
	if r.Page == 0 {
		return "/"
	}
	return transactionsURL(r.Filter, r.Page)
}

// clamped is the return with its page clamped to the pages its filter has now.
func (a *App) clamped(r returnTo) (returnTo, error) {
	if r.Page == 0 {
		return r, nil
	}
	total, err := a.store.CountTransactions(r.Filter.store())
	r.Page = clampPage(r.Page, total)
	return r, err
}

// pageCount is the number of pages of the Transactions page for total Transactions.
func pageCount(total int) int { return (total + store.PageSize - 1) / store.PageSize }

// clampPage is the nearest valid page; page 1 even when there are no Transactions.
func clampPage(page, total int) int { return max(1, min(page, pageCount(total))) }

// transactions renders the Transactions page. A bad page parameter falls back
// to the nearest valid page, a bad filter parameter to All.
func (a *App) transactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	var edit *editing
	if id, err := strconv.ParseInt(q.Get("edit"), 10, 64); err == nil {
		t, err := a.store.GetTransaction(id)
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			a.serverError(w, err)
			return
		default:
			edit = &editing{ID: id, Form: formOf(t)}
		}
	}
	a.renderTransactions(w, http.StatusOK, page, parseFilter(q), edit)
}

// renderTransactions renders the Transactions page at the page, narrowed by
// the filter, with the Transaction in edit (if it is on that page) shown as a form.
func (a *App) renderTransactions(w http.ResponseWriter, status, page int, f filter, edit *editing) {
	years, err := a.store.TransactionYears()
	if err != nil {
		a.serverError(w, err)
		return
	}
	f = f.among(years)
	total, err := a.store.CountTransactions(f.store())
	if err != nil {
		a.serverError(w, err)
		return
	}
	pages := pageCount(total)
	page = clampPage(page, total)

	d := transactionsData{Page: page, Pages: pages, PageURL: transactionsURL(f, page), Filter: f, Years: years, Months: monthChoices}
	if total > 0 {
		if d.Transactions, err = a.store.ListTransactionsPage(f.store(), store.PageSize, (page-1)*store.PageSize); err != nil {
			a.serverError(w, err)
			return
		}
	}
	if edit != nil {
		lists, err := a.formLists()
		if err != nil {
			a.serverError(w, err)
			return
		}
		d.Edit = newEditView(edit, lists, returnTo{Page: page, Filter: f})
	}
	if page > 1 {
		d.Newer = transactionsURL(f, page-1)
	}
	if page < pages {
		d.Older = transactionsURL(f, page+1)
	}
	a.render(w, status, "transactions.html", d)
}
