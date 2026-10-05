package store

import (
	"database/sql"
	"strings"
)

// Transaction types, as stored in the database.
const (
	TypeOpeningBalance = "opening_balance"
	TypeIncome         = "income"
	TypeExpense        = "expense"
	TypeInvestment     = "investment"
	TypeDivestment     = "divestment"
)

// Named is an entry of a flat named list (Category, Subcategory).
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
}

// TransactionRow is a Transaction with its Category and Subcategory names
// resolved, for display.
type TransactionRow struct {
	ID          int64
	Date        string
	Amount      int64
	Description string
	Type        string
	Category    string
	Subcategory string
}

// AddTransaction stores a Transaction and returns its id.
func (s *Store) AddTransaction(t NewTransaction) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO transactions (date, amount, description, type, category_id, subcategory_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		t.Date, t.Amount, t.Description, t.Type, nullID(t.CategoryID), nullID(t.SubcategoryID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListTransactions returns all Transactions, newest first.
func (s *Store) ListTransactions() ([]TransactionRow, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.date, t.amount, t.description, t.type,
		        COALESCE(c.name, ''), COALESCE(sc.name, '')
		 FROM transactions t
		 LEFT JOIN categories c ON c.id = t.category_id
		 LEFT JOIN subcategories sc ON sc.id = t.subcategory_id
		 ORDER BY t.date DESC, t.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TransactionRow
	for rows.Next() {
		var r TransactionRow
		if err := rows.Scan(&r.ID, &r.Date, &r.Amount, &r.Description, &r.Type, &r.Category, &r.Subcategory); err != nil {
			return nil, err
		}
		out = append(out, r)
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
	defer rows.Close()
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
