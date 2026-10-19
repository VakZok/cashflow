package store

import (
	"database/sql"
	"strings"
)

// Transaction types, as stored in the database.
const (
	TypeIncome     = "income"
	TypeExpense    = "expense"
	TypeLevy       = "levy"
	TypeRebate     = "rebate"
	TypeInvestment = "investment"
	TypeDivestment = "divestment"
)

// Limits enforced by the database (see migrations/0001_init.sql) and
// checked by the web layer first so users get a message instead of an error.
const (
	MaxAmount         = 100_000_000
	MaxNameLen        = 50 // Category, Subcategory, Levy Category, Levy Subcategory and Holding, in characters
	MaxDescriptionLen = 100

	MinDate = "2000-01-01"
	MaxDate = "2200-12-31"
)

// Named is an entry of a flat named list (Category, Subcategory, Levy Category,
// Levy Subcategory, Holding).
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

	LevyCategoryID    int64
	LevySubcategoryID int64
}

// TransactionRow is a Transaction with its Category, Subcategory (Levy Category and
// Levy Subcategory for a Levy or Rebate) and Holding names
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
		`INSERT INTO transactions (date, amount, description, type, category_id, subcategory_id, holding_id,
		                           levy_category_id, levy_subcategory_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Date, t.Amount, nullString(t.Description), t.Type,
		nullID(t.CategoryID), nullID(t.SubcategoryID), nullID(t.HoldingID),
		nullID(t.LevyCategoryID), nullID(t.LevySubcategoryID))
	if err != nil {
		return 0, translate(err)
	}
	return res.LastInsertId()
}

// UpdateTransaction replaces all fields of a Transaction. Fields its Type does
// not take are stored empty, so a Type change never violates the database CHECK.
func (s *Store) UpdateTransaction(id int64, t NewTransaction) error {
	res, err := s.db.Exec(
		`UPDATE transactions SET date = ?, amount = ?, description = ?, type = ?, category_id = ?, subcategory_id = ?,
		        holding_id = ?, levy_category_id = ?, levy_subcategory_id = ?
		 WHERE id = ?`,
		t.Date, t.Amount, nullString(t.Description), t.Type,
		nullID(t.CategoryID), nullID(t.SubcategoryID), nullID(t.HoldingID),
		nullID(t.LevyCategoryID), nullID(t.LevySubcategoryID), id)
	if err != nil {
		return translate(err)
	}
	return affectedOne(res)
}

// DeleteTransaction removes a Transaction for good.
func (s *Store) DeleteTransaction(id int64) error {
	res, err := s.db.Exec("DELETE FROM transactions WHERE id = ?", id)
	if err != nil {
		return err
	}
	return affectedOne(res)
}

// GetTransaction returns the stored fields of a Transaction, for prefilling
// the edit form.
func (s *Store) GetTransaction(id int64) (NewTransaction, error) {
	var t NewTransaction
	var cat, sub, hold, lcat, lsub sql.NullInt64
	err := s.db.QueryRow(
		`SELECT date, amount, COALESCE(description, ''), type, category_id, subcategory_id, holding_id,
		        levy_category_id, levy_subcategory_id
		 FROM transactions WHERE id = ?`, id).
		Scan(&t.Date, &t.Amount, &t.Description, &t.Type, &cat, &sub, &hold, &lcat, &lsub)
	if err == sql.ErrNoRows {
		return NewTransaction{}, ErrNotFound
	}
	t.CategoryID, t.SubcategoryID, t.HoldingID = cat.Int64, sub.Int64, hold.Int64
	t.LevyCategoryID, t.LevySubcategoryID = lcat.Int64, lsub.Int64
	return t, err
}

// affectedOne reports ErrNotFound if a statement by id changed no row.
func affectedOne(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// PageSize is the number of Transactions on one page of the Transactions page
// (and in the Newest Transactions).
const PageSize = 5

// Filter narrows the Transactions listed and counted; the zero Filter
// narrows nothing.
type Filter struct {
	// From and To bound the date: From <= date < To (YYYY-MM-DD). An empty
	// bound is open.
	From, To string

	// Type keeps only Transactions of that Type; empty is all Types.
	Type string
	// CategoryID and SubcategoryID keep only Transactions with that entry of
	// the Type's Category and Subcategory list (the Levy ones for Levy and
	// Rebate); 0 is no narrowing. Both must match. They are ignored unless the
	// Type takes such lists.
	CategoryID, SubcategoryID int64
	// HoldingID keeps only Transactions of that Holding; 0 is no narrowing.
	// It is ignored unless the Type takes a Holding.
	HoldingID int64

	// Search keeps only Transactions whose description or Holding contains
	// the trimmed text as one literal phrase, ignoring ASCII case; empty or
	// whitespace-only is no narrowing.
	Search string
}

// where is the Filter as an SQL condition on the transactions table aliased
// as alias, with its arguments.
func (f Filter) where(alias string) (string, []any) {
	conds, args := []string{"1"}, []any{}
	if f.From != "" {
		conds, args = append(conds, alias+".date >= ?"), append(args, f.From)
	}
	if f.To != "" {
		conds, args = append(conds, alias+".date < ?"), append(args, f.To)
	}
	if f.Type != "" {
		conds, args = append(conds, alias+".type = ?"), append(args, f.Type)
	}
	if spec, _ := SpecFor(f.Type); spec.Lists != nil {
		if f.CategoryID != 0 {
			conds, args = append(conds, alias+"."+spec.Lists.Category.column+" = ?"), append(args, f.CategoryID)
		}
		if f.SubcategoryID != 0 {
			conds, args = append(conds, alias+"."+spec.Lists.Subcategory.column+" = ?"), append(args, f.SubcategoryID)
		}
	}
	if spec, _ := SpecFor(f.Type); spec.Holding && f.HoldingID != 0 {
		conds, args = append(conds, alias+"."+HoldingList.column+" = ?"), append(args, f.HoldingID)
	}
	// instr and SQLite's lower() are literal and ASCII-only, so %, _ and "ä" vs "Ä" need no special care.
	if text := strings.TrimSpace(f.Search); text != "" {
		conds = append(conds, `(instr(lower(COALESCE(`+alias+`.description, '')), lower(?)) > 0
		 OR instr(lower(COALESCE((SELECT h.name FROM holdings h WHERE h.id = `+alias+`.holding_id), '')), lower(?)) > 0)`)
		args = append(args, text, text)
	}
	return strings.Join(conds, " AND "), args
}

// ListTransactions returns all Transactions, newest first.
func (s *Store) ListTransactions() ([]TransactionRow, error) {
	return s.listTransactions(Filter{}, -1, 0)
}

// ListTransactionsPage returns up to limit of the Transactions matching the
// filter, newest first, skipping the first offset ones. A negative limit
// means no limit.
func (s *Store) ListTransactionsPage(f Filter, limit, offset int) ([]TransactionRow, error) {
	return s.listTransactions(f, limit, offset)
}

// CountTransactions returns the number of Transactions matching the filter.
func (s *Store) CountTransactions(f Filter) (int, error) {
	where, args := f.where("t")
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM transactions t WHERE "+where, args...).Scan(&n)
	return n, err
}

// TransactionPage returns the 1-based page of the Transactions page, narrowed
// by the filter, the Transaction is on: the matching Transactions listed
// before it (a newer date, or the same date and a higher id) divided by
// PageSize. It is ErrNotFound if the Transaction does not match the filter.
func (s *Store) TransactionPage(id int64, f Filter) (int, error) {
	inner, innerArgs := f.where("o")
	outer, outerArgs := f.where("t")
	var before int
	err := s.db.QueryRow(
		`SELECT (SELECT COUNT(*) FROM transactions o
		         WHERE (o.date > t.date OR (o.date = t.date AND o.id > t.id)) AND `+inner+`)
		 FROM transactions t WHERE t.id = ? AND `+outer,
		append(append(innerArgs, id), outerArgs...)...).Scan(&before)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return before/PageSize + 1, err
}

// TransactionYears returns the years that have at least one Transaction, newest first.
func (s *Store) TransactionYears() ([]int, error) {
	rows, err := s.db.Query("SELECT DISTINCT CAST(substr(date, 1, 4) AS INTEGER) AS y FROM transactions ORDER BY y DESC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rows.Err()
}

// listTransactions orders by date, then id, both descending; a negative limit means no limit.
func (s *Store) listTransactions(f Filter, limit, offset int) ([]TransactionRow, error) {
	where, args := f.where("t")
	rows, err := s.db.Query(
		`SELECT t.id, t.date, t.amount, COALESCE(t.description, ''), t.type,
		        COALESCE(c.name, lc.name, ''), COALESCE(sc.name, lsc.name, ''), COALESCE(h.name, '')
		 FROM transactions t
		 LEFT JOIN categories c ON c.id = t.category_id
		 LEFT JOIN subcategories sc ON sc.id = t.subcategory_id
		 LEFT JOIN levy_categories lc ON lc.id = t.levy_category_id
		 LEFT JOIN levy_subcategories lsc ON lsc.id = t.levy_subcategory_id
		 LEFT JOIN holdings h ON h.id = t.holding_id
		 WHERE `+where+`
		 ORDER BY t.date DESC, t.id DESC
		 LIMIT ? OFFSET ?`, append(args, limit, offset)...)
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

// Breakdown returns the summed amounts of Transactions of one Type per entry
// of the list (a Category or Subcategory list), dated from <= date < to,
// largest first.
func (s *Store) Breakdown(typ string, list List, from, to string) ([]Slice, error) {
	rows, err := s.db.Query(
		`SELECT n.name, SUM(t.amount) AS total FROM transactions t JOIN `+list.table+` n ON n.id = t.`+list.column+`
		 WHERE t.type = ? AND t.date >= ? AND t.date < ?
		 GROUP BY n.id ORDER BY total DESC, n.name`, typ, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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

// ListNamed returns all entries of the list ordered by name.
func (s *Store) ListNamed(l List) ([]Named, error) {
	rows, err := s.db.Query("SELECT id, name FROM " + l.table + " ORDER BY name COLLATE NOCASE")
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

// EnsureNamed returns the id of the entry of the list with the given name
// (case-insensitive), creating it if needed.
func (s *Store) EnsureNamed(l List, name string) (int64, error) {
	name = strings.TrimSpace(name)
	var id int64
	err := s.db.QueryRow("SELECT id FROM "+l.table+" WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}
	res, err := s.db.Exec("INSERT INTO "+l.table+" (name) VALUES (?)", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// NamedExists reports whether the list has an entry with the id.
func (s *Store) NamedExists(l List, id int64) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM "+l.table+" WHERE id = ?", id).Scan(&n)
	return n > 0, err
}

// EnsureCategory returns the id of the Category with the given name,
// creating it if needed.
func (s *Store) EnsureCategory(name string) (int64, error) { return s.EnsureNamed(CategoryList, name) }

// EnsureSubcategory returns the id of the Subcategory with the given name,
// creating it if needed.
func (s *Store) EnsureSubcategory(name string) (int64, error) {
	return s.EnsureNamed(SubcategoryList, name)
}

// EnsureLevyCategory returns the id of the Levy Category with the given name,
// creating it if needed.
func (s *Store) EnsureLevyCategory(name string) (int64, error) {
	return s.EnsureNamed(LevyCategoryList, name)
}

// EnsureLevySubcategory returns the id of the Levy Subcategory with the given
// name, creating it if needed.
func (s *Store) EnsureLevySubcategory(name string) (int64, error) {
	return s.EnsureNamed(LevySubcategoryList, name)
}

// EnsureHolding returns the id of the Holding with the given name,
// creating it if needed.
func (s *Store) EnsureHolding(name string) (int64, error) { return s.EnsureNamed(HoldingList, name) }

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

// DescriptionTaken reports whether a Transaction of the Type with the date and
// description (ignoring ASCII case) exists, other than the one with excludeID
// (0 excludes none).
func (s *Store) DescriptionTaken(date, description, typ string, excludeID int64) (bool, error) {
	var n int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM transactions WHERE date = ? AND description = ? COLLATE NOCASE AND type = ? AND id != ?",
		date, description, typ, excludeID).Scan(&n)
	return n > 0, err
}

// NamedUsage is a list entry with the number of Transactions and Repeating
// entries that use it, counted when it is read.
type NamedUsage struct {
	Named
	Transactions, Repeating int
}

// CountNamed returns the number of entries in the list.
func (s *Store) CountNamed(l List) (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM " + l.table).Scan(&n)
	return n, err
}

// ListNamedPage returns up to limit entries of the list from offset, ordered by
// name ignoring case, each with how many Transactions and Repeating entries use it.
func (s *Store) ListNamedPage(l List, limit, offset int) ([]NamedUsage, error) {
	rows, err := s.db.Query(
		`SELECT n.id, n.name,
		        (SELECT COUNT(*) FROM transactions t WHERE t.`+l.column+` = n.id),
		        (SELECT COUNT(*) FROM repeating r WHERE r.`+l.column+` = n.id)
		 FROM `+l.table+` n ORDER BY n.name COLLATE NOCASE LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []NamedUsage
	for rows.Next() {
		var u NamedUsage
		if err := rows.Scan(&u.ID, &u.Name, &u.Transactions, &u.Repeating); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// AddNamed adds the name to the list unless the list has it already, ignoring
// case. It returns the page (of PageSize entries, from 1) the entry sits on in
// the list ordered by name, and whether it was created.
func (s *Store) AddNamed(l List, name string) (page int, created bool, err error) {
	name = strings.TrimSpace(name)
	res, err := s.db.Exec("INSERT INTO "+l.table+" (name) VALUES (?) ON CONFLICT (name) DO NOTHING", name)
	if err != nil {
		return 0, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	var before int
	err = s.db.QueryRow("SELECT COUNT(*) FROM "+l.table+" WHERE name < ? COLLATE NOCASE", name).Scan(&before)
	return before/PageSize + 1, n == 1, err
}
