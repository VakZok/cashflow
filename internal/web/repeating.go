package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VakZok/money_manager_app/internal/store"
)

const (
	errIncomplete       = "All fields must be filled in for every row."
	errTooManyRows      = "The list can hold at most 50 rows."
	errInListDuplicates = "Duplicate rows in the list: "
	errNothingToAdd     = "The list is empty, there is nothing to add."
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
	// Levy Category and Levy Subcategory are separate inputs, as on the home form.
	LevyCategory       string
	NewLevyCategory    string
	LevySubcategory    string
	NewLevySubcategory string
	// Holding is the only field Investment and Divestment rows take besides the amount.
	Holding    string
	NewHolding string

	holdingName string // set on save: the Holding's name, existing or new
}

// TakesHolding reports whether the row's Type takes a Holding instead of
// description, Category and Subcategory.
func (r repeatingRow) TakesHolding() bool {
	s, ok := repeatingSpec(r.Type)
	return ok && s.Holding
}

// choices are the list picks a row of this Type must make, in form order.
func (r repeatingRow) choices(spec store.TypeSpec) []choice {
	if spec.Holding {
		return []choice{r.holding()}
	}
	return []choice{r.category(*spec.Lists), r.subcategory(*spec.Lists)}
}

func (r repeatingRow) category(set store.ListSet) choice {
	if set == store.LevyListSet {
		return choice{set.Category, r.LevyCategory, r.NewLevyCategory}
	}
	return choice{set.Category, r.Category, r.NewCategory}
}

func (r repeatingRow) subcategory(set store.ListSet) choice {
	if set == store.LevyListSet {
		return choice{set.Subcategory, r.LevySubcategory, r.NewLevySubcategory}
	}
	return choice{set.Subcategory, r.Subcategory, r.NewSubcategory}
}

// TakesLevyLists reports whether the row's Type draws from the Levy lists.
func (r repeatingRow) TakesLevyLists() bool {
	s, ok := repeatingSpec(r.Type)
	return ok && s.TakesLevyLists()
}

// blank reports whether the user left the row untouched; its Type does not
// count, a select always has one.
func (r repeatingRow) blank() bool {
	return r.Amount == "" && r.Description == "" && r.Category == "" && r.NewCategory == "" &&
		r.Subcategory == "" && r.NewSubcategory == "" && r.LevyCategory == "" && r.NewLevyCategory == "" &&
		r.LevySubcategory == "" && r.NewLevySubcategory == "" && r.Holding == "" && r.NewHolding == ""
}

type repeatingData struct {
	Rows              []repeatingRow
	Types             []store.TypeSpec
	Categories        []store.Named
	Subcategories     []store.Named
	LevyCategories    []store.Named
	LevySubcategories []store.Named
	Holdings          []store.Named
	MaxRows           int
	Date              string
	Errors            []string // shown in the box below the buttons
	Notice            string   // likewise; never persisted
}

// repeatingTypes are the Types a Repeating row can take: all of them.
func repeatingTypes() []store.TypeSpec { return store.Types }

func repeatingSpec(typ string) (store.TypeSpec, bool) {
	for _, s := range repeatingTypes() {
		if s.Type == typ {
			return s, true
		}
	}
	return store.TypeSpec{}, false
}

// defaultBookingDate is the 1st day of the current month.
func (a *App) defaultBookingDate() string {
	n := a.now()
	return time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC).Format(dateLayout)
}

// savedRows reads the stored list as form rows.
func (a *App) savedRows() ([]repeatingRow, error) {
	entries, err := a.store.ListRepeating()
	if err != nil {
		return nil, err
	}
	var rows []repeatingRow
	for _, e := range entries {
		row := repeatingRow{Type: e.Type, Amount: strconv.FormatInt(e.Amount, 10), Description: e.Description}
		if row.TakesHolding() {
			row.Holding = strconv.FormatInt(e.HoldingID, 10)
		} else if row.TakesLevyLists() {
			row.LevyCategory = strconv.FormatInt(e.LevyCategoryID, 10)
			row.LevySubcategory = strconv.FormatInt(e.LevySubcategoryID, 10)
		} else {
			row.Category = strconv.FormatInt(e.CategoryID, 10)
			row.Subcategory = strconv.FormatInt(e.SubcategoryID, 10)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (a *App) repeating(w http.ResponseWriter, r *http.Request) {
	rows, err := a.savedRows()
	if err != nil {
		a.serverError(w, err)
		return
	}
	notice := ""
	if r.URL.Query().Get("saved") == "1" {
		notice = "List saved"
	}
	a.renderRepeating(w, http.StatusOK, rows, a.defaultBookingDate(), nil, notice)
}

func (a *App) renderRepeating(w http.ResponseWriter, status int, rows []repeatingRow, date string, errs []string, notice string) {
	if len(rows) == 0 {
		rows = []repeatingRow{{Type: store.TypeExpense}}
	}
	for i := range rows {
		rows[i].Index = i
	}
	d := repeatingData{Rows: rows, Types: repeatingTypes(), MaxRows: store.MaxRepeating, Date: date, Errors: errs, Notice: notice}
	var err error
	if d.Categories, err = a.store.ListNamed(store.DefaultListSet.Category); err != nil {
		a.serverError(w, err)
		return
	}
	if d.Subcategories, err = a.store.ListNamed(store.DefaultListSet.Subcategory); err != nil {
		a.serverError(w, err)
		return
	}
	if d.LevyCategories, err = a.store.ListNamed(store.LevyListSet.Category); err != nil {
		a.serverError(w, err)
		return
	}
	if d.LevySubcategories, err = a.store.ListNamed(store.LevyListSet.Subcategory); err != nil {
		a.serverError(w, err)
		return
	}
	if d.Holdings, err = a.store.ListNamed(store.HoldingList); err != nil {
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
	date := a.defaultBookingDate()
	if r.PostForm.Has("date") {
		date = strings.TrimSpace(r.PostFormValue("date"))
		if len(date) == len("2006-01") {
			date += "-01" // month picker: book on the 1st
		}
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

			LevyCategory:       r.PostFormValue("levy_category_" + n),
			NewLevyCategory:    strings.TrimSpace(r.PostFormValue("new_levy_category_" + n)),
			LevySubcategory:    r.PostFormValue("levy_subcategory_" + n),
			NewLevySubcategory: strings.TrimSpace(r.PostFormValue("new_levy_subcategory_" + n)),

			Holding:    r.PostFormValue("holding_" + n),
			NewHolding: strings.TrimSpace(r.PostFormValue("new_holding_" + n)),
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
	holdings, err := a.store.ListNamed(store.HoldingList)
	if err != nil {
		a.serverError(w, err)
		return
	}
	for i := range filled {
		row := &filled[i]
		spec, ok := repeatingSpec(row.Type)
		if !ok {
			addError("Type must be " + repeatingTypeChoices() + ".")
			continue
		}
		choices := row.choices(spec)
		complete := row.Amount != "" && (spec.Holding || row.Description != "")
		for _, c := range choices {
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
		if spec.Holding {
			row.Description = ""
			row.holdingName = row.NewHolding
			for _, h := range holdings {
				if row.Holding == strconv.FormatInt(h.ID, 10) {
					row.holdingName = h.Name
				}
			}
		} else if utf8.RuneCountInString(row.Description) > store.MaxDescriptionLen {
			addError(errDescriptionLen)
		}
		for _, c := range choices {
			if c.value == newOption && utf8.RuneCountInString(c.newName) > store.MaxNameLen {
				addError(c.list.Label + " name must be at most 50 characters.")
			}
		}
		entries = append(entries, store.RepeatingEntry{Type: row.Type, Amount: amount, Description: row.Description})
	}
	if len(errs) == 0 {
		if dups := inListDuplicates(filled); len(dups) > 0 {
			addError(errInListDuplicates + strings.Join(dups, ", "))
		}
	}
	if len(errs) > 0 {
		a.renderRepeating(w, http.StatusUnprocessableEntity, rows, date, errs, "")
		return
	}

	// Everything is valid: only now create inline "new..." entries.
	for i, row := range filled {
		if row.TakesHolding() {
			holdingID, err := a.resolveChoice(row.holding())
			if err != nil {
				a.serverError(w, err)
				return
			}
			entries[i].HoldingID = holdingID
			continue
		}
		set := store.DefaultListSet
		if row.TakesLevyLists() {
			set = store.LevyListSet
		}
		categoryID, err := a.resolveChoice(row.category(set))
		if err != nil {
			a.serverError(w, err)
			return
		}
		subcategoryID, err := a.resolveChoice(row.subcategory(set))
		if err != nil {
			a.serverError(w, err)
			return
		}
		if set == store.LevyListSet {
			entries[i].LevyCategoryID, entries[i].LevySubcategoryID = categoryID, subcategoryID
		} else {
			entries[i].CategoryID, entries[i].SubcategoryID = categoryID, subcategoryID
		}
	}
	if err := a.store.ReplaceRepeating(entries); err != nil {
		a.serverError(w, err)
		return
	}
	if r.PostFormValue("action") != "add" {
		http.Redirect(w, r, "/repeating?saved=1", http.StatusSeeOther)
		return
	}
	a.addTransactions(w, date, len(entries))
}

// collision names how a row collides with others of the same list when
// booked on one date (the Duplicate rule, ADR 0006, minus the date): rows with
// equal key collide, and label is what the error calls them. Each Type with
// its own rule adds a case here.
func (r repeatingRow) collision() (key, label string) {
	if r.TakesHolding() {
		// Whatever the Type: a Holding can be booked once per date.
		return "holding\x00" + asciiLower(r.holdingName), r.holdingName
	}
	return r.Type + "\x00" + asciiLower(r.Description), r.Description
}

// inListDuplicates returns the labels of the groups of rows that collide with
// each other, in first-seen order.
func inListDuplicates(rows []repeatingRow) []string {
	count := map[string]int{}
	for _, r := range rows {
		key, _ := r.collision()
		count[key]++
	}
	var labels []string
	for _, r := range rows {
		key, label := r.collision()
		if count[key] > 1 {
			labels = append(labels, label)
			count[key] = 0
		}
	}
	return labels
}

// asciiLower folds ASCII case only, like SQLite's NOCASE the Duplicate rule uses.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// addTransactions books the just-saved list on date. The list is already
// saved, so every failure here re-renders it as stored, without booking.
func (a *App) addTransactions(w http.ResponseWriter, date string, rowCount int) {
	var errs []string
	notice := ""
	switch msg := validateDate(date); {
	case msg != "":
		errs = []string{msg}
	case rowCount == 0:
		errs = []string{errNothingToAdd}
	default:
		booked, duplicates, err := a.store.BookRepeating(date)
		switch {
		case err != nil:
			a.serverError(w, err)
			return
		case len(duplicates) > 0:
			errs = []string{"Already exists on " + date + ": " + strings.Join(duplicates, ", ")}
		case booked == 1:
			notice = "1 Transaction added for " + date + "."
		default:
			notice = strconv.Itoa(booked) + " Transactions added for " + date + "."
		}
	}
	rows, err := a.savedRows()
	if err != nil {
		a.serverError(w, err)
		return
	}
	status := http.StatusOK
	if len(errs) > 0 {
		status = http.StatusUnprocessableEntity
	}
	a.renderRepeating(w, status, rows, date, errs, notice)
}

// repeatingTypeChoices reads "Expense or Income" from the Types a row can take.
func repeatingTypeChoices() string {
	var labels []string
	for _, s := range repeatingTypes() {
		labels = append(labels, s.Label)
	}
	return strings.Join(labels[:len(labels)-1], ", ") + " or " + labels[len(labels)-1]
}

func (r repeatingRow) holding() choice {
	return choice{store.HoldingList, r.Holding, r.NewHolding}
}

// TakesDefaultLists reports whether the row's Type draws from the Category and Subcategory lists.
func (r repeatingRow) TakesDefaultLists() bool {
	s, ok := repeatingSpec(r.Type)
	return ok && s.TakesDefaultLists()
}
