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
	Types  []store.TypeSpec // the Type dropdown's choices besides All
	// Groupings are the Category and Subcategory dropdowns of the chosen Type, nil if it has none.
	Groupings *groupings
}

// groupings are the choices of a Type's Category and Subcategory dropdowns,
// or of its Holding dropdown.
type groupings struct {
	Category, Subcategory     store.List
	Categories, Subcategories []store.Named
	Holding                   bool // the Type takes a Holding dropdown instead
	Holdings                  []store.Named
}

// filter is the narrowing of the Transactions page, as carried in its URL.
// The zero filter narrows nothing.
type filter struct {
	Year  int // 0 is All
	Month int // 1-12, 0 is the whole Year; always 0 without a Year

	Type string // a stored Type, "" is All
	// Category and Subcategory are ids of the Type's Category and Subcategory
	// list (the Levy ones for Levy and Rebate), 0 is all. Always 0 for a Type
	// without such lists.
	Category, Subcategory int64
	// Holding is the id of a Holding, 0 is all. Always 0 for a Type without one.
	Holding int64

	// Search is the trimmed text to find in descriptions and Holdings, "" is none.
	// Always "" for a Type with a Holding dropdown, which replaces the search field.
	Search string
}

// maxSearchLen caps the Search text taken from the URL, in characters.
const maxSearchLen = store.MaxDescriptionLen

// filterTypes are the Type dropdown's choices besides All, in the order the filter offers them.
var filterTypes = func() []store.TypeSpec {
	var out []store.TypeSpec
	for _, typ := range []string{store.TypeIncome, store.TypeExpense, store.TypeLevy, store.TypeRebate, store.TypeInvestment, store.TypeDivestment} {
		spec, _ := store.SpecFor(typ)
		out = append(out, spec)
	}
	return out
}()

// parseFilter reads ?year=&month=&type=&category=&subcategory=&holding=; anything
// missing or invalid is All.
func parseFilter(q url.Values) filter {
	var f filter
	if y := q.Get("year"); len(y) == len(yearLayout) && wholeNumber.MatchString(y) {
		f.Year, _ = strconv.Atoi(y)
	}
	if f.Year < minYear || f.Year > maxYear {
		f.Year = 0
	} else if m := q.Get("month"); wholeNumber.MatchString(m) && len(m) <= 2 {
		if n, _ := strconv.Atoi(m); n >= 1 && n <= 12 {
			f.Month = n
		}
	}
	spec, ok := store.SpecFor(q.Get("type"))
	if ok {
		f.Type = spec.Type
		// The Type dropdown sends the Type it was changed from as "from"; the
		// groupings only carry over to a Type that uses the same lists.
		keep := true
		if q.Has("from") {
			prev, _ := store.SpecFor(q.Get("from"))
			keep = sharesGroupings(prev, spec)
		}
		if keep && spec.Lists != nil {
			f.Category, f.Subcategory = parseID(q.Get("category")), parseID(q.Get("subcategory"))
		}
		if keep && spec.Holding {
			f.Holding = parseID(q.Get("holding"))
		}
	}
	if !spec.Holding {
		f.Search = strings.TrimSpace(q.Get("search"))
		if r := []rune(strings.ToValidUTF8(f.Search, "")); len(r) > maxSearchLen {
			f.Search = strings.TrimSpace(string(r[:maxSearchLen]))
		}
	}
	return f
}

// sharesGroupings reports whether two Types filter by the same groupings:
// both take the same Category and Subcategory lists, or both take Holdings.
func sharesGroupings(a, b store.TypeSpec) bool {
	switch {
	case a.Holding:
		return b.Holding
	case a.Lists != nil:
		return b.TakesLists(*a.Lists)
	}
	return false
}

// parseID reads a positive id; anything else is 0.
func parseID(s string) int64 {
	if len(s) > 18 || !wholeNumber.MatchString(s) {
		return 0
	}
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}

// groupings loads the choices of the Type's Category and Subcategory dropdowns
// (or Holding dropdown) and drops a selection that is not among them, as it is
// not a choice of the dropdown. It is nil if the Type has no such dropdowns.
func (a *App) groupings(f filter) (*groupings, filter, error) {
	spec, _ := store.SpecFor(f.Type)
	if spec.Holding {
		g := &groupings{Holding: true}
		var err error
		if g.Holdings, err = a.store.ListNamed(store.HoldingList); err != nil {
			return nil, f, err
		}
		if !slices.ContainsFunc(g.Holdings, func(n store.Named) bool { return n.ID == f.Holding }) {
			f.Holding = 0
		}
		return g, f, nil
	}
	if spec.Lists == nil {
		return nil, f, nil
	}
	g := &groupings{Category: spec.Lists.Category, Subcategory: spec.Lists.Subcategory}
	var err error
	if g.Categories, err = a.store.ListNamed(g.Category); err != nil {
		return nil, f, err
	}
	if g.Subcategories, err = a.store.ListNamed(g.Subcategory); err != nil {
		return nil, f, err
	}
	if !slices.ContainsFunc(g.Categories, func(n store.Named) bool { return n.ID == f.Category }) {
		f.Category = 0
	}
	if !slices.ContainsFunc(g.Subcategories, func(n store.Named) bool { return n.ID == f.Subcategory }) {
		f.Subcategory = 0
	}
	return g, f, nil
}

// among drops a Year that no Transaction is dated in (with its Month): it is
// not a choice of the Year dropdown.
func (f filter) among(years []int) filter {
	if f.Year != 0 && !slices.Contains(years, f.Year) {
		f.Year, f.Month = 0, 0
	}
	return f
}

// store is the filter as the store applies it.
func (f filter) store() store.Filter {
	sf := store.Filter{Type: f.Type, CategoryID: f.Category, SubcategoryID: f.Subcategory, HoldingID: f.Holding, Search: f.Search}
	if f.Year != 0 {
		from := time.Date(f.Year, time.January, 1, 0, 0, 0, 0, time.UTC)
		to := from.AddDate(1, 0, 0)
		if f.Month != 0 {
			from = from.AddDate(0, f.Month-1, 0)
			to = from.AddDate(0, 1, 0)
		}
		sf.From, sf.To = from.Format(dateLayout), to.Format(dateLayout)
	}
	return sf
}

// query is the filter's URL parameters, "" when it narrows nothing.
func (f filter) query() string {
	var parts []string
	if f.Year != 0 {
		parts = append(parts, "year="+strconv.Itoa(f.Year))
	}
	if f.Month != 0 {
		parts = append(parts, "month="+strconv.Itoa(f.Month))
	}
	if f.Type != "" {
		parts = append(parts, "type="+f.Type)
	}
	if f.Category != 0 {
		parts = append(parts, "category="+strconv.FormatInt(f.Category, 10))
	}
	if f.Subcategory != 0 {
		parts = append(parts, "subcategory="+strconv.FormatInt(f.Subcategory, 10))
	}
	if f.Holding != 0 {
		parts = append(parts, "holding="+strconv.FormatInt(f.Holding, 10))
	}
	if f.Search != "" {
		parts = append(parts, "search="+url.QueryEscape(f.Search))
	}
	return strings.Join(parts, "&")
}

// Active reports whether the filter narrows anything.
func (f filter) Active() bool { return f.query() != "" }

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

// returnTo is where Apply, Delete and Discard take the user back to: the add
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
	g, f, err := a.groupings(f)
	if err != nil {
		a.serverError(w, err)
		return
	}
	total, err := a.store.CountTransactions(f.store())
	if err != nil {
		a.serverError(w, err)
		return
	}
	pages := pageCount(total)
	page = clampPage(page, total)

	d := transactionsData{Page: page, Pages: pages, PageURL: transactionsURL(f, page), Filter: f, Years: years, Months: monthChoices, Types: filterTypes, Groupings: g}
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
