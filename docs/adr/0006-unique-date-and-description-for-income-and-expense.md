# Income and Expense are unique per date and description

Income and Expense Transactions are unique per `(date, description)`, enforced by a unique index on `(date, description COLLATE NOCASE)`. Type, Category and Subcategory are not part of the key, so an Income and an Expense with the same date and description collide too. A second Transaction is rejected as a duplicate, on create and on edit, and the user resolves it by editing the existing Transaction so its amount is the sum of both; there is no auto-merge. Matching ignores ASCII case, like Category, Subcategory and Holding names. Investments and Divestments have no description (NULL) and are exempt, because SQLite treats NULLs as distinct in a unique index.

`id` stays the primary key. The tuple is nullable, descriptions are editable, and nothing needs it as a key. The old date index is dropped because the unique index starts with `date` and serves the same queries.

We deferred, as out of scope: Unicode case folding (`Ärzte` vs `ärzte` stay distinct), case folding for Category, Subcategory and Holding names, collapsing internal whitespace, DB-level enforcement of trimming (the app trims the ends before storing and nothing should bypass it), auto-merge, and a link to the existing entry in the error message.
