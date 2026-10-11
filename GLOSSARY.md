# Money Manager

A personal, single-user tracker of money the user has actually moved. It only knows what the user manually enters; there are no live prices or bank feeds.

## Language

**Transaction**:
A single manually entered record of money moving, with a date, a whole-euro amount, and a Type. Income and Expense Transactions also have a description, a Category and a Subcategory; Levy and Rebate Transactions have a description, a Levy Category and a Levy Subcategory; Investment and Divestment Transactions have a Holding instead, and no description, Category or Subcategory.
_Avoid_: Entry, booking, payment

**Type**:
The nature of a Transaction: Income, Expense, Levy, Rebate, Investment, or Divestment.
_Avoid_: Kind

**Income**:
Money actually received to spend or invest, after any Levy was taken off (salary as paid out, gifts, investment profit).
_Avoid_: Earnings, revenue

**Expense**:
Money spent, recorded net of anything paid back (e.g. a dinner of 40 with 20 returned by a friend is an Expense of 20).
_Avoid_: Spending (as a noun for one record), cost

**Levy**:
A mandatory charge taken off the user's earnings automatically, before the money arrives (e.g. income tax, solidarity surcharge). Recorded for information only: it is not part of Net and does not appear in the Report's charts, because the user cannot influence it and it would otherwise dominate the view of what they can. Income is therefore typed in net of Levies, and the Levy is typed in separately.
_Avoid_: Tax, duty, deduction, Abgabe

**Rebate**:
Money returned against an earlier Levy (e.g. a tax refund), recorded for information only. The money actually received is typed separately as Income, so a refund is one Rebate and one Income of the same amount.
_Avoid_: Refund, credit, reimbursement

**Investment**:
Money moved from cash into an investment, recorded at the amount put in, never at market value.
_Avoid_: Purchase, deposit

**Divestment**:
Money moved from an investment back into cash, recorded as the principal returned. Profit is booked separately as Income and loss as Expense.
_Avoid_: Disinvestment, withdrawal, sale

**Holding**:
A named investment (e.g. "ETF World") that Investments and Divestments are recorded against. Its invested amount is the sum of its Investments minus its Divestments.
_Avoid_: Asset, position, portfolio item

**Period**:
A calendar month or a calendar year that a Report covers.
_Avoid_: Range, timeframe

**Report**:
The page showing figures for one Period, covering Income and Expense.
_Avoid_: Dashboard, summary

**Net**:
Income minus Expense for a Period. Positive, negative, or exactly zero.
_Avoid_: Net saved, balance, profit

**Category**:
A grouping of an Income or Expense Transaction (e.g. "Food"). The same Categories are shared by Income and Expense. Levies and Rebates use Levy Categories instead; Investments and Divestments have no Category.
_Avoid_: Label, group

**Subcategory**:
A second, independent grouping of an Income or Expense Transaction (e.g. "Restaurant"). It is not nested under a Category: any Subcategory can be combined with any Category. Each Income or Expense Transaction has exactly one Category and exactly one Subcategory, and each is reported on separately. A Category or Subcategory that still has Transactions cannot be deleted until they are reassigned.
_Avoid_: Child category, tag

**Levy Category**:
A grouping of a Levy or Rebate Transaction (e.g. "Income tax"). A separate list from Category: a Levy Category cannot be used on Income or Expense, and a Category cannot be used on a Levy or Rebate.
_Avoid_: Tax category

**Levy Subcategory**:
A second, independent grouping of a Levy or Rebate Transaction, related to Levy Category exactly as Subcategory is related to Category. A separate list from Subcategory.
_Avoid_: Tax subcategory

**Duplicate**:
A Transaction that collides with an existing one, in one of two independent ways. An Income, Expense, Levy or Rebate with the same date, description (ignoring ASCII case) and Type as an existing Transaction, whatever its Category or Subcategory; the same date and description under different Types is allowed (e.g. a Rebate and the Income that carries its money). An Investment or Divestment with the same date and Holding as an existing Investment or Divestment, whatever its Type. It is rejected on create and on edit; the user merges by hand by editing the existing Transaction so its amount is the sum of both. Investments and Divestments have no description and the other four Types have no Holding, so the two rules never apply to each other's Transactions.
_Avoid_: Conflict, clash
