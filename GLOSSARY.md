# Money Manager

A personal, single-user tracker of money the user has actually moved. It only knows what the user manually enters; there are no live prices or bank feeds.

## Language

**Transaction**:
A single manually entered record of money moving, with a date, a whole-euro amount, and a Type. Income and Expense Transactions also have a description, a Category and a Subcategory; Levy and Rebate Transactions have a description, a Levy Category and a Levy Subcategory; Investment and Divestment Transactions have a Holding instead, and no description, Category or Subcategory.
_Avoid_: Entry, booking, payment

**Repeating**:
A saved Transaction without a date, kept in a user-maintained list so a Transaction that comes back often does not have to be retyped. It has a Type, an amount, and the same other fields as a Transaction of that Type, each set per entry. The list changes only when the user edits it, and only complete entries can be saved. Adding the list books one Transaction per entry on a single date chosen at that moment; the Transactions are independent copies, so later editing or removing an entry never changes Transactions already booked. A Repeating entry is not a Transaction and appears in no Report.
_Avoid_: Recurring, template, standing order

**Type**:
The nature of a Transaction: Income, Expense, Levy, Rebate, Investment, or Divestment. Each Type has a Sign.
_Avoid_: Kind

**Sign**:
Whether a Type adds to what the user keeps (positive: Income, Rebate, Investment) or takes away from it (negative: Expense, Levy, Divestment). Investment is positive because invested money is still the user's; Divestment is negative because it takes money back out.
_Avoid_: Cashflow, incoming/outgoing, direction

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
A calendar month or a calendar year that a Report covers, or that the Transactions page is narrowed to. A month always belongs to a specific year.
_Avoid_: Range, timeframe

**Report**:
The page showing figures for one Period, covering Income and Expense. Its pie chart groups items with a small share of the total as Other when there are at least two; the table below the pie chart lists every item.
_Avoid_: Dashboard, summary

**Transactions page**:
The page listing all Transactions from the newest date to the oldest, a few at a time, with arrows to move to the newer or older ones. Transactions can be edited and deleted from it. It can be narrowed by a Period, a Type, the Type's Category-like groupings or Holding, and a Search; the narrowing applies together, shows all matching Transactions across the pages, and survives editing and deleting. By default nothing narrows it. A single action clears all narrowing at once.
_Avoid_: History, log, journal

**Type filter**:
Narrows the Transactions page to one Type. Once a Type is chosen, its groupings can be narrowed too: Category and Subcategory for Income and Expense, Levy Category and Levy Subcategory for Levy and Rebate, Holding for Investment and Divestment. Each is a single choice from all existing values, and when two are set both must match. Changing the Type keeps the groupings when the new Type uses the same lists, and drops them otherwise.
_Avoid_: Tab, view

**Search**:
Narrows the Transactions page to those whose description or Holding contains the typed text, ignoring ASCII case. The text is matched literally, as one phrase. It is available for all Types except Investment and Divestment, where the Holding filter replaces it; the typed text survives a Type change that keeps it available.
_Avoid_: Query, find

**Newest Transactions**:
The few Transactions with the most recent dates, shown on the add page so the user sees what is already entered. Newest means the latest Transaction date, not the latest time of entry: a Transaction entered today with an old date is not among them. Transactions on the same date are ordered by when they were entered, last first.
_Avoid_: Latest, recent, last added

**Net**:
Income minus Expense for a Period. Positive, negative, or exactly zero.
_Avoid_: Net saved, balance, profit

**Category**:
A grouping of an Income or Expense Transaction (e.g. "Food"). The same Categories are shared by Income and Expense. Levies and Rebates use Levy Categories instead; Investments and Divestments have no Category.
_Avoid_: Label, group

**Subcategory**:
A second, independent grouping of an Income or Expense Transaction (e.g. "Restaurant"). It is not nested under a Category: any Subcategory can be combined with any Category. Each Income or Expense Transaction has exactly one Category and exactly one Subcategory, and each is reported on separately. A Category or Subcategory that still has Transactions or Repeating entries cannot be deleted until they are reassigned.
_Avoid_: Child category, tag

**Levy Category**:
A grouping of a Levy or Rebate Transaction (e.g. "Income tax"). A separate list from Category: a Levy Category cannot be used on Income or Expense, and a Category cannot be used on a Levy or Rebate.
_Avoid_: Tax category

**Levy Subcategory**:
A second, independent grouping of a Levy or Rebate Transaction, related to Levy Category exactly as Subcategory is related to Category. A separate list from Subcategory.
_Avoid_: Tax subcategory

**Duplicate**:
A Transaction that collides with an existing one, in one of two independent ways. An Income, Expense, Levy or Rebate with the same date, description (ignoring ASCII case) and Type as an existing Transaction, whatever its Category or Subcategory; the same date and description under different Types is allowed (e.g. a Rebate and the Income that carries its money). An Investment or Divestment with the same date and Holding as an existing Investment or Divestment, whatever its Type. It is rejected on create and on edit; the user merges by hand by editing the existing Transaction so its amount is the sum of both. Investments and Divestments have no description and the other four Types have no Holding, so the two rules never apply to each other's Transactions. The same rules, minus the date, apply between two entries of a Repeating list, since they would collide when booked on one date; such a list is rejected on save.
_Avoid_: Conflict, clash
