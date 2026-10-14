// Package web is the HTTP interface of the app: routing, handlers and
// server-rendered templates. It is the only public seam of the application.
package web

import (
	"embed"
	"errors"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VakZok/money_manager_app/internal/store"
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

// App is the Money Manager web application.
type App struct {
	store *store.Store
	tmpl  *template.Template
	now   func() time.Time
	mux   *http.ServeMux
}

// New opens the database (migrating it) and builds the app.
func New(cfg Config) (*App, error) {
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("").Funcs(template.FuncMap{"euro": formatEuro, "typeLabel": typeLabel, "specsJSON": specsJSON}).ParseFS(templateFS, "templates/*.html")
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
	a.mux.HandleFunc("POST /transactions", a.createTransaction)
	a.mux.HandleFunc("GET /repeating", a.repeating)
	a.mux.HandleFunc("POST /repeating", a.saveRepeating)
	return a, nil
}

// Handler returns the HTTP handler serving the app.
func (a *App) Handler() http.Handler { return a.mux }

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
	Errors             []string
}

// choice is the value of a select with an inline "new..." option, bound to
// the list it draws from.
type choice struct {
	list    store.List
	value   string
	newName string
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

type indexData struct {
	Form              entryForm
	Types             []store.TypeSpec
	Transactions      []store.TransactionRow
	Categories        []store.Named
	Subcategories     []store.Named
	LevyCategories    []store.Named
	LevySubcategories []store.Named
	Holdings          []store.Named
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	form := entryForm{Type: store.TypeExpense, Date: a.now().Format(dateLayout)}
	a.renderIndex(w, http.StatusOK, form)
}

func (a *App) renderIndex(w http.ResponseWriter, status int, form entryForm) {
	// An unknown Type is rendered with the fields of the default one.
	form.Spec, _ = store.SpecFor(form.Type)
	if form.Spec.Type == "" {
		form.Spec = store.Types[0]
	}
	d := indexData{Form: form, Types: store.Types}
	var err error
	if d.Transactions, err = a.store.ListTransactions(); err != nil {
		a.serverError(w, err)
		return
	}
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
	a.render(w, status, "index.html", d)
}

const dateLayout = "2006-01-02"

const (
	errDuplicate        = "An entry for this date and description already exists, edit it instead."
	errDuplicateHolding = "An entry for this date and holding already exists, edit it instead."
)

const (
	errAmount         = "Amount must be a whole number of euros between 1 and 100.000.000."
	errDescriptionLen = "Description must be at most 100 characters."
)

// validateDate returns the error message for a date input, or "" if it is valid.
func validateDate(s string) string {
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		return "Date must be in the format YYYY-MM-DD."
	}
	if f := d.Format(dateLayout); f < store.MinDate || f > store.MaxDate {
		return "Date must be between 2000 and 2200."
	}
	return ""
}

var wholeNumber = regexp.MustCompile(`^[0-9]+$`)

// parseAmount reads a whole number of euros within the allowed bounds.
func parseAmount(s string) (int64, bool) {
	amount, err := strconv.ParseInt(s, 10, 64)
	ok := wholeNumber.MatchString(s) && err == nil && amount >= 1 && amount <= store.MaxAmount
	return amount, ok
}

func (a *App) createTransaction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	form := entryForm{
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
	}

	spec, known := store.SpecFor(form.Type)
	if !known {
		form.Errors = append(form.Errors, "Type must be "+typeChoices()+".")
	}
	if msg := validateDate(form.Date); msg != "" {
		form.Errors = append(form.Errors, msg)
	}
	amount, ok := parseAmount(form.Amount)
	if !ok {
		form.Errors = append(form.Errors, errAmount)
	}
	var err error
	if known {
		if err := a.validateFields(&form, spec); err != nil {
			a.serverError(w, err)
			return
		}
		if len(form.Errors) == 0 && spec.Description {
			taken, err := a.store.DescriptionTaken(form.Date, form.Description, form.Type)
			if err != nil {
				a.serverError(w, err)
				return
			}
			if taken {
				form.Errors = append(form.Errors, errDuplicate)
			}
		}
	}
	if len(form.Errors) > 0 {
		a.renderIndex(w, http.StatusUnprocessableEntity, form)
		return
	}

	// Everything is valid: only now create inline "new..." entries.
	nt := store.NewTransaction{Date: form.Date, Amount: amount, Type: form.Type}
	if spec.Description {
		nt.Description = form.Description
	}
	if spec.Lists != nil {
		categoryID, err := a.resolveChoice(form.category(*spec.Lists))
		if err != nil {
			a.serverError(w, err)
			return
		}
		subcategoryID, err := a.resolveChoice(form.subcategory(*spec.Lists))
		if err != nil {
			a.serverError(w, err)
			return
		}
		if spec.TakesLists(store.LevyListSet) {
			nt.LevyCategoryID, nt.LevySubcategoryID = categoryID, subcategoryID
		} else {
			nt.CategoryID, nt.SubcategoryID = categoryID, subcategoryID
		}
	}
	if spec.Holding {
		if nt.HoldingID, err = a.resolveChoice(form.holding()); err != nil {
			a.serverError(w, err)
			return
		}
	}
	if _, err = a.store.AddTransaction(nt); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			// A Holding collision is only caught here; for Types with a
			// Description this means a race with another submission.
			msg := errDuplicate
			if spec.Holding {
				msg = errDuplicateHolding
			}
			form.Errors = append(form.Errors, msg)
			a.renderIndex(w, http.StatusUnprocessableEntity, form)
			return
		}
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// validateFields appends an error for every field the Type does not take but
// was submitted, and for every field it takes but is missing or too long.
func (a *App) validateFields(form *entryForm, spec store.TypeSpec) error {
	set := store.DefaultListSet
	if spec.Lists != nil {
		set = *spec.Lists
	}
	if !spec.Description && form.Description != "" {
		form.Errors = append(form.Errors, spec.Label+" takes no Description.")
	}
	for _, other := range []store.ListSet{store.DefaultListSet, store.LevyListSet} {
		if !spec.TakesLists(other) && form.submitted(other) {
			form.Errors = append(form.Errors, spec.Label+" takes no "+other.Category.Label+" or "+other.Subcategory.Label+".")
		}
	}
	if !spec.Holding && (form.Holding != "" || form.NewHolding != "") {
		form.Errors = append(form.Errors, spec.Label+" takes no Holding.")
	}

	if spec.Description {
		if form.Description == "" {
			form.Errors = append(form.Errors, "Description is required.")
		} else if utf8.RuneCountInString(form.Description) > store.MaxDescriptionLen {
			form.Errors = append(form.Errors, errDescriptionLen)
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
			form.Errors = append(form.Errors, c.list.Label+" name must be at most 50 characters.")
		}
	}
	for _, c := range required {
		ok, err := a.validChoice(c)
		if err != nil {
			return err
		}
		if !ok {
			form.Errors = append(form.Errors, c.list.Label+" is required.")
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
