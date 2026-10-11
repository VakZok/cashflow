// Package store owns the SQLite database: opening it, applying versioned
// migrations, and the queries the web layer needs.
package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"modernc.org/sqlite" // pure-Go driver, no CGO
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is a handle on the application database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and applies any
// pending migrations.
func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := migrate(db, migrationFS); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// migrate applies migrations/NNNN_name.sql files in order. The applied
// version is kept in PRAGMA user_version; each migration runs in its own
// transaction.
func migrate(db *sql.DB, fsys fs.FS) error {
	files, err := fs.Glob(fsys, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)

	var current int
	if err := db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	for _, f := range files {
		name := f[strings.LastIndex(f, "/")+1:]
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("bad migration name %q", name)
		}
		if version <= current {
			continue
		}
		body, err := fs.ReadFile(fsys, f)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// sqliteConstraintUnique is SQLITE_CONSTRAINT_UNIQUE, the extended result code
// of a violated UNIQUE constraint or index.
const sqliteConstraintUnique = 2067

// ErrDuplicate means a Transaction with the same date, description and Type
// (Income, Expense and Levy, ignoring ASCII case) or the same date and Holding (Investment
// and Divestment) already exists.
var ErrDuplicate = errors.New("duplicate transaction")

// translate maps a unique-constraint violation to ErrDuplicate.
func translate(err error) error {
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqliteConstraintUnique {
		return ErrDuplicate
	}
	return err
}
