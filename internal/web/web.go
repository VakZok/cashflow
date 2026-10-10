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
	tmpl, err := template.New("").Funcs(template.FuncMap{"euro": formatEuro, "typeLabel": typeLabel}).ParseFS(templateFS, "templates/*.html")
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
	return a, nil
}

// Handler returns the HTTP handler serving the app.
func (a *App) Handler() http.Handler { return a.mux }

// Close releases the database.
func (a *App) Close() error { return a.store.Close() }

// entryForm is the Income/Expense entry form: submitted values (so a rejected
// submission keeps what was typed) and validation messages.
type entryForm struct {
	Type           string
	Date           string
	Amount         string
	Description    string
	Category       string
	NewCategory    string
	Subcategory    string
	NewSubcategory string
	Holding        string
	NewHolding     string
	Errors         []string
}

// HoldingType reports whether the form's Type records against a Holding
// (Investment, Divestment) instead of a Description, Category and Subcategory.
func (f entryForm) HoldingType() bool {
	return f.Type == store.TypeInvestment || f.Type == store.TypeDivestment
}

type indexData struct {
	Form          entryForm
	Transactions  []store.TransactionRow
	Categories    []store.Named
	Subcategories []store.Named
	Holdings      []store.Named
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	form := entryForm{Type: store.TypeExpense, Date: a.now().Format(dateLayout)}
	a.renderIndex(w, http.StatusOK, form)
}

func (a *App) renderIndex(w http.ResponseWriter, status int, form entryForm) {
	d := indexData{Form: form}
	var err error
	if d.Transactions, err = a.store.ListTransactions(); err != nil {
		a.serverError(w, err)
		return
	}
	if d.Categories, err = a.store.ListCategories(); err != nil {
		a.serverError(w, err)
		return
	}
	if d.Subcategories, err = a.store.ListSubcategories(); err != nil {
		a.serverError(w, err)
		return
	}
	if d.Holdings, err = a.store.ListHoldings(); err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, status, "index.html", d)
}

const dateLayout = "2006-01-02"

const errDuplicate = "An entry for this date and description already exists, edit it instead."

var wholeNumber = regexp.MustCompile(`^[0-9]+$`)

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
	}

	switch form.Type {
	case store.TypeExpense, store.TypeIncome, store.TypeInvestment, store.TypeDivestment:
	default:
		form.Errors = append(form.Errors, "Type must be Income, Expense, Investment or Divestment.")
	}
	if d, err := time.Parse(dateLayout, form.Date); err != nil {
		form.Errors = append(form.Errors, "Date must be in the format YYYY-MM-DD.")
	} else if f := d.Format(dateLayout); f < store.MinDate || f > store.MaxDate {
		form.Errors = append(form.Errors, "Date must be between 2000 and 2200.")
	}
	amount, err := strconv.ParseInt(form.Amount, 10, 64)
	if !wholeNumber.MatchString(form.Amount) || err != nil || amount < 1 || amount > store.MaxAmount {
		form.Errors = append(form.Errors, "Amount must be a whole number of euros between 1 and 100.000.000.")
	}
	var catOK, subOK, holdOK bool
	var err2 error
	switch {
	case form.HoldingType():
		if form.Description != "" || form.Category != "" || form.NewCategory != "" || form.Subcategory != "" || form.NewSubcategory != "" {
			form.Errors = append(form.Errors, "Investments and Divestments take a Holding, not a Description, Category or Subcategory.")
		}
		if utf8.RuneCountInString(form.NewHolding) > store.MaxNameLen {
			form.Errors = append(form.Errors, "Holding name must be at most 50 characters.")
		}
		if holdOK, err2 = a.validChoice(form.Holding, form.NewHolding, a.store.HoldingExists); err2 != nil {
			a.serverError(w, err2)
			return
		}
		if !holdOK {
			form.Errors = append(form.Errors, "Holding is required.")
		}
	case form.Type == store.TypeExpense || form.Type == store.TypeIncome:
		if form.Holding != "" || form.NewHolding != "" {
			form.Errors = append(form.Errors, "Income and Expense take no Holding.")
		}
		if form.Description == "" {
			form.Errors = append(form.Errors, "Description is required.")
		} else if utf8.RuneCountInString(form.Description) > store.MaxDescriptionLen {
			form.Errors = append(form.Errors, "Description must be at most 100 characters.")
		}
		if utf8.RuneCountInString(form.NewCategory) > store.MaxNameLen {
			form.Errors = append(form.Errors, "Category name must be at most 50 characters.")
		}
		if utf8.RuneCountInString(form.NewSubcategory) > store.MaxNameLen {
			form.Errors = append(form.Errors, "Subcategory name must be at most 50 characters.")
		}
		if catOK, err2 = a.validChoice(form.Category, form.NewCategory, a.store.CategoryExists); err2 != nil {
			a.serverError(w, err2)
			return
		}
		if !catOK {
			form.Errors = append(form.Errors, "Category is required.")
		}
		if subOK, err2 = a.validChoice(form.Subcategory, form.NewSubcategory, a.store.SubcategoryExists); err2 != nil {
			a.serverError(w, err2)
			return
		}
		if !subOK {
			form.Errors = append(form.Errors, "Subcategory is required.")
		}
		if len(form.Errors) == 0 {
			taken, err := a.store.DescriptionTaken(form.Date, form.Description)
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
	if form.HoldingType() {
		if nt.HoldingID, err = a.resolveChoice(form.Holding, form.NewHolding, a.store.EnsureHolding); err != nil {
			a.serverError(w, err)
			return
		}
	} else {
		nt.Description = form.Description
		if nt.CategoryID, err = a.resolveChoice(form.Category, form.NewCategory, a.store.EnsureCategory); err != nil {
			a.serverError(w, err)
			return
		}
		if nt.SubcategoryID, err = a.resolveChoice(form.Subcategory, form.NewSubcategory, a.store.EnsureSubcategory); err != nil {
			a.serverError(w, err)
			return
		}
	}
	if _, err = a.store.AddTransaction(nt); err != nil {
		if errors.Is(err, store.ErrDuplicate) { // lost a race with another submission
			form.Errors = append(form.Errors, errDuplicate)
			a.renderIndex(w, http.StatusUnprocessableEntity, form)
			return
		}
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// validChoice checks a select value ("new" with a name, or an existing id).
func (a *App) validChoice(choice, newName string, exists func(int64) (bool, error)) (bool, error) {
	if choice == newOption {
		return newName != "", nil
	}
	id, err := strconv.ParseInt(choice, 10, 64)
	if err != nil {
		return false, nil
	}
	return exists(id)
}

// resolveChoice turns a validated select value into an id, creating the
// entry for the inline "new..." choice.
func (a *App) resolveChoice(choice, newName string, ensure func(string) (int64, error)) (int64, error) {
	if choice == newOption {
		return ensure(newName)
	}
	return strconv.ParseInt(choice, 10, 64)
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
