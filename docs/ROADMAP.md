# Roadmap

## v1

- Manual entry, a paged Transactions page (newest first), editing and deleting of Transactions (Income, Expense, Levy, Rebate, Investment, Divestment).
- Categories, Subcategories, Levy Categories, Levy Subcategories and Holdings, created inline and added, renamed and deleted on the Edit Categories page.
- Month and year reports: totals per Type, net saved (green if positive, red if negative), pie charts of Income and Expense by Category and by Subcategory (small slices grouped as "Other", full table alongside), invested amount per Holding.

## v2

- Net worth over time, based only on entered data. This needs a starting cash figure (an Opening balance), which v1 deliberately leaves out.
- Line charts per Category and per Subcategory showing month-by-month development.

## Later / optional

- Levy and Rebate report (e.g. the annual tax summary). Levy and Rebate are kept out of the Report for now (ADR 0007).
- Check that a Rebate does not exceed the Levies in its Levy Category. Known limitation: Levy and Rebate amounts are currently independent records.
- Reassign all Transactions and Repeating entries of one Category, Subcategory, Levy Category, Levy Subcategory or Holding to another of the same list in one step, which also serves as merging two entries. For now an entry in use cannot be deleted and its Transactions are edited one by one.
- Tags (filter-only, never summed).
- Budgeting and forecasting.
- Installable home-screen app (PWA).
- Named accounts.
