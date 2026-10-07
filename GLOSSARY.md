# Money Manager

A personal, single-user tracker of money the user has actually moved. It only knows what the user manually enters; there are no live prices or bank feeds.

## Language

**Transaction**:
A single manually entered record of money moving, with a date, a whole-euro amount, and a Type. Income and Expense Transactions also have a description, a Category and a Subcategory; Investment and Divestment Transactions have a Holding instead, and no description, Category or Subcategory.
_Avoid_: Entry, booking, payment

**Type**:
The nature of a Transaction: Opening balance, Income, Expense, Investment, or Divestment.
_Avoid_: Kind

**Opening balance**:
The Type of the single Transaction that states the user's cash on its date, from which all later Transactions are counted. It is not Income and is excluded from Income and Expense reports.
_Avoid_: Start, initial deposit

**Income**:
Money received (salary, gifts, investment profit).
_Avoid_: Earnings, revenue

**Expense**:
Money spent, recorded net of anything paid back (e.g. a dinner of 40 with 20 returned by a friend is an Expense of 20).
_Avoid_: Spending (as a noun for one record), cost

**Investment**:
Money moved from cash into an investment, recorded at the amount put in, never at market value.
_Avoid_: Purchase, deposit

**Divestment**:
Money moved from an investment back into cash, recorded as the principal returned. Profit is booked separately as Income and loss as Expense.
_Avoid_: Disinvestment, withdrawal, sale

**Holding**:
A named investment (e.g. "ETF World") that Investments and Divestments are recorded against. Its invested amount is the sum of its Investments minus its Divestments.
_Avoid_: Asset, position, portfolio item

**Category**:
A grouping of an Income or Expense Transaction (e.g. "Food"). The same Categories are shared by Income and Expense. Investments and Divestments have no Category.
_Avoid_: Label, group

**Subcategory**:
A second, independent grouping of an Income or Expense Transaction (e.g. "Restaurant"). It is not nested under a Category: any Subcategory can be combined with any Category. Each Income or Expense Transaction has exactly one Category and exactly one Subcategory, and each is reported on separately. A Category or Subcategory that still has Transactions cannot be deleted until they are reassigned.
_Avoid_: Child category, tag
