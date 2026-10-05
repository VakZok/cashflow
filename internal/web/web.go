// Package web is the HTTP interface of the app: routing, handlers and
// server-rendered templates. It is the only public seam of the application.
package web

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VakZok/money_manager_app/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

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
	tmpl, err := template.New("").Funcs(template.FuncMap{"euro": formatEuro}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		st.Close()
		return nil, err
	}
	a := &App{store: st, tmpl: tmpl, now: cfg.Now, mux: http.NewServeMux()}
	if a.now == nil {
		a.now = time.Now
	}
	a.mux.HandleFunc("GET /{$}", a.index)
	a.mux.HandleFunc("POST /transactions", a.createTransaction)
	return a, nil
}

// Handler returns the HTTP handler serving the app.
func (a *App) Handler() http.Handler { return a.mux }

// Close releases the database.
func (a *App) Close() error { return a.store.Close() }

type indexData struct {
	Transactions  []store.TransactionRow
	Categories    []store.Named
	Subcategories []store.Named
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	var d indexData
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
	a.render(w, http.StatusOK, "index.html", d)
}

func (a *App) createTransaction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	amount, _ := strconv.ParseInt(strings.TrimSpace(r.PostFormValue("amount")), 10, 64)
	catID, err := a.resolveCategory(r.PostFormValue("category"), r.PostFormValue("new_category"))
	if err != nil {
		a.serverError(w, err)
		return
	}
	subID, err := a.resolveSubcategory(r.PostFormValue("subcategory"), r.PostFormValue("new_subcategory"))
	if err != nil {
		a.serverError(w, err)
		return
	}
	_, err = a.store.AddTransaction(store.NewTransaction{
		Date:          strings.TrimSpace(r.PostFormValue("date")),
		Amount:        amount,
		Description:   strings.TrimSpace(r.PostFormValue("description")),
		Type:          store.TypeExpense,
		CategoryID:    catID,
		SubcategoryID: subID,
	})
	if err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// resolveCategory turns the form's select value ("new" or an id) plus the
// inline new name into a Category id.
func (a *App) resolveCategory(choice, newName string) (int64, error) {
	if choice == newOption {
		return a.store.EnsureCategory(newName)
	}
	id, _ := strconv.ParseInt(choice, 10, 64)
	return id, nil
}

func (a *App) resolveSubcategory(choice, newName string) (int64, error) {
	if choice == newOption {
		return a.store.EnsureSubcategory(newName)
	}
	id, _ := strconv.ParseInt(choice, 10, 64)
	return id, nil
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
