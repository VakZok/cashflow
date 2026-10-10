-- Category and Subcategory are independent flat lists (ADR 0001).
-- Names are 1..50 characters after trimming.
CREATE TABLE categories (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) BETWEEN 1 AND 50)
);

CREATE TABLE subcategories (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) BETWEEN 1 AND 50)
);

-- A Holding is a named investment that Investments and Divestments are recorded against.
CREATE TABLE holdings (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) BETWEEN 1 AND 50)
);

-- Transactions reference Category/Subcategory/Holding by id, so renames propagate.
-- All four Types are allowed from the start.
--
-- Per-Type fields (the CHECK on the last line of the table):
--   income, expense:         description, category and subcategory; no holding
--   investment, divestment:  holding only; no description, category or subcategory

--
-- date:   strict YYYY-MM-DD, a real calendar day, 2000..2200. date() returns
--         NULL (month 13) or a normalised date (2026-02-30) for impossible
--         days; COALESCE matters because a NULL CHECK result counts as a pass.
-- amount: whole euros, 1..100.000.000. typeof() rejects text such as '12,5',
--         which SQLite would otherwise store as-is in an INTEGER column.
CREATE TABLE transactions (
    id             INTEGER PRIMARY KEY,
    date           TEXT    NOT NULL CHECK (
        date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'
        AND COALESCE(date(date), '') = date
        AND date BETWEEN '2000-01-01' AND '2200-12-31'),
    amount         INTEGER NOT NULL CHECK (typeof(amount) = 'integer' AND amount BETWEEN 1 AND 100000000),
    description    TEXT    CHECK (description IS NULL OR length(trim(description)) BETWEEN 1 AND 100),
    type           TEXT    NOT NULL CHECK (type IN ('income', 'expense', 'investment', 'divestment')),
    category_id    INTEGER REFERENCES categories (id),
    subcategory_id INTEGER REFERENCES subcategories (id),
    holding_id     INTEGER REFERENCES holdings (id),
    CHECK (
        (type IN ('income', 'expense')
            AND description IS NOT NULL AND category_id IS NOT NULL AND subcategory_id IS NOT NULL AND holding_id IS NULL)
        OR (type IN ('investment', 'divestment')
            AND description IS NULL AND category_id IS NULL AND subcategory_id IS NULL AND holding_id IS NOT NULL))
);

-- Income and Expense are unique per (date, description), ignoring ASCII case (ADR 0006).
-- Investments and Divestments have a NULL description, which SQLite treats as distinct.
-- Leading with date, it also serves date-range queries.
CREATE UNIQUE INDEX transactions_date_description ON transactions (date, description COLLATE NOCASE);
CREATE INDEX transactions_category ON transactions (category_id);
CREATE INDEX transactions_subcategory ON transactions (subcategory_id);
CREATE INDEX transactions_holding ON transactions (holding_id);
