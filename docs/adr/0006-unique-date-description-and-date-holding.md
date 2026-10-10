# Transactions are unique per date and description, or per date and holding

Two independent rules keep Transactions from being entered twice. A second entry is rejected as a duplicate, on create and on edit, and the user resolves it by editing the existing entry so its amount is the sum of both; there is no auto-merge. Both rules use the same duplicate error path, with wording that names what collided.

**Income and Expense** are unique per `(date, description)`, enforced by a unique index on `(date, description COLLATE NOCASE)`. Type, Category and Subcategory are not part of the key, so an Income and an Expense with the same date and description collide too. Matching ignores ASCII case, like Category, Subcategory and Holding names. Investments and Divestments have no description (NULL) and are exempt, because SQLite treats NULLs as distinct in a unique index.

**Investment and Divestment** are unique per `(date, holding_id)`, enforced by a unique index on `(date, holding_id)`. The Type is not part of the key, so an Investment and a Divestment of the same Holding on the same date collide too. Income and Expense have no Holding (NULL) and are exempt for the same reason. The web layer does not pre-check this rule: it resolves the Holding by name, and the failed insert is translated to the duplicate error.

`id` stays the primary key. The tuples are nullable, descriptions are editable, and nothing needs them as a key. The old date index is dropped because the description index starts with `date` and serves the same queries.

We deferred, as out of scope: Unicode case folding (`Ärzte` vs `ärzte` stay distinct), case folding for Category, Subcategory and Holding names beyond what they already have, collapsing internal whitespace, DB-level enforcement of trimming (the app trims the ends before storing and nothing should bypass it), auto-merge, and a link to the existing entry in the error message.
