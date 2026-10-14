package web

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/VakZok/money_manager_app/internal/store"
)

const (
	errIncomplete  = "All fields must be filled in for every row."
	errTooManyRows = "The list can hold at most 50 rows."
)

// repeatingRow is one row of the Repeating form: submitted values (so a
// rejected submission keeps what was typed). Index only names its inputs.
type repeatingRow struct {
	Index          int
	Type           string
	Amount         string
	Description    string
	Category       string
	NewCategory    string
	Subcategory    string
	NewSubcategory string
}

func (r repeatingRow) category() choice {
	return choice{store.DefaultListSet.Category, r.Category, r.NewCategory}
}

func (r repeatingRow) subcategory() choice {
	return choice{store.DefaultListSet.Subcategory, r.Subcategory, r.NewSubcategory}
}

// blank reports whether the user left the row untouched; its Type does not
// count, a select always has one.
func (r repeatingRow) blank() bool {
	return r.Amount == "" && r.Description == "" && r.Category == "" && r.NewCategory == "" &&
		r.Subcategory == "" && r.NewSubcategory == ""
}

type repeatingData struct {
	Rows          []repeatingRow
	Types         []store.TypeSpec
	Categories    []store.Named
	Subcategories []store.Named
	MaxRows       int
	Errors        []string
	Saved         bool
}

// repeatingTypes are the Types a Repeating row can take so far: those drawing
// from the Category and Subcategory lists, Income and Expense.
func repeatingTypes() []store.TypeSpec {
	var out []store.TypeSpec
	for _, s := range store.Types {
		if s.TakesDefaultLists() {
			out = append(out, s)
		}
	}
	return out
}

func repeatingSpec(typ string) (store.TypeSpec, bool) {
	for _, s := range repeatingTypes() {
		if s.Type == typ {
			return s, true
		}
	}
	return store.TypeSpec{}, false
}

func (a *App) repeating(w http.ResponseWriter, r *http.Request) {
	entries, err := a.store.ListRepeating()
	if err != nil {
		a.serverError(w, err)
		return
	}
	var rows []repeatingRow
	for _, e := range entries {
		rows = append(rows, repeatingRow{
			Type:        e.Type,
			Amount:      strconv.FormatInt(e.Amount, 10),
			Description: e.Description,
			Category:    strconv.FormatInt(e.CategoryID, 10),
			Subcategory: strconv.FormatInt(e.SubcategoryID, 10),
		})
	}
	a.renderRepeating(w, http.StatusOK, rows, nil, r.URL.Query().Get("saved") == "1")
}

func (a *App) renderRepeating(w http.ResponseWriter, status int, rows []repeatingRow, errs []string, saved bool) {
	if len(rows) == 0 {
		rows = []repeatingRow{{Type: store.TypeExpense}}
	}
	for i := range rows {
		rows[i].Index = i
	}
	d := repeatingData{Rows: rows, Types: repeatingTypes(), MaxRows: store.MaxRepeating, Errors: errs, Saved: saved}
	var err error
	if d.Categories, err = a.store.ListNamed(store.DefaultListSet.Category); err != nil {
		a.serverError(w, err)
		return
	}
	if d.Subcategories, err = a.store.ListNamed(store.DefaultListSet.Subcategory); err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, status, "repeating.html", d)
}

// saveRepeating replaces the stored list with the submitted one. Fully blank
// rows are dropped; if any other row is invalid nothing is saved or created.
func (a *App) saveRepeating(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var rows, filled []repeatingRow
	for _, n := range r.PostForm["row"] {
		row := repeatingRow{
			Type:           r.PostFormValue("type_" + n),
			Amount:         strings.TrimSpace(r.PostFormValue("amount_" + n)),
			Description:    strings.TrimSpace(r.PostFormValue("description_" + n)),
			Category:       r.PostFormValue("category_" + n),
			NewCategory:    strings.TrimSpace(r.PostFormValue("new_category_" + n)),
			Subcategory:    r.PostFormValue("subcategory_" + n),
			NewSubcategory: strings.TrimSpace(r.PostFormValue("new_subcategory_" + n)),
		}
		rows = append(rows, row)
		if !row.blank() {
			filled = append(filled, row)
		}
	}

	var errs []string
	addError := func(msg string) {
		for _, e := range errs {
			if e == msg {
				return
			}
		}
		errs = append(errs, msg)
	}
	if len(filled) > store.MaxRepeating {
		addError(errTooManyRows)
	}
	entries := make([]store.RepeatingEntry, 0, len(filled))
	for _, row := range filled {
		if _, ok := repeatingSpec(row.Type); !ok {
			addError("Type must be " + repeatingTypeChoices() + ".")
			continue
		}
		complete := row.Amount != "" && row.Description != ""
		for _, c := range []choice{row.category(), row.subcategory()} {
			ok, err := a.validChoice(c)
			if err != nil {
				a.serverError(w, err)
				return
			}
			complete = complete && ok
		}
		if !complete {
			addError(errIncomplete)
			continue
		}
		amount, ok := parseAmount(row.Amount)
		if !ok {
			addError(errAmount)
		}
		if utf8.RuneCountInString(row.Description) > store.MaxDescriptionLen {
			addError(errDescriptionLen)
		}
		for _, c := range []choice{row.category(), row.subcategory()} {
			if c.value == newOption && utf8.RuneCountInString(c.newName) > store.MaxNameLen {
				addError(c.list.Label + " name must be at most 50 characters.")
			}
		}
		entries = append(entries, store.RepeatingEntry{Type: row.Type, Amount: amount, Description: row.Description})
	}
	if len(errs) > 0 {
		a.renderRepeating(w, http.StatusUnprocessableEntity, rows, errs, false)
		return
	}

	// Everything is valid: only now create inline "new..." entries.
	for i, row := range filled {
		var err error
		if entries[i].CategoryID, err = a.resolveChoice(row.category()); err != nil {
			a.serverError(w, err)
			return
		}
		if entries[i].SubcategoryID, err = a.resolveChoice(row.subcategory()); err != nil {
			a.serverError(w, err)
			return
		}
	}
	if err := a.store.ReplaceRepeating(entries); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/repeating?saved=1", http.StatusSeeOther)
}

// repeatingTypeChoices reads "Expense or Income" from the Types a row can take.
func repeatingTypeChoices() string {
	var labels []string
	for _, s := range repeatingTypes() {
		labels = append(labels, s.Label)
	}
	return strings.Join(labels, " or ")
}
