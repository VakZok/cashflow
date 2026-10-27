package web

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VakZok/Cashflow/internal/store"
)

// repeatingRow is one row of the Repeating form: submitted values (so a
// rejected submission keeps what was typed) and their problems. Index only
// names its inputs.
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

	// The problems of a submitted row, which the shared error treatment
	// renders: the fields to highlight (by input name without the row
	// index), the amount bubble, and the mark of a row that duplicates an
	// earlier one.
	Missing   map[string]bool
	AmountBad bool
	Duplicate bool
}

// Bad reports whether the row's input of that name (without the row index) is highlighted.
func (r repeatingRow) Bad(field string) bool { return r.Missing[field] }

// missing records that the row's input of that name has nothing in it.
func (r *repeatingRow) missing(field string) {
	if r.Missing == nil {
		r.Missing = map[string]bool{}
	}
	r.Missing[field] = true
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
	DateMissing       bool // the month picker is highlighted
	// Save and Add are the Save List and Add Transactions buttons.
	Save, Add listButton
	// Pressed is the button ("save" or "add") whose submit failed, "" if the
	// page shows no failed submit; leaving the page carries it along.
	Pressed string
}

// listButton is a submit button of the Repeating form while it shows a
// problem or a success text; the zero listButton shows its usual label.
type listButton struct {
	Label string // "" is the usual label
	Flash bool   // Label is a success text, shown for a moment
}

// listProblems is what is wrong with a submitted list, as the pressed button shows it.
type listProblems struct {
	missing, duplicate bool
	invalid            bool // only a hand-crafted request gets here
	amount             bool // shown by the amount's bubble, not by the button
}

func (p listProblems) failed() bool { return p.missing || p.duplicate || p.invalid || p.amount }

// button is the pressed button showing the problems.
func (p listProblems) button() listButton {
	switch {
	case p.missing && p.duplicate:
		return listButton{Label: "Duplicate rows and missing fields"}
	case p.missing:
		return listButton{Label: "Missing fields"}
	case p.invalid:
		return listButton{Label: "Invalid input"}
	case p.duplicate:
		return listButton{Label: "Duplicate rows"}
	}
	return listButton{}
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

// repeating shows the rows carried in the address (the page state of a
// list left unsaved) or, without any, the saved list.
func (a *App) repeating(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var d repeatingData
	if q.Has("row") {
		d.Rows, d.Date = a.readRepeating(q)
		if len(d.Rows) > store.MaxRepeating {
			d.Rows = d.Rows[:store.MaxRepeating]
		}
		if err := a.dropStaleChoices(d.Rows); err != nil {
			a.serverError(w, err)
			return
		}
		if pressed := q.Get(failedParam); pressed == "save" || pressed == "add" {
			// A Month cleared before the failed Add Transactions is left out of the address.
			if pressed == "add" && !q.Has("date") {
				d.Date = ""
			}
			if err := a.recheckRepeating(&d, pressed); err != nil {
				a.serverError(w, err)
				return
			}
		}
	} else {
		rows, err := a.savedRows()
		if err != nil {
			a.serverError(w, err)
			return
		}
		d = repeatingData{Rows: rows, Date: a.defaultBookingDate()}
	}
	if q.Get("saved") == "1" {
		d.Save = listButton{Label: "List saved!", Flash: true}
	}
	a.renderRepeating(w, http.StatusOK, d)
}

// recheckRepeating runs the check of the pressed button's submit on the rows
// carried in the address and shows its result on d. It saves, creates and
// books nothing.
func (a *App) recheckRepeating(d *repeatingData, pressed string) error {
	c, err := a.checkRepeating(d.Rows)
	if err != nil {
		return err
	}
	var button listButton
	switch {
	case c.problems.failed():
		button = c.problems.button()
	case pressed == "add":
		button.Label, d.DateMissing = checkBooking(d.Date, len(c.filled))
	}
	if !c.problems.failed() && button.Label == "" {
		return nil
	}
	d.Pressed = pressed
	if pressed == "add" {
		d.Add = button
	} else {
		d.Save = button
	}
	return nil
}

// checkBooking is what is wrong with booking a list of rowCount rows on date, as Add
// Transactions shows it on its button ("" if nothing); dateMissing says the Month is empty.
func checkBooking(date string, rowCount int) (label string, dateMissing bool) {
	switch {
	case date == "":
		return "Missing fields", true
	case !validDate(date):
		return "Invalid input", false
	case rowCount == 0:
		return "List is empty", false
	}
	return "", false
}

// dropStaleChoices makes carried rows fit what exists now: a Type that is
// unknown becomes Expense, and a picked Category, Subcategory, Levy
// Category, Levy Subcategory or Holding that was deleted meanwhile becomes empty.
func (a *App) dropStaleChoices(rows []repeatingRow) error {
	exists := map[store.List]map[string]bool{}
	for _, l := range []store.List{store.DefaultListSet.Category, store.DefaultListSet.Subcategory, store.LevyListSet.Category, store.LevyListSet.Subcategory, store.HoldingList} {
		named, err := a.store.ListNamed(l)
		if err != nil {
			return err
		}
		exists[l] = map[string]bool{newOption: true}
		for _, n := range named {
			exists[l][strconv.FormatInt(n.ID, 10)] = true
		}
	}
	keep := func(l store.List, v *string) {
		if !exists[l][*v] {
			*v = ""
		}
	}
	for i := range rows {
		r := &rows[i]
		if _, ok := repeatingSpec(r.Type); !ok {
			r.Type = store.TypeExpense
		}
		keep(store.DefaultListSet.Category, &r.Category)
		keep(store.DefaultListSet.Subcategory, &r.Subcategory)
		keep(store.LevyListSet.Category, &r.LevyCategory)
		keep(store.LevyListSet.Subcategory, &r.LevySubcategory)
		keep(store.HoldingList, &r.Holding)
	}
	return nil
}

// renderRepeating renders the Repeating page with d's rows, date and buttons;
// it fills in the choices.
func (a *App) renderRepeating(w http.ResponseWriter, status int, d repeatingData) {
	if len(d.Rows) == 0 {
		d.Rows = []repeatingRow{{Type: store.TypeExpense}}
	}
	for i := range d.Rows {
		d.Rows[i].Index = i
	}
	d.Types, d.MaxRows = repeatingTypes(), store.MaxRepeating
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

// readRepeating reads the list's rows and Month from form values, which
// come from a submitted form or from a page address alike. Without a "date"
// value the Month is the default booking date.
func (a *App) readRepeating(v url.Values) (rows []repeatingRow, date string) {
	date = a.defaultBookingDate()
	if v.Has("date") {
		date = strings.TrimSpace(v.Get("date"))
		if len(date) == len("2006-01") {
			date += "-01" // month picker: book on the 1st
		}
	}
	for _, n := range v["row"] {
		rows = append(rows, repeatingRow{
			Type:           v.Get("type_" + n),
			Amount:         strings.TrimSpace(v.Get("amount_" + n)),
			Description:    strings.TrimSpace(v.Get("description_" + n)),
			Category:       v.Get("category_" + n),
			NewCategory:    strings.TrimSpace(v.Get("new_category_" + n)),
			Subcategory:    v.Get("subcategory_" + n),
			NewSubcategory: strings.TrimSpace(v.Get("new_subcategory_" + n)),

			LevyCategory:       v.Get("levy_category_" + n),
			NewLevyCategory:    strings.TrimSpace(v.Get("new_levy_category_" + n)),
			LevySubcategory:    v.Get("levy_subcategory_" + n),
			NewLevySubcategory: strings.TrimSpace(v.Get("new_levy_subcategory_" + n)),

			Holding:    v.Get("holding_" + n),
			NewHolding: strings.TrimSpace(v.Get("new_holding_" + n)),
		})
	}
	return rows, date
}

// checkedList is the outcome of checking a list: the filled rows (pointers
// into the checked rows, which carry their marks) with the entry each would
// be saved as, and the list's problems.
type checkedList struct {
	filled   []*repeatingRow
	entries  []store.RepeatingEntry
	problems listProblems
}

// checkRepeating checks the rows: it marks each row's problems and returns
// the list's. Fully blank rows are skipped. It creates and saves nothing.
func (a *App) checkRepeating(rows []repeatingRow) (checkedList, error) {
	var c checkedList
	for i := range rows {
		if !rows[i].blank() {
			c.filled = append(c.filled, &rows[i])
		}
	}
	p := &c.problems
	if len(c.filled) > store.MaxRepeating {
		p.invalid = true
	}
	c.entries = make([]store.RepeatingEntry, 0, len(c.filled))
	holdings, err := a.store.ListNamed(store.HoldingList)
	if err != nil {
		return c, err
	}
	for _, row := range c.filled {
		spec, ok := repeatingSpec(row.Type)
		if !ok {
			p.invalid = true
			continue
		}
		amount, ok := parseAmount(row.Amount)
		switch {
		case row.Amount == "":
			row.missing("amount")
		case !ok:
			row.AmountBad, p.amount = true, true
		}
		if spec.Holding {
			row.Description = ""
			if row.Holding == newOption {
				row.holdingName = row.NewHolding
			}
			for _, h := range holdings {
				if row.Holding == strconv.FormatInt(h.ID, 10) {
					row.holdingName = h.Name
				}
			}
		} else if row.Description == "" {
			row.missing("description")
		} else if utf8.RuneCountInString(row.Description) > store.MaxDescriptionLen {
			p.invalid = true
		}
		for _, ch := range row.choices(spec) {
			if utf8.RuneCountInString(ch.newName) > store.MaxNameLen {
				p.invalid = true
			}
			ok, err := a.validChoice(ch)
			if err != nil {
				return c, err
			}
			switch {
			case ok:
			case ch.value == "":
				row.missing(ch.inputName())
			case ch.value == newOption && ch.newName == "":
				row.missing("new_" + ch.inputName())
			default: // an id the UI never offers
				p.invalid = true
			}
		}
		p.missing = p.missing || len(row.Missing) > 0
		c.entries = append(c.entries, store.RepeatingEntry{Type: row.Type, Amount: amount, Description: row.Description})
	}
	p.duplicate = markInListDuplicates(c.filled)
	return c, nil
}

// saveRepeating replaces the stored list with the submitted one. Fully blank
// rows are dropped; if any other row is invalid nothing is saved or created.
func (a *App) saveRepeating(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rows, date := a.readRepeating(r.PostForm)
	c, err := a.checkRepeating(rows)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if c.problems.failed() {
		d := repeatingData{Rows: rows, Date: date, Pressed: "save"}
		if r.PostFormValue("action") == "add" {
			d.Pressed = "add"
			d.Add = c.problems.button()
		} else {
			d.Save = c.problems.button()
		}
		a.renderRepeating(w, http.StatusUnprocessableEntity, d)
		return
	}
	a.saveAndBookRepeating(w, r, date, c)
}

// saveAndBookRepeating saves a list that passed the check and, for Add
// Transactions, books it.
func (a *App) saveAndBookRepeating(w http.ResponseWriter, r *http.Request, date string, c checkedList) {
	filled, entries := c.filled, c.entries
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
// equal key collide. ok is false for a row that collides with none, as it
// lacks the field its rule compares. Each Type with its own rule adds a case here.
func (r repeatingRow) collision() (key string, ok bool) {
	if _, known := repeatingSpec(r.Type); !known {
		return "", false
	}
	if r.TakesHolding() {
		// Whatever the Type: a Holding can be booked once per date.
		return "holding\x00" + asciiLower(r.holdingName), r.holdingName != ""
	}
	return r.Type + "\x00" + asciiLower(r.Description), r.Description != ""
}

// markInListDuplicates marks every row that collides with an earlier one,
// so the first row of each set of colliding rows stays unmarked, and reports
// whether it marked any.
func markInListDuplicates(rows []*repeatingRow) bool {
	seen := map[string]bool{}
	marked := false
	for _, r := range rows {
		key, ok := r.collision()
		switch {
		case !ok:
		case seen[key]:
			r.Duplicate, marked = true, true
		default:
			seen[key] = true
		}
	}
	return marked
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
// The result is shown on the Add Transactions button.
func (a *App) addTransactions(w http.ResponseWriter, date string, rowCount int) {
	d := repeatingData{Date: date}
	d.Add.Label, d.DateMissing = checkBooking(date, rowCount)
	if d.Add.Label == "" {
		booked, skipped, err := a.store.BookRepeating(date)
		if err != nil {
			a.serverError(w, err)
			return
		}
		d.Add = listButton{Label: addedText(booked, skipped), Flash: true}
	}
	var err error
	if d.Rows, err = a.savedRows(); err != nil {
		a.serverError(w, err)
		return
	}
	status := http.StatusOK
	if !d.Add.Flash {
		status = http.StatusUnprocessableEntity
		d.Pressed = "add"
	}
	a.renderRepeating(w, status, d)
}

// addedText is the Add Transactions success text for booked new and skipped
// existing Transactions.
func addedText(booked, skipped int) string {
	switch {
	case booked == 0:
		return "Transactions already added. Skipping and saving List."
	case skipped == 0:
		return "Added " + transactionCount(booked) + " and saved List!"
	}
	return "Added " + transactionCount(booked) + ", skipped " + existingCount(skipped) + ", and saved List!"
}

// existingCount reads "1 existing one" or "n existing ones".
func existingCount(n int) string {
	if n == 1 {
		return "1 existing one"
	}
	return strconv.Itoa(n) + " existing ones"
}

// transactionCount reads "1 Transaction" or "n Transactions".
func transactionCount(n int) string {
	if n == 1 {
		return "1 Transaction"
	}
	return strconv.Itoa(n) + " Transactions"
}

func (r repeatingRow) holding() choice {
	return choice{store.HoldingList, r.Holding, r.NewHolding}
}

// TakesDefaultLists reports whether the row's Type draws from the Category and Subcategory lists.
func (r repeatingRow) TakesDefaultLists() bool {
	s, ok := repeatingSpec(r.Type)
	return ok && s.TakesDefaultLists()
}
