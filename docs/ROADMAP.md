# Roadmap

## v1

- Manual entry, listing, editing and deleting of Transactions (Income, Expense, Levy, Rebate, Investment, Divestment).
- Categories, Subcategories and Holdings, created inline and managed on a settings page.
- Month and year reports: totals per Type, net saved (green if positive, red if negative), pie charts of Income and Expense by Category and by Subcategory (small slices grouped as "Other", full table alongside), invested amount per Holding.

## v2

- Net worth over time, based only on entered data. This needs a starting cash figure (an Opening balance), which v1 deliberately leaves out.
- Line charts per Category and per Subcategory showing month-by-month development.

## Later / optional

- Levy and Rebate report (e.g. the annual tax summary). Levy and Rebate are kept out of the Report for now (ADR 0007).
- Check that a Rebate does not exceed the Levies in its Levy Category. Known limitation: Levy and Rebate amounts are currently independent records.
- Tags (filter-only, never summed).
- Budgeting and forecasting.
- One-off import script for existing Income and Expense history (`date, description, value, main_category, subcategory, type`).
- Installable home-screen app (PWA).
- Named accounts.
