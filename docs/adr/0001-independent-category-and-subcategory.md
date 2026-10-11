# Category and Subcategory are independent, not nested

Category and Subcategory are two flat lists, shared by Income and Expense. A Transaction picks one of each freely. We chose this over a Category → Subcategory hierarchy so that reports by Category and by Subcategory are fully separate views (e.g. all "Restaurant" spending across "Food" and "Social"), at the cost that the form cannot narrow Subcategories after a Category is chosen and nonsensical combinations are possible.

Levy and Rebate Transactions have their own pair of lists, Levy Category and Levy Subcategory, following the same independent model. They are separate tables from Category and Subcategory, and `transactions` references them through their own foreign-key columns, so the database guarantees a Levy never carries a Category. A single shared table with a scope column was rejected because a plain foreign key cannot enforce the scope.
