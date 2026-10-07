package store

import (
	"database/sql"
	"strings"
)

// Transaction types, as stored in the database.
const (
	TypeIncome     = "income"
	TypeExpense    = "expense"
	TypeInvestment = "investment"
	TypeDivestment = "divestment"
)

// Limits enforced by the database (see migrations/0001_init.sql) and
// checked by the web layer first so users get a message instead of an error.
const (
	MaxAmount         = 100_000_000
	MaxNameLen        = 50 // Category, Subcategory and Holding, in characters
	MaxDescriptionLen = 100

	MinDate = "2000-01-01"
	MaxDate = "2200-12-31"
)

// Named is an entry of a flat named list (Category, Subcategory, Holding).
type Named struct {
	ID   int64
	Name string
}

// NewTransaction is the data needed to record a Transaction.
type NewTransaction struct {
	Date          string // YYYY-MM-DD
	Amount        int64  // whole euros, > 0
	Description   string
	Type          string
	CategoryID    int64
	SubcategoryID int64
	HoldingID     int64
}

// TransactionRow is a Transaction with its Category, Subcategory and Holding names
// resolved, for display.
type TransactionRow struct {
	ID          int64
	Date        string
	Amount      int64
	Description string
	Type        string
	Category    string
	Subcategory string
	Holding     string
}

// AddTransaction stores a Transaction and returns its id.
func (s *Store) AddTransaction(t NewTransaction) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO transactions (date, amount, description, type, category_id, subcategory_id, holding_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		t.Date, t.Amount, nullString(t.Description), t.Type,
		nullID(t.CategoryID), nullID(t.SubcategoryID), nullID(t.HoldingID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListTransactions returns all Transactions, newest first.
func (s *Store) ListTransactions() ([]TransactionRow, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.date, t.amount, COALESCE(t.description, ''), t.type,
		        COALESCE(c.name, ''), COALESCE(sc.name, ''), COALESCE(h.name, '')
		 FROM transactions t
		 LEFT JOIN categories c ON c.id = t.category_id
		 LEFT JOIN subcategories sc ON sc.id = t.subcategory_id
		 LEFT JOIN holdings h ON h.id = t.holding_id
		 ORDER BY t.date DESC, t.id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []TransactionRow
	for rows.Next() {
		var r TransactionRow
		if err := rows.Scan(&r.ID, &r.Date, &r.Amount, &r.Description, &r.Type, &r.Category, &r.Subcategory, &r.Holding); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Totals returns the summed Income and Expense (whole euros) of Transactions
// dated from <= date < to (YYYY-MM-DD). Investments and Divestments are ignored.
func (s *Store) Totals(from, to string) (income, expense int64, err error) {
	err = s.db.QueryRow(
		`SELECT COALESCE(SUM(CASE WHEN type = ? THEN amount END), 0),
		        COALESCE(SUM(CASE WHEN type = ? THEN amount END), 0)
		 FROM transactions WHERE date >= ? AND date < ?`,
		TypeIncome, TypeExpense, from, to).Scan(&income, &expense)
	return income, expense, err
}

// Slice is one Category's or Subcategory's summed amount.
type Slice struct {
	Name   string
	Amount int64
}

// Breakdown returns the summed amounts of Transactions of one Type (income or
// expense) per Category, or per Subcategory when bySubcategory, dated
// from <= date < to, largest first.
func (s *Store) Breakdown(typ string, bySubcategory bool, from, to string) ([]Slice, error) {
	table, col := "categories", "category_id"
	if bySubcategory {
		table, col = "subcategories", "subcategory_id"
	}
	rows, err := s.db.Query(
		`SELECT n.name, SUM(t.amount) AS total FROM transactions t JOIN `+table+` n ON n.id = t.`+col+`
		 WHERE t.type = ? AND t.date >= ? AND t.date < ?
		 GROUP BY n.id ORDER BY total DESC, n.name`, typ, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Slice
	for rows.Next() {
		var sl Slice
		if err := rows.Scan(&sl.Name, &sl.Amount); err != nil {
			return nil, err
		}
		out = append(out, sl)
	}
	return out, rows.Err()
}

// ListCategories returns all Categories ordered by name.
func (s *Store) ListCategories() ([]Named, error) { return s.listNamed("categories") }

// ListSubcategories returns all Subcategories ordered by name.
func (s *Store) ListSubcategories() ([]Named, error) { return s.listNamed("subcategories") }

// EnsureCategory returns the id of the Category with the given name
// (case-insensitive), creating it if needed.
func (s *Store) EnsureCategory(name string) (int64, error) { return s.ensureNamed("categories", name) }

// EnsureSubcategory returns the id of the Subcategory with the given name
// (case-insensitive), creating it if needed.
func (s *Store) EnsureSubcategory(name string) (int64, error) {
	return s.ensureNamed("subcategories", name)
}

// ListHoldings returns all Holdings ordered by name.
func (s *Store) ListHoldings() ([]Named, error) { return s.listNamed("holdings") }

// EnsureHolding returns the id of the Holding with the given name
// (case-insensitive), creating it if needed.
func (s *Store) EnsureHolding(name string) (int64, error) { return s.ensureNamed("holdings", name) }

// HoldingExists reports whether a Holding with the id exists.
func (s *Store) HoldingExists(id int64) (bool, error) { return s.exists("holdings", id) }

// CategoryExists reports whether a Category with the id exists.
func (s *Store) CategoryExists(id int64) (bool, error) { return s.exists("categories", id) }

// SubcategoryExists reports whether a Subcategory with the id exists.
func (s *Store) SubcategoryExists(id int64) (bool, error) { return s.exists("subcategories", id) }

// table is always one of our own constant table names, never user input.
func (s *Store) listNamed(table string) ([]Named, error) {
	rows, err := s.db.Query("SELECT id, name FROM " + table + " ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Named
	for rows.Next() {
		var n Named
		if err := rows.Scan(&n.ID, &n.Name); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) ensureNamed(table, name string) (int64, error) {
	name = strings.TrimSpace(name)
	var id int64
	err := s.db.QueryRow("SELECT id FROM "+table+" WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := s.db.Exec("INSERT INTO "+table+" (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) exists(table string, id int64) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE id = ?", id).Scan(&n)
	return n > 0, err
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// nullString stores an empty string as NULL (fields a Type does not allow).
func nullString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
