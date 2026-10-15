package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/VakZok/money_manager_app/internal/store"
)

type transactionsData struct {
	Transactions []store.TransactionRow
	Page, Pages  int
	// Newer and Older are the links to the neighbouring pages, empty on the first and last page.
	Newer, Older string
	Edit         *editView // the row being edited, nil if none
}

// returnTo is where Change, Delete and Discard take the user back to: the add
// page, or a page of the Transactions page. It is a closed set so the hidden
// form value can never redirect anywhere else.
type returnTo struct {
	Page int // 0 is the add page, otherwise the page of the Transactions page
}

const returnAdd, returnTransactionsPrefix = "add", "transactions:"

// parseReturn reads the hidden "return" value back; anything unrecognised is the add page.
func parseReturn(s string) returnTo {
	if p, ok := strings.CutPrefix(s, returnTransactionsPrefix); ok && wholeNumber.MatchString(p) {
		if n, err := strconv.Atoi(p); err == nil && n >= 1 {
			return returnTo{Page: n}
		}
	}
	return returnTo{}
}

// String is the value of the hidden "return" input.
func (r returnTo) String() string {
	if r.Page == 0 {
		return returnAdd
	}
	return returnTransactionsPrefix + strconv.Itoa(r.Page)
}

// URL is the page itself, which is where Discard and Delete go.
func (r returnTo) URL() string {
	if r.Page == 0 {
		return "/"
	}
	return "/transactions?page=" + strconv.Itoa(r.Page)
}

// pageCount is the number of pages of the Transactions page for total Transactions.
func pageCount(total int) int { return (total + store.PageSize - 1) / store.PageSize }

// clampPage is the nearest valid page; page 1 even when there are no Transactions.
func clampPage(page, total int) int { return max(1, min(page, pageCount(total))) }

// transactions renders the Transactions page. A bad page parameter falls back
// to the nearest valid page.
func (a *App) transactions(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	var edit *editing
	if id, err := strconv.ParseInt(r.URL.Query().Get("edit"), 10, 64); err == nil {
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
	a.renderTransactions(w, http.StatusOK, page, edit)
}

// renderTransactions renders the Transactions page at the page, with the
// Transaction in edit (if it is on that page) shown as a form.
func (a *App) renderTransactions(w http.ResponseWriter, status, page int, edit *editing) {
	total, err := a.store.CountTransactions()
	if err != nil {
		a.serverError(w, err)
		return
	}
	pages := pageCount(total)
	page = clampPage(page, total)

	d := transactionsData{Page: page, Pages: pages}
	if total > 0 {
		if d.Transactions, err = a.store.ListTransactionsPage(store.PageSize, (page-1)*store.PageSize); err != nil {
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
		d.Edit = newEditView(edit, lists, returnTo{Page: page})
	}
	if page > 1 {
		d.Newer = "/transactions?page=" + strconv.Itoa(page-1)
	}
	if page < pages {
		d.Older = "/transactions?page=" + strconv.Itoa(page+1)
	}
	a.render(w, status, "transactions.html", d)
}
