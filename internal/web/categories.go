package web

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/VakZok/money_manager_app/internal/store"
)

// maxNameLen is the longest name of a list entry, in characters.
const maxNameLen = 50

// listChoice is a choice of the Edit Categories page's dropdown; Key is its
// value in the URL.
type listChoice struct {
	Key  string
	List store.List
}

// listChoices are the dropdown's choices, the first being the default.
var listChoices = []listChoice{
	{"category", store.CategoryList},
	{"subcategory", store.SubcategoryList},
	{"levy-category", store.LevyCategoryList},
	{"levy-subcategory", store.LevySubcategoryList},
	{"holding", store.HoldingList},
}

// choiceFor is the choice with the key, or the default if there is none.
func choiceFor(key string) listChoice {
	for _, c := range listChoices {
		if c.Key == key {
			return c
		}
	}
	return listChoices[0]
}

// Values of the one-off "added" flag the redirect after an Add carries.
const (
	addedNew    = "new"
	addedExists = "exists"
)

type categoriesData struct {
	Choices      []listChoice
	Current      listChoice
	Entries      []store.NamedUsage
	Page, Pages  int
	Newer, Older string
	PageURL      string // this page, without any edit; Discard goes back to it
	Added        string // addedNew, addedExists or "" for the Add button's label
	EditID       int64  // the entry whose row is open, 0 for none
	EditName     string // the text in the open row's name field
	Taken        bool   // the open row's rename was refused: Apply shows "Taken"
}

// categoriesURL is the page of the list on the Edit Categories page.
func categoriesURL(c listChoice, page int) string {
	return "/categories?list=" + c.Key + "&page=" + strconv.Itoa(page)
}

// categories renders the Edit Categories page. A bad list falls back to
// Category and a bad page to the nearest valid one.
func (a *App) categories(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c := choiceFor(q.Get("list"))
	page, _ := strconv.Atoi(q.Get("page"))
	total, err := a.store.CountNamed(c.List)
	if err != nil {
		a.serverError(w, err)
		return
	}
	pages := pageCount(total)
	page = clampPage(page, total)
	d := categoriesData{Choices: listChoices, Current: c, Page: page, Pages: pages}
	switch q.Get("added") {
	case addedNew, addedExists:
		d.Added = q.Get("added")
	}
	if total > 0 {
		if d.Entries, err = a.store.ListNamedPage(c.List, store.PageSize, (page-1)*store.PageSize); err != nil {
			a.serverError(w, err)
			return
		}
	}
	d.PageURL = categoriesURL(c, page)
	if editID, err := strconv.ParseInt(q.Get("edit"), 10, 64); err == nil {
		for _, e := range d.Entries {
			if e.ID != editID {
				continue
			}
			d.EditID, d.EditName = e.ID, e.Name
			if q.Has("name") {
				d.EditName = q.Get("name")
				d.Taken = q.Get("taken") == "1"
			}
		}
	}
	if page > 1 {
		d.Newer = categoriesURL(c, page-1)
	}
	if page < pages {
		d.Older = categoriesURL(c, page+1)
	}
	a.render(w, http.StatusOK, "categories.html", d)
}

// addCategory adds the name to the chosen list unless it is there already,
// and redirects to the page the entry sits on, flagged for the Add button.
func (a *App) addCategory(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	c := choiceFor(r.PostForm.Get("list"))
	name := strings.TrimSpace(strings.ToValidUTF8(r.PostForm.Get("name"), ""))
	if utf8.RuneCountInString(name) > maxNameLen {
		http.Error(w, "name too long", http.StatusBadRequest)
		return
	}
	if name == "" {
		http.Redirect(w, r, categoriesURL(c, 1), http.StatusSeeOther)
		return
	}
	page, created, err := a.store.AddNamed(c.List, name)
	if err != nil {
		a.serverError(w, err)
		return
	}
	flag := addedExists
	if created {
		flag = addedNew
	}
	http.Redirect(w, r, categoriesURL(c, page)+"&added="+url.QueryEscape(flag), http.StatusSeeOther)
}

// renameCategory renames the entry of the chosen list and redirects to the page
// the entry now sorts on. A name another entry of the list has, ignoring case,
// is refused: nothing changes and the redirect reopens the entry's row with the
// typed text, flagged for the Apply button. A blank name changes nothing.
func (a *App) renameCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	c := choiceFor(r.PostForm.Get("list"))
	name := strings.TrimSpace(strings.ToValidUTF8(r.PostForm.Get("name"), ""))
	if utf8.RuneCountInString(name) > maxNameLen {
		http.Error(w, "name too long", http.StatusBadRequest)
		return
	}
	reopen := "" // the query that keeps the entry's row open
	if name == "" {
		reopen = "&edit=" + strconv.FormatInt(id, 10)
	} else if err := a.store.RenameNamed(c.List, id, name); errors.Is(err, store.ErrNameTaken) {
		reopen = "&edit=" + strconv.FormatInt(id, 10) + "&name=" + url.QueryEscape(name) + "&taken=1"
	} else if err != nil {
		a.renameError(w, r, err)
		return
	}
	page, err := a.store.NamedPage(c.List, id)
	if err != nil {
		a.renameError(w, r, err)
		return
	}
	http.Redirect(w, r, categoriesURL(c, page)+reopen, http.StatusSeeOther)
}

// renameError answers a failed rename: 404 for a missing entry, else 500.
func (a *App) renameError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrEntryNotFound) {
		http.NotFound(w, r)
		return
	}
	a.serverError(w, err)
}
