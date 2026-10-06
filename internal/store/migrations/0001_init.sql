-- Category and Subcategory are independent flat lists (ADR 0001).
CREATE TABLE categories (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) > 0)
);

CREATE TABLE subcategories (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE CHECK (length(trim(name)) > 0)
);

-- Transactions reference Category/Subcategory by id, so renames propagate.
-- All five Types are allowed from the start; later tickets add the Holding
-- table and its foreign key (ALTER TABLE ... ADD COLUMN holding_id) without
-- rewriting this table.
CREATE TABLE transactions (
    id             INTEGER PRIMARY KEY,
    date           TEXT    NOT NULL CHECK (date GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]'),
    amount         INTEGER NOT NULL CHECK (amount > 0),
    description    TEXT    NOT NULL,
    type           TEXT    NOT NULL CHECK (type IN ('opening_balance', 'income', 'expense', 'investment', 'divestment')),
    category_id    INTEGER REFERENCES categories (id),
    subcategory_id INTEGER REFERENCES subcategories (id)
);

CREATE INDEX transactions_date ON transactions (date);
CREATE INDEX transactions_category ON transactions (category_id);
CREATE INDEX transactions_subcategory ON transactions (subcategory_id);
