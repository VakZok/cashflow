// Package web is the HTTP interface of the app: routing, handlers and
// server-rendered templates. It is the only public seam of the application.
package web

import (
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VakZok/Cashflow/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Config configures an App.
type Config struct {
	DBPath string
	// Now returns the current time; defaults to time.Now.
	Now func() time.Time
}

// App is the Cashflow web application.
type App struct {
	store *store.Store
	tmpl  *template.Template
	now   func() time.Time
	mux   *http.ServeMux
	// handler wraps mux; built once in New and served by Handler.
	handler http.Handler
}

// New opens the database (migrating it) and builds the app.
func New(cfg Config) (*App, error) {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("").Funcs(template.FuncMap{"euro": formatEuro, "signedAmount": signedAmount, "typeLabel": typeLabel, "specsJSON": specsJSON}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	a := &App{store: st, tmpl: tmpl, now: cfg.Now, mux: http.NewServeMux()}
	if a.now == nil {
		a.now = time.Now
	}
	a.mux.HandleFunc("GET /{$}", a.index)
	a.mux.Handle("GET /static/", http.FileServerFS(staticFS))
	a.mux.HandleFunc("GET /report", a.report)
	a.mux.HandleFunc("GET /transactions", a.transactions)
	a.mux.HandleFunc("POST /transactions", a.createTransaction)
	a.mux.HandleFunc("POST /transactions/{id}", a.updateTransaction)
	a.mux.HandleFunc("POST /transactions/{id}/delete", a.deleteTransaction)
	a.mux.HandleFunc("GET /categories", a.categories)
	a.mux.HandleFunc("POST /categories", a.addCategory)
	a.mux.HandleFunc("POST /categories/{id}", a.renameCategory)
	a.mux.HandleFunc("POST /categories/{id}/delete", a.deleteCategory)
	a.mux.HandleFunc("GET /repeating", a.repeating)
	a.mux.HandleFunc("POST /repeating", a.saveRepeating)
	a.handler = protect(a.mux)
	return a, nil
}

// protect rejects cross-origin writes and forbids framing of every response,
// including rejections (ADR 0003: the access path does not cover other sites
// open in the user's own browser).
func protect(next http.Handler) http.Handler {
	protected := http.NewCrossOriginProtection().Handler(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'")
		protected.ServeHTTP(w, r)
	})
}

// Handler returns the HTTP handler serving the app.
func (a *App) Handler() http.Handler { return a.handler }

// Close releases the database.
func (a *App) Close() error { return a.store.Close() }

// entryForm is the Transaction entry form: submitted values (so a rejected
// submission keeps what was typed) and validation messages.
type entryForm struct {
	Type           string
	Spec           store.TypeSpec // the fields Type takes
	Date           string
	Amount         string
	Description    string
	Category       string
	NewCategory    string
	Subcategory    string
	NewSubcategory string
	Holding        string
	NewHolding     string
	// Levy Category and Levy Subcategory are separate fields from Category and
	// Subcategory: the Types of the two list sets never share an input.
	LevyCategory       string
	NewLevyCategory    string
	LevySubcategory    string
	NewLevySubcategory string

	// The problems of a submitted form, which the shared error treatment
	// renders: the fields to highlight (by input name), the amount bubble and
	// the message of the submit button.
	Missing   map[string]bool
	AmountBad bool
	Problem   problem
}

// problem is what is wrong with a submitted form, as the submit button shows it.
// A form with several problems shows the highest one.
type problem int

const (
	noProblem        problem = iota
	problemAmount            // shown by the amount's bubble, not by the button
	problemDuplicate         // "Already exists"
	problemInvalid           // "Invalid input": only a hand-crafted request gets here
	problemMissing           // "Missing fields"
)

// ButtonMessage is the submit button's label while the form has a problem, "" if
// the button keeps its normal label.
func (f entryForm) ButtonMessage() string {
	switch f.Problem {
	case problemMissing:
		return "Missing fields"
	case problemInvalid:
		return "Invalid input"
	case problemDuplicate:
		return "Already exists"
	}
	return ""
}

// Bad reports whether the input of that name is highlighted.
func (f entryForm) Bad(field string) bool { return f.Missing[field] }

// failed reports whether the form has a problem.
func (f entryForm) failed() bool { return f.Problem != noProblem }

// Failed is failed for the templates, which mark the form so leaving the page carries the problems along.
func (f entryForm) Failed() bool { return f.failed() }

// failedParam is the address parameter that marks a page left while it showed
// the result of a failed submit, naming the pressed button. The page then
// re-checks its carried input instead of showing no problems.
const failedParam = "failed"

// recheck runs the submit's check on input carried in an address, when the
// address carries the failed marker. Nothing is saved or created.
func (a *App) recheck(q url.Values, form *entryForm, excludeID int64) error {
	if !q.Has(failedParam) {
		return nil
	}
	form.Date, form.Amount, form.Description = strings.TrimSpace(form.Date), strings.TrimSpace(form.Amount), strings.TrimSpace(form.Description)
	form.NewCategory, form.NewSubcategory, form.NewHolding = strings.TrimSpace(form.NewCategory), strings.TrimSpace(form.NewSubcategory), strings.TrimSpace(form.NewHolding)
	form.NewLevyCategory, form.NewLevySubcategory = strings.TrimSpace(form.NewLevyCategory), strings.TrimSpace(form.NewLevySubcategory)
	_, _, err := a.checkEntry(form, excludeID)
	return err
}

// fail records a problem.
func (f *entryForm) fail(p problem) { f.Problem = max(f.Problem, p) }

// missing records that the input of that name has nothing in it.
func (f *entryForm) missing(field string) {
	if f.Missing == nil {
		f.Missing = map[string]bool{}
	}
	f.Missing[field] = true
	f.fail(problemMissing)
}

// choice is the value of a select with an inline "new..." option, bound to
// the list it draws from.
type choice struct {
	list    store.List
	value   string
	newName string
}

// inputName is the name of the select of the list in the entry form; its
// new-name input is "new_" and that.
func (c choice) inputName() string {
	switch c.list {
	case store.LevyListSet.Category:
		return "levy_category"
	case store.LevyListSet.Subcategory:
		return "levy_subcategory"
	case store.DefaultListSet.Category:
		return "category"
	case store.DefaultListSet.Subcategory:
		return "subcategory"
	}
	return "holding"
}

func (f entryForm) category(set store.ListSet) choice {
	if set == store.LevyListSet {
		return choice{set.Category, f.LevyCategory, f.NewLevyCategory}
	}
	return choice{set.Category, f.Category, f.NewCategory}
}

func (f entryForm) subcategory(set store.ListSet) choice {
	if set == store.LevyListSet {
		return choice{set.Subcategory, f.LevySubcategory, f.NewLevySubcategory}
	}
	return choice{set.Subcategory, f.Subcategory, f.NewSubcategory}
}

// submitted reports whether the form carries a value for either list of the set.
func (f entryForm) submitted(set store.ListSet) bool {
	c, s := f.category(set), f.subcategory(set)
	return c.value != "" || c.newName != "" || s.value != "" || s.newName != ""
}

func (f entryForm) holding() choice {
	return choice{store.HoldingList, f.Holding, f.NewHolding}
}

// formLists are the choices the entry form's selects offer.
type formLists struct {
	Types             []store.TypeSpec
	Categories        []store.Named
	Subcategories     []store.Named
	LevyCategories    []store.Named
	LevySubcategories []store.Named
	Holdings          []store.Named
}

// fieldsData is what the shared "fields" template renders: one entry form,
// whose element ids start with P so the Add form and an edit row can coexist.
type fieldsData struct {
	P    string
	Form entryForm
	*formLists
}

// editView is a Transaction's row shown as a form, with where Discard goes and
// the hidden "return" value that takes Apply and Delete back to the same list.
type editView struct {
	ID      int64
	Fields  *fieldsData
	Return  string
	Discard string
	// Address is the page with this edit open: where leaving the page carries
	// an edit that failed to apply, which is shown at its POST target.
	Address string
}

func newEditView(e *editing, lists *formLists, ret returnTo) *editView {
	return &editView{
		ID:      e.ID,
		Fields:  &fieldsData{P: "edit-", Form: withSpec(e.Form), formLists: lists},
		Return:  ret.String(),
		Discard: ret.URL(),
		Address: ret.URL() + "&edit=" + strconv.FormatInt(e.ID, 10),
	}
}

type indexData struct {
	Add fieldsData
	// Added makes the Add button show its success text for a moment.
	Added bool
}

// addedQuery is what the redirect after a successful add carries, so the page it lands on shows the success text.
const addedQuery = "added=1"

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lists, err := a.formLists()
	if err != nil {
		a.serverError(w, err)
		return
	}
	form := formFromQuery(q, lists, a.defaultAddForm())
	if err := a.recheck(q, &form, 0); err != nil {
		a.serverError(w, err)
		return
	}
	a.renderIndexWith(w, http.StatusOK, form, lists, q.Get("added") == "1")
}

// formFromQuery fills the Add form from the address, with the field names the
// form submits. The values only fill the form: nothing is validated into a
// problem. An unknown Type falls back to the base form's, a choice that does
// not exist (any more) to empty, and a missing date to the base form's.
func formFromQuery(q url.Values, lists *formLists, base entryForm) entryForm {
	f := base
	if _, known := store.SpecFor(q.Get("type")); known {
		f.Type = q.Get("type")
	}
	if d := q.Get("date"); d != "" {
		f.Date = d
	}
	f.Amount = q.Get("amount")
	f.Description = q.Get("description")
	// pick keeps a choice that exists in the list, or "new" with the name typed for it.
	pick := func(name string, options []store.Named) (value, newName string) {
		v := q.Get(name)
		if v == newOption {
			return v, q.Get("new_" + name)
		}
		for _, o := range options {
			if strconv.FormatInt(o.ID, 10) == v {
				return v, ""
			}
		}
		return "", ""
	}
	f.Category, f.NewCategory = pick("category", lists.Categories)
	f.Subcategory, f.NewSubcategory = pick("subcategory", lists.Subcategories)
	f.LevyCategory, f.NewLevyCategory = pick("levy_category", lists.LevyCategories)
	f.LevySubcategory, f.NewLevySubcategory = pick("levy_subcategory", lists.LevySubcategories)
	f.Holding, f.NewHolding = pick("holding", lists.Holdings)
	return f
}

// editParamPrefix starts the names of the edit form's fields in the address, so they do not mix with the filter's.
const editParamPrefix = "e_"

// editFromQuery is the form of an edit whose unapplied changes the address carries, with the
// field names unprefixed; the stored form if it carries none. A carried form replaces the stored values
// entirely: a field left out was emptied.
func editFromQuery(q url.Values, lists *formLists, stored entryForm) entryForm {
	if !q.Has(editParamPrefix + "type") {
		return stored
	}
	carried := url.Values{}
	for k, v := range q {
		if name, ok := strings.CutPrefix(k, editParamPrefix); ok {
			carried[name] = v
		}
	}
	return formFromQuery(carried, lists, entryForm{Type: stored.Type, Date: stored.Date})
}

// keptQuery is the address the Add page is redirected to after a successful
// add: the success flag, and the date, Type and choices the next Transaction
// is likely to reuse. A "new..." choice is carried as the entry the add created.
func keptQuery(form entryForm, spec store.TypeSpec, nt store.NewTransaction) string {
	q := url.Values{"date": {form.Date}, "type": {form.Type}}
	id := func(name string, v int64) {
		if v != 0 {
			q.Set(name, strconv.FormatInt(v, 10))
		}
	}
	if spec.TakesDefaultLists() {
		id("category", nt.CategoryID)
		id("subcategory", nt.SubcategoryID)
	}
	if spec.TakesLevyLists() {
		id("levy_category", nt.LevyCategoryID)
		id("levy_subcategory", nt.LevySubcategoryID)
	}
	if spec.Holding {
		id("holding", nt.HoldingID)
	}
	return q.Encode() + "&" + addedQuery
}

// defaultAddForm is the empty Add form: an Expense dated today.
func (a *App) defaultAddForm() entryForm {
	return entryForm{Type: store.TypeExpense, Date: a.now().Format(dateLayout)}
}

// editing is the Transaction whose row is shown as a form, with that form's state.
type editing struct {
	ID   int64
	Form entryForm
}

// formOf is the entry form prefilled with a stored Transaction.
func formOf(t store.NewTransaction) entryForm {
	id := func(v int64) string {
		if v == 0 {
			return ""
		}
		return strconv.FormatInt(v, 10)
	}
	return entryForm{
		Type: t.Type, Date: t.Date, Amount: strconv.FormatInt(t.Amount, 10), Description: t.Description,
		Category: id(t.CategoryID), Subcategory: id(t.SubcategoryID), Holding: id(t.HoldingID),
		LevyCategory: id(t.LevyCategoryID), LevySubcategory: id(t.LevySubcategoryID),
	}
}

func (a *App) renderIndex(w http.ResponseWriter, status int, add entryForm, added bool) {
	lists, err := a.formLists()
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.renderIndexWith(w, status, add, lists, added)
}

func (a *App) renderIndexWith(w http.ResponseWriter, status int, add entryForm, lists *formLists, added bool) {
	a.render(w, status, "index.html", indexData{Add: fieldsData{Form: withSpec(add), formLists: lists}, Added: added})
}

// formLists loads the choices of the entry form's selects.
func (a *App) formLists() (*formLists, error) {
	lists := &formLists{Types: store.Types}
	var err error
	if lists.Categories, err = a.store.ListNamed(store.DefaultListSet.Category); err != nil {
		return nil, err
	}
	if lists.Subcategories, err = a.store.ListNamed(store.DefaultListSet.Subcategory); err != nil {
		return nil, err
	}
	if lists.LevyCategories, err = a.store.ListNamed(store.LevyListSet.Category); err != nil {
		return nil, err
	}
	if lists.LevySubcategories, err = a.store.ListNamed(store.LevyListSet.Subcategory); err != nil {
		return nil, err
	}
	if lists.Holdings, err = a.store.ListNamed(store.HoldingList); err != nil {
		return nil, err
	}
	return lists, nil
}

// withSpec sets the form's Spec to the fields its Type takes. An unknown Type
// is rendered with the fields of the default one.
func withSpec(form entryForm) entryForm {
	form.Spec, _ = store.SpecFor(form.Type)
	if form.Spec.Type == "" {
		form.Spec = store.Types[0]
	}
	return form
}

const dateLayout = "2006-01-02"

// validDate reports whether a date input is a date in YYYY-MM-DD between store.MinDate and store.MaxDate.
func validDate(s string) bool {
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		return false
	}
	f := d.Format(dateLayout)
	return f >= store.MinDate && f <= store.MaxDate
}

var wholeNumber = regexp.MustCompile(`^[0-9]+$`)

// parseAmount reads a whole number of euros within the allowed bounds.
func parseAmount(s string) (int64, bool) {
	amount, err := strconv.ParseInt(s, 10, 64)
	ok := wholeNumber.MatchString(s) && err == nil && amount >= 1 && amount <= store.MaxAmount
	return amount, ok
}

func (a *App) createTransaction(w http.ResponseWriter, r *http.Request) {
	form, ok := readForm(w, r)
	if !ok {
		return
	}
	nt, err := a.prepare(&form, 0)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if form.failed() {
		a.renderIndex(w, http.StatusUnprocessableEntity, form, false)
		return
	}
	if _, err = a.store.AddTransaction(nt); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			form.fail(problemDuplicate)
			a.renderIndex(w, http.StatusUnprocessableEntity, form, false)
			return
		}
		a.serverError(w, err)
		return
	}
	spec, _ := store.SpecFor(form.Type)
	http.Redirect(w, r, "/?"+keptQuery(form, spec, nt), http.StatusSeeOther)
}

// updateTransaction replaces the Transaction in the path with the submitted
// form, under the same rules as creating one.
func (a *App) updateTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	form, ok := readForm(w, r)
	if !ok {
		return
	}
	if _, err := a.store.GetTransaction(id); errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.serverError(w, err)
		return
	}
	nt, err := a.prepare(&form, id)
	if err != nil {
		a.serverError(w, err)
		return
	}
	if !form.failed() {
		if err = a.store.UpdateTransaction(id, nt); errors.Is(err, store.ErrDuplicate) {
			form.fail(problemDuplicate)
		} else if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		} else if err != nil {
			a.serverError(w, err)
			return
		}
	}
	ret := parseReturn(r.PostFormValue("return"), a.now())
	if form.failed() {
		// The Add page has no rows to edit: an unrecognised return shows the row on
		// the Transactions page, at the page it sits on.
		if ret.Page == 0 {
			if ret.Page, err = a.store.TransactionPage(id, ret.Filter.store()); err != nil {
				a.serverError(w, err)
				return
			}
		}
		a.renderTransactions(w, http.StatusUnprocessableEntity, ret.Page, ret.Filter, &editing{ID: id, Form: form})
		return
	}
	anchor := "#tx-" + strconv.FormatInt(id, 10)
	if ret.Page == 0 {
		http.Redirect(w, r, "/"+anchor, http.StatusSeeOther)
		return
	}
	// From the Transactions page the row is followed to the page it now sits on
	// under the same filter. A row edited out of the filter is gone from the
	// list: back to the same page, clamped.
	page, err := a.store.TransactionPage(id, ret.Filter.store())
	switch {
	case errors.Is(err, store.ErrNotFound):
		if ret, err = a.clamped(ret); err != nil {
			a.serverError(w, err)
			return
		}
		http.Redirect(w, r, ret.URL(), http.StatusSeeOther)
	case err != nil:
		a.serverError(w, err)
	default:
		ret.Page = page
		http.Redirect(w, r, ret.URL()+anchor, http.StatusSeeOther)
	}
}

// deleteTransaction removes the Transaction in the path.
func (a *App) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteTransaction(id); errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		a.serverError(w, err)
		return
	}
	// Deleting the only row of the last page lands on the new last page.
	ret, err := a.clamped(parseReturn(r.PostFormValue("return"), a.now()))
	if err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, ret.URL(), http.StatusSeeOther)
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

// readForm reads the submitted entry form, writing the response itself if the
// request is malformed.
func readForm(w http.ResponseWriter, r *http.Request) (entryForm, bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return entryForm{}, false
	}
	return entryForm{
		Type:           r.PostFormValue("type"),
		Date:           strings.TrimSpace(r.PostFormValue("date")),
		Amount:         strings.TrimSpace(r.PostFormValue("amount")),
		Description:    strings.TrimSpace(r.PostFormValue("description")),
		Category:       r.PostFormValue("category"),
		NewCategory:    strings.TrimSpace(r.PostFormValue("new_category")),
		Subcategory:    r.PostFormValue("subcategory"),
		NewSubcategory: strings.TrimSpace(r.PostFormValue("new_subcategory")),
		Holding:        r.PostFormValue("holding"),
		NewHolding:     strings.TrimSpace(r.PostFormValue("new_holding")),

		LevyCategory:       r.PostFormValue("levy_category"),
		NewLevyCategory:    strings.TrimSpace(r.PostFormValue("new_levy_category")),
		LevySubcategory:    r.PostFormValue("levy_subcategory"),
		NewLevySubcategory: strings.TrimSpace(r.PostFormValue("new_levy_subcategory")),
	}, true
}

// prepare validates the form, recording its problems on it, and if it is valid
// returns the Transaction it describes, creating inline "new..." entries.
// excludeID is the Transaction being edited (0 on create): it is never a
// Duplicate of itself.
func (a *App) prepare(form *entryForm, excludeID int64) (store.NewTransaction, error) {
	spec, amount, err := a.checkEntry(form, excludeID)
	var nt store.NewTransaction
	if err != nil || form.failed() {
		return nt, err
	}

	// Everything is valid: only now create inline "new..." entries.
	nt = store.NewTransaction{Date: form.Date, Amount: amount, Type: form.Type}
	if spec.Description {
		nt.Description = form.Description
	}
	if spec.Lists != nil {
		categoryID, err := a.resolveChoice(form.category(*spec.Lists))
		if err != nil {
			return nt, err
		}
		subcategoryID, err := a.resolveChoice(form.subcategory(*spec.Lists))
		if err != nil {
			return nt, err
		}
		if spec.TakesLists(store.LevyListSet) {
			nt.LevyCategoryID, nt.LevySubcategoryID = categoryID, subcategoryID
		} else {
			nt.CategoryID, nt.SubcategoryID = categoryID, subcategoryID
		}
	}
	if spec.Holding {
		if nt.HoldingID, err = a.resolveChoice(form.holding()); err != nil {
			return nt, err
		}
	}
	return nt, nil
}

// checkEntry validates the form and records its problems on it, with the Type's
// spec and the amount it read. It creates and saves nothing, so a page can
// re-check carried input. excludeID is as for prepare.
func (a *App) checkEntry(form *entryForm, excludeID int64) (store.TypeSpec, int64, error) {
	spec, known := store.SpecFor(form.Type)
	if !known {
		form.fail(problemInvalid)
	}
	if form.Date == "" {
		form.missing("date")
	} else if !validDate(form.Date) {
		form.fail(problemInvalid)
	}
	amount, ok := parseAmount(form.Amount)
	if !ok {
		form.AmountBad = true
		form.fail(problemAmount)
	}
	if known {
		if err := a.validateFields(form, spec); err != nil {
			return spec, amount, err
		}
		if !form.failed() && spec.Description {
			taken, err := a.store.DescriptionTaken(form.Date, form.Description, form.Type, excludeID)
			if err != nil {
				return spec, amount, err
			}
			if taken {
				form.fail(problemDuplicate)
			}
		}
	}
	return spec, amount, nil
}

// validateFields records a problem for every field the Type does not take but
// was submitted, and for every field it takes but is missing or too long.
func (a *App) validateFields(form *entryForm, spec store.TypeSpec) error {
	set := store.DefaultListSet
	if spec.Lists != nil {
		set = *spec.Lists
	}
	if !spec.Description && form.Description != "" {
		form.fail(problemInvalid)
	}
	for _, other := range []store.ListSet{store.DefaultListSet, store.LevyListSet} {
		if !spec.TakesLists(other) && form.submitted(other) {
			form.fail(problemInvalid)
		}
	}
	if !spec.Holding && (form.Holding != "" || form.NewHolding != "") {
		form.fail(problemInvalid)
	}

	if spec.Description {
		if form.Description == "" {
			form.missing("description")
		} else if utf8.RuneCountInString(form.Description) > store.MaxDescriptionLen {
			form.fail(problemInvalid)
		}
	}
	var required []choice
	if spec.Lists != nil {
		required = append(required, form.category(set), form.subcategory(set))
	}
	if spec.Holding {
		required = append(required, form.holding())
	}
	for _, c := range required {
		if utf8.RuneCountInString(c.newName) > store.MaxNameLen {
			form.fail(problemInvalid)
		}
	}
	for _, c := range required {
		ok, err := a.validChoice(c)
		if err != nil {
			return err
		}
		switch {
		case ok:
		case c.value == "":
			form.missing(c.inputName())
		case c.value == newOption && c.newName == "":
			form.missing("new_" + c.inputName())
		default: // an id the UI never offers
			form.fail(problemInvalid)
		}
	}
	return nil
}

// validChoice checks a select value ("new" with a name, or an existing id).
func (a *App) validChoice(c choice) (bool, error) {
	if c.value == newOption {
		return c.newName != "", nil
	}
	id, err := strconv.ParseInt(c.value, 10, 64)
	if err != nil {
		return false, nil
	}
	return a.store.NamedExists(c.list, id)
}

// resolveChoice turns a validated select value into an id, creating the
// entry for the inline "new..." choice.
func (a *App) resolveChoice(c choice) (int64, error) {
	if c.value == newOption {
		return a.store.EnsureNamed(c.list, c.newName)
	}
	return strconv.ParseInt(c.value, 10, 64)
}

// newOption is the select value of the inline "new..." choice.
const newOption = "new"

func (a *App) serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func (a *App) render(w http.ResponseWriter, status int, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := a.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
