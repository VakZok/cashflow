package web

import (
	"net/http"
	"strconv"

	"github.com/VakZok/money_manager_app/internal/store"
)

type transactionsData struct {
	Transactions []store.TransactionRow
	Page, Pages  int
	// Newer and Older are the links to the neighbouring pages, empty on the first and last page.
	Newer, Older string
}

// transactions renders the Transactions page. A bad page parameter falls back
// to the nearest valid page.
func (a *App) transactions(w http.ResponseWriter, r *http.Request) {
	total, err := a.store.CountTransactions()
	if err != nil {
		a.serverError(w, err)
		return
	}
	pages := (total + store.PageSize - 1) / store.PageSize
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	page = max(1, min(page, pages))

	d := transactionsData{Page: page, Pages: pages}
	if total > 0 {
		if d.Transactions, err = a.store.ListTransactionsPage(store.PageSize, (page-1)*store.PageSize); err != nil {
			a.serverError(w, err)
			return
		}
	}
	if page > 1 {
		d.Newer = "/transactions?page=" + strconv.Itoa(page-1)
	}
	if page < pages {
		d.Older = "/transactions?page=" + strconv.Itoa(page+1)
	}
	a.render(w, http.StatusOK, "transactions.html", d)
}
