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

-- Transactions reference Category/Subcategory by id, so renames propagate.
-- All five Types are allowed from the start; later tickets add the Holding table and its foreign key
-- (ALTER TABLE ... ADD COLUMN holding_id) without rewriting this table.
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
    description    TEXT    NOT NULL CHECK (length(trim(description)) BETWEEN 1 AND 100),
    type           TEXT    NOT NULL CHECK (type IN ('opening_balance', 'income', 'expense', 'investment', 'divestment')),
    category_id    INTEGER REFERENCES categories (id),
    subcategory_id INTEGER REFERENCES subcategories (id)
);

CREATE INDEX transactions_date ON transactions (date);
CREATE INDEX transactions_category ON transactions (category_id);
CREATE INDEX transactions_subcategory ON transactions (subcategory_id);
