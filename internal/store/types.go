package store

// List is a flat named list a Transaction refers to by id (Category,
// Subcategory, Levy Category, Levy Subcategory, Holding). table and column are
// our own constants, never user input.
type List struct {
	Label  string // as shown to the user
	table  string
	column string // the transactions column referring to the list
}

// ListSet is a pair of lists a Type draws its Category and Subcategory from.
type ListSet struct {
	Category    List
	Subcategory List
}

// The lists backing the Transaction fields.
var (
	CategoryList        = List{"Category", "categories", "category_id"}
	SubcategoryList     = List{"Subcategory", "subcategories", "subcategory_id"}
	LevyCategoryList    = List{"Levy Category", "levy_categories", "levy_category_id"}
	LevySubcategoryList = List{"Levy Subcategory", "levy_subcategories", "levy_subcategory_id"}
	HoldingList         = List{"Holding", "holdings", "holding_id"}
)

// DefaultListSet is the list set of Income and Expense.
var DefaultListSet = ListSet{Category: CategoryList, Subcategory: SubcategoryList}

// LevyListSet is the list set of Levy, separate from DefaultListSet.
var LevyListSet = ListSet{Category: LevyCategoryList, Subcategory: LevySubcategoryList}

// TypeSpec describes which fields a Transaction Type takes. It is the one
// place that knows; validation, form rendering and persistence read from it.
type TypeSpec struct {
	Type        string // as stored in the database
	Label       string // as shown to the user
	Description bool
	Lists       *ListSet // the Category and Subcategory pair, nil if the Type takes none
	Holding     bool
}

// Types lists every Type, in the order the entry form offers them.
var Types = []TypeSpec{
	{Type: TypeExpense, Label: "Expense", Description: true, Lists: &DefaultListSet},
	{Type: TypeIncome, Label: "Income", Description: true, Lists: &DefaultListSet},
	{Type: TypeLevy, Label: "Levy", Description: true, Lists: &LevyListSet},
	{Type: TypeInvestment, Label: "Investment", Holding: true},
	{Type: TypeDivestment, Label: "Divestment", Holding: true},
}

// TakesDefaultLists reports whether the Type draws from DefaultListSet.
func (s TypeSpec) TakesDefaultLists() bool { return s.TakesLists(DefaultListSet) }

// TakesLevyLists reports whether the Type draws from LevyListSet.
func (s TypeSpec) TakesLevyLists() bool { return s.TakesLists(LevyListSet) }

// TakesLists reports whether the Type draws from exactly this list set.
func (s TypeSpec) TakesLists(set ListSet) bool { return s.Lists != nil && *s.Lists == set }

// SpecFor returns the TypeSpec of a stored Type.
func SpecFor(typ string) (TypeSpec, bool) {
	for _, s := range Types {
		if s.Type == typ {
			return s, true
		}
	}
	return TypeSpec{}, false
}
