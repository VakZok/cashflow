package store

import "errors"

// MaxRepeating is the most rows the Repeating list holds.
const MaxRepeating = 50

// ErrTooManyRepeating means a list longer than MaxRepeating was submitted.
var ErrTooManyRepeating = errors.New("too many repeating rows")

// RepeatingEntry is one row of the Repeating list: a Transaction without a date.
type RepeatingEntry struct {
	Type          string
	Amount        int64 // whole euros, > 0
	Description   string
	CategoryID    int64
	SubcategoryID int64
	HoldingID     int64

	LevyCategoryID    int64
	LevySubcategoryID int64
}

// ListRepeating returns the Repeating list grouped by Type (Income, Expense,
// Investment, Divestment, Levy, Rebate) and, within a Type, in saved order.
func (s *Store) ListRepeating() ([]RepeatingEntry, error) {
	rows, err := s.db.Query(
		`SELECT type, amount, COALESCE(description, ''), COALESCE(category_id, 0), COALESCE(subcategory_id, 0),
		        COALESCE(holding_id, 0), COALESCE(levy_category_id, 0), COALESCE(levy_subcategory_id, 0)
		 FROM repeating
		 ORDER BY CASE type WHEN 'income' THEN 0 WHEN 'expense' THEN 1 WHEN 'investment' THEN 2
		                    WHEN 'divestment' THEN 3 WHEN 'levy' THEN 4 ELSE 5 END, position`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RepeatingEntry
	for rows.Next() {
		var e RepeatingEntry
		if err := rows.Scan(&e.Type, &e.Amount, &e.Description, &e.CategoryID, &e.SubcategoryID,
			&e.HoldingID, &e.LevyCategoryID, &e.LevySubcategoryID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ReplaceRepeating replaces the stored list with entries, in one transaction:
// on error the old list is kept. The slice order is the saved order.
func (s *Store) ReplaceRepeating(entries []RepeatingEntry) error {
	if len(entries) > MaxRepeating {
		return ErrTooManyRepeating
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec("DELETE FROM repeating"); err != nil {
		return err
	}
	for i, e := range entries {
		if _, err := tx.Exec(
			`INSERT INTO repeating (position, amount, description, type, category_id, subcategory_id, holding_id,
			                        levy_category_id, levy_subcategory_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			i, e.Amount, nullString(e.Description), e.Type,
			nullID(e.CategoryID), nullID(e.SubcategoryID), nullID(e.HoldingID),
			nullID(e.LevyCategoryID), nullID(e.LevySubcategoryID)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
