package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// expenseForm builds the entry form submission. category/subcategory are
// either an existing id or "new" together with the new name.
func expenseForm(date, amount, desc, category, newCategory, subcategory, newSubcategory string) url.Values {
	return url.Values{
		"type":            {"expense"},
		"date":            {date},
		"amount":          {amount},
		"description":     {desc},
		"category":        {category},
		"new_category":    {newCategory},
		"subcategory":     {subcategory},
		"new_subcategory": {newSubcategory},
	}
}

func TestAppServesPagesAndSurvivesRestart(t *testing.T) {
	ta := newTestApp(t)
	if r := ta.get("/"); r.Status != 200 {
		t.Fatalf("GET / = %d, want 200", r.Status)
	}
	// Restarting on the same database re-runs migrations without error.
	ta.restart()
	if r := ta.get("/"); r.Status != 200 {
		t.Fatalf("GET / after restart = %d, want 200", r.Status)
	}
}

func TestAddExpenseShowsInList(t *testing.T) {
	ta := newTestApp(t)

	r := ta.post("/transactions", expenseForm("2026-10-01", "1234", "Rent", "new", "Housing", "new", "Monthly"))
	if r.Status != http.StatusSeeOther || r.Location != "/" {
		t.Fatalf("POST = %d -> %q, want 303 -> /", r.Status, r.Location)
	}
	body := ta.follow(r).Body
	for _, want := range []string{"2026-10-01", ">1.234<", "Rent", "Housing", "Monthly"} {
		if !strings.Contains(body, want) {
			t.Errorf("list missing %q in:\n%s", want, body)
		}
	}
}

func TestFormDefaultsDateToToday(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/").Body
	if !strings.Contains(body, `name="date" value="2026-10-05"`) {
		t.Errorf("date input not defaulted to today:\n%s", body)
	}
}

func TestPastAndFutureDatesAreAccepted(t *testing.T) {
	ta := newTestApp(t)
	for _, date := range []string{"2020-01-15", "2031-12-31"} {
		r := ta.post("/transactions", expenseForm(date, "5", "Thing", "new", "Misc", "new", "Other"))
		if r.Status != http.StatusSeeOther {
			t.Fatalf("date %s: status %d, want 303", date, r.Status)
		}
		if body := ta.follow(r).Body; !strings.Contains(body, date) {
			t.Errorf("list missing %s", date)
		}
	}
}

func TestInvalidAmountsAreRejectedWithMessage(t *testing.T) {
	for _, amount := range []string{"0", "-5", "+5", "12.50", "12,50", "1.000", "1e3", "0x10", "５", "abc", "", "100000001", "99999999999999999999"} {
		t.Run("amount="+amount, func(t *testing.T) {
			ta := newTestApp(t)
			r := ta.post("/transactions", expenseForm("2026-10-05", amount, "Lunch", "new", "Food", "new", "Restaurant"))
			if r.Status != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", r.Status)
			}
			if !strings.Contains(r.Body, "Amount must be a whole number of euros between 1 and 100.000.000") {
				t.Errorf("missing amount message in:\n%s", r.Body)
			}
			if strings.Contains(ta.get("/").Body, "Lunch") {
				t.Error("rejected expense was stored")
			}
		})
	}
}

func TestAmountBoundsAreInclusive(t *testing.T) {
	ta := newTestApp(t)
	for _, amount := range []string{"1", "100000000"} {
		r := ta.post("/transactions", expenseForm("2026-10-05", amount, "Big "+amount, "new", "Misc", "new", "Other"))
		if r.Status != http.StatusSeeOther {
			t.Errorf("amount %s: status %d, want 303", amount, r.Status)
		}
	}
}

func TestDateRangeIsInclusive(t *testing.T) {
	ta := newTestApp(t)
	for _, date := range []string{"2000-01-01", "2200-12-31"} {
		r := ta.post("/transactions", expenseForm(date, "5", "Edge", "new", "Misc", "new", "Other"))
		if r.Status != http.StatusSeeOther {
			t.Errorf("date %s: status %d, want 303", date, r.Status)
		}
	}
	for _, date := range []string{"1999-12-31", "2201-01-01", "0001-01-01", "9999-12-31", "2026-13-01", "2026-00-10", "2026-10-00", "2026-10-32", "2026-04-31", "2025-02-29"} {
		r := ta.post("/transactions", expenseForm(date, "5", "Edge", "new", "Misc", "new", "Other"))
		if r.Status != http.StatusUnprocessableEntity {
			t.Errorf("date %s: status %d, want 422", date, r.Status)
		}
	}
}

func TestTextLengthLimits(t *testing.T) {
	ta := newTestApp(t)
	ok := strings.Repeat("ä", 50) // limits count characters, not bytes
	r := ta.post("/transactions", expenseForm("2026-10-05", "5", strings.Repeat("d", 100), "new", ok, "new", ok))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("at-limit values: status %d, want 303", r.Status)
	}
	cases := map[string]url.Values{
		"description": expenseForm("2026-10-05", "5", strings.Repeat("d", 101), "new", "A", "new", "B"),
		"category":    expenseForm("2026-10-05", "5", "x", "new", strings.Repeat("a", 51), "new", "B"),
		"subcategory": expenseForm("2026-10-05", "5", "x", "new", "A", "new", strings.Repeat("b", 51)),
	}
	for name, form := range cases {
		if r := ta.post("/transactions", form); r.Status != http.StatusUnprocessableEntity {
			t.Errorf("%s too long: status %d, want 422", name, r.Status)
		}
	}
}

func TestHostileInputIsStoredAsPlainTextAndEscaped(t *testing.T) {
	ta := newTestApp(t)
	evil := "x'); DROP TABLE transactions;--"
	ta.post("/transactions", expenseForm("2026-10-05", "5", evil, "new", "<script>alert(1)</script>", "new", "B"))
	body := ta.get("/").Body
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("category name rendered unescaped")
	}
	if !strings.Contains(body, "DROP TABLE transactions") {
		t.Error("transactions table or row lost")
	}
}

func TestInvalidSubmissionKeepsEnteredValues(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", expenseForm("2026-09-01", "0", "Lunch", "new", "Food", "new", "Restaurant"))
	for _, want := range []string{`value="2026-09-01"`, `value="Lunch"`, `value="Food"`, `value="Restaurant"`} {
		if !strings.Contains(r.Body, want) {
			t.Errorf("form lost %s", want)
		}
	}
	// Nothing was created as a side effect of the failed submission.
	if body := ta.get("/").Body; strings.Contains(body, ">Food<") || strings.Contains(body, ">Restaurant<") {
		t.Errorf("failed submission created a Category/Subcategory:\n%s", body)
	}
}

func TestInvalidDateAndMissingFieldsAreRejected(t *testing.T) {
	cases := map[string]struct {
		form url.Values
		msg  string
	}{
		"bad date":        {expenseForm("05/10/2026", "5", "x", "new", "A", "new", "B"), "Date must be in the format YYYY-MM-DD"},
		"impossible date": {expenseForm("2026-02-30", "5", "x", "new", "A", "new", "B"), "Date must be in the format YYYY-MM-DD"},
		"no description":  {expenseForm("2026-10-05", "5", "  ", "new", "A", "new", "B"), "Description is required"},
		"no category":     {expenseForm("2026-10-05", "5", "x", "", "", "new", "B"), "Category is required"},
		"empty new cat":   {expenseForm("2026-10-05", "5", "x", "new", " ", "new", "B"), "Category is required"},
		"no subcategory":  {expenseForm("2026-10-05", "5", "x", "new", "A", "", ""), "Subcategory is required"},
		"unknown cat id":  {expenseForm("2026-10-05", "5", "x", "999", "", "new", "B"), "Category is required"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ta := newTestApp(t)
			r := ta.post("/transactions", c.form)
			if r.Status != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", r.Status)
			}
			if !strings.Contains(r.Body, c.msg) {
				t.Errorf("missing %q in:\n%s", c.msg, r.Body)
			}
		})
	}
}

func TestInlineCategoriesAreUsableImmediatelyAndInAnyCombination(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "40", "Dinner", "new", "Food", "new", "Restaurant"))
	ta.post("/transactions", expenseForm("2026-10-02", "30", "Party", "new", "Social", "new", "Cocktails"))

	body := ta.get("/").Body
	// New entries are offered in the form's selects.
	for _, name := range []string{"Food", "Social", "Restaurant", "Cocktails"} {
		if !strings.Contains(body, `>`+name+`</option>`) {
			t.Errorf("form does not offer %q", name)
		}
	}

	// Reuse existing ids in a combination not entered before.
	catID := optionValue(t, body, "category", "Food")
	subID := optionValue(t, body, "subcategory", "Cocktails")
	r := ta.post("/transactions", expenseForm("2026-10-03", "12", "Drinks", catID, "", subID, ""))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("existing ids: status %d, want 303", r.Status)
	}
	if !strings.Contains(ta.follow(r).Body, "Drinks") {
		t.Error("expense with existing Category/Subcategory not listed")
	}
}

func TestNewCategoryNamesReuseExistingCaseInsensitively(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "1", "a", "new", "Food", "new", "Restaurant"))
	ta.post("/transactions", expenseForm("2026-10-01", "1", "b", "new", "food", "new", "RESTAURANT"))
	body := strings.ToLower(ta.get("/").Body)
	if n := strings.Count(body, `>food</option>`); n != 1 {
		t.Errorf("Category offered %d times, want 1", n)
	}
	if n := strings.Count(body, `>restaurant</option>`); n != 1 {
		t.Errorf("Subcategory offered %d times, want 1", n)
	}
}

func TestAmountFormatting(t *testing.T) {
	ta := newTestApp(t)
	amounts := map[string]string{"7": "7", "999": "999", "1000": "1.000", "1234": "1.234", "1234567": "1.234.567"}
	for in, want := range amounts {
		ta.post("/transactions", expenseForm("2026-10-05", in, "amt"+in, "new", "C", "new", "S"))
		if body := ta.get("/").Body; !strings.Contains(body, ">"+want+"<") {
			t.Errorf("amount %s: want %s in its own element in list", in, want)
		}
	}
}

func TestListIsNewestFirst(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-01-01", "1", "entry-older", "new", "C", "new", "S"))
	ta.post("/transactions", expenseForm("2026-03-01", "1", "entry-newer", "new", "C", "new", "S"))
	body := ta.get("/").Body
	if strings.Index(body, "entry-newer") > strings.Index(body, "entry-older") {
		t.Error("list is not ordered newest first")
	}
}

func TestDataPersistsAcrossRestart(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "50", "Persisted", "new", "C", "new", "S"))
	ta.restart()
	if !strings.Contains(ta.get("/").Body, "Persisted") {
		t.Error("Transaction lost after restart")
	}
}

func TestPageIsPhoneFriendly(t *testing.T) {
	ta := newTestApp(t)
	if !strings.Contains(ta.get("/").Body, `<meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover, interactive-widget=resizes-visual">`) {
		t.Error("missing responsive viewport meta tag")
	}
}

// optionValue finds the value of the <option> labelled label inside the
// <select name="selectName"> of an HTML page.
func optionValue(t *testing.T, html, selectName, label string) string {
	t.Helper()
	loc := regexp.MustCompile(`<select[^>]*name="` + regexp.QuoteMeta(selectName) + `"`).FindStringIndex(html)
	if loc == nil {
		t.Fatalf("no select %q", selectName)
	}
	sel := html[loc[0] : loc[0]+strings.Index(html[loc[0]:], "</select>")]
	m := regexp.MustCompile(`<option value="([^"]*)"[^>]*>` + regexp.QuoteMeta(label) + `</option>`).FindStringSubmatch(sel)
	if m == nil {
		t.Fatalf("no option %q in select %q:\n%s", label, selectName, sel)
	}
	return m[1]
}

func incomeForm(date, amount, desc, category, newCategory, subcategory, newSubcategory string) url.Values {
	f := expenseForm(date, amount, desc, category, newCategory, subcategory, newSubcategory)
	f.Set("type", "income")
	return f
}

func TestAddIncomeShowsInListWithType(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", incomeForm("2026-10-02", "2500", "Salary", "new", "Work", "new", "Employer"))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("POST = %d, want 303", r.Status)
	}
	body := ta.follow(r).Body
	for _, want := range []string{"Salary", ">2.500<", "Work", "Employer", "Income"} {
		if !strings.Contains(body, want) {
			t.Errorf("list missing %q in:\n%s", want, body)
		}
	}
}

func TestIncomeSharesCategoriesWithExpense(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "10", "Lunch", "new", "Shared", "new", "Sub"))
	body := ta.get("/").Body
	cat, sub := optionValue(t, body, "category", "Shared"), optionValue(t, body, "subcategory", "Sub")
	r := ta.post("/transactions", incomeForm("2026-10-02", "5", "Refund", cat, "", sub, ""))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("income with existing category = %d", r.Status)
	}
}

func TestIncomeRequiresCategoryAndSubcategory(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", incomeForm("2026-10-02", "5", "Gift", "", "", "", ""))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", r.Status)
	}
	for _, want := range []string{"Category is required.", "Subcategory is required."} {
		if !strings.Contains(r.Body, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestUnknownTypeIsRejected(t *testing.T) {
	ta := newTestApp(t)
	f := expenseForm("2026-10-02", "5", "X", "new", "A", "new", "B")
	f.Set("type", "bogus")
	if r := ta.post("/transactions", f); r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", r.Status)
	}
}

func TestInvalidIncomeKeepsSelectedType(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", incomeForm("2026-10-02", "x", "X", "new", "A", "new", "B"))
	if !regexp.MustCompile(`value="income" selected`).MatchString(r.Body) {
		t.Errorf("income not preselected:\n%s", r.Body)
	}
}

// holdingForm builds an Investment or Divestment submission. holding is an
// existing id or "new" together with the new name.
func holdingForm(typ, date, amount, holding, newHolding string) url.Values {
	return url.Values{
		"type":        {typ},
		"date":        {date},
		"amount":      {amount},
		"holding":     {holding},
		"new_holding": {newHolding},
	}
}

var allTypes = []string{"expense", "income", "investment", "divestment"}

// validForm is a valid submission of the given Type; label ends up in the
// Description (Income, Expense) or the new Holding's name (Investment,
// Divestment).
func validForm(typ, date, amount, label string) url.Values {
	if typ == "investment" || typ == "divestment" {
		return holdingForm(typ, date, amount, "new", label)
	}
	f := expenseForm(date, amount, label, "new", "Misc", "new", "Other")
	f.Set("type", typ)
	return f
}

func TestSharedValidationRunsForAllTypes(t *testing.T) {
	for _, typ := range allTypes {
		t.Run(typ, func(t *testing.T) {
			ta := newTestApp(t)
			for _, amount := range []string{"0", "-5", "12.50", "1e3", "abc", "", "100000001"} {
				r := ta.post("/transactions", validForm(typ, "2026-10-05", amount, "Label"))
				if r.Status != http.StatusUnprocessableEntity || !strings.Contains(r.Body, "Amount must be a whole number of euros") {
					t.Errorf("amount %q: status %d", amount, r.Status)
				}
			}
			for _, amount := range []string{"1", "100000000"} {
				if r := ta.post("/transactions", validForm(typ, "2026-10-05", amount, "Label "+amount)); r.Status != http.StatusSeeOther {
					t.Errorf("amount %s: status %d, want 303", amount, r.Status)
				}
			}
			for _, date := range []string{"1999-12-31", "2201-01-01", "2026-13-01", "2026-02-30", "05/10/2026", ""} {
				if r := ta.post("/transactions", validForm(typ, date, "5", "Label")); r.Status != http.StatusUnprocessableEntity {
					t.Errorf("date %q: status %d, want 422", date, r.Status)
				}
			}
			for _, date := range []string{"2000-01-01", "2200-12-31"} {
				if r := ta.post("/transactions", validForm(typ, date, "5", "Label")); r.Status != http.StatusSeeOther {
					t.Errorf("date %s: status %d, want 303", date, r.Status)
				}
			}
		})
	}
}

func TestInvestmentAndDivestmentShowInListWithHolding(t *testing.T) {
	for typ, label := range map[string]string{"investment": "Investment", "divestment": "Divestment"} {
		t.Run(typ, func(t *testing.T) {
			ta := newTestApp(t)
			r := ta.post("/transactions", holdingForm(typ, "2026-10-03", "1500", "new", "ETF World"))
			if r.Status != http.StatusSeeOther || r.Location != "/" {
				t.Fatalf("POST = %d -> %q, want 303 -> /", r.Status, r.Location)
			}
			body := ta.follow(r).Body
			if !strings.Contains(body, "<span>ETF World</span>") || !strings.Contains(body, ">1.500<") ||
				!strings.Contains(body, "2026-10-03 · "+label+"</span>") {
				t.Errorf("list entry wrong in:\n%s", body)
			}
		})
	}
}

func TestHoldingsAreReusedAndMatchedCaseInsensitively(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", holdingForm("investment", "2026-10-01", "100", "new", "ETF World"))
	ta.post("/transactions", holdingForm("divestment", "2026-10-02", "50", "new", "etf WORLD"))
	body := ta.get("/").Body
	if n := strings.Count(strings.ToLower(body), `>etf world</option>`); n != 1 {
		t.Errorf("Holding offered %d times, want 1", n)
	}
	id := optionValue(t, body, "holding", "ETF World")
	if r := ta.post("/transactions", holdingForm("investment", "2026-10-03", "10", id, "")); r.Status != http.StatusSeeOther {
		t.Fatalf("existing holding id: status %d, want 303", r.Status)
	}
	if n := strings.Count(ta.get("/").Body, "<span>ETF World</span>"); n != 3 {
		t.Errorf("list shows ETF World %d times, want 3 (first spelling kept)", n)
	}
}

func TestHoldingValidation(t *testing.T) {
	cases := map[string]struct {
		form url.Values
		msg  string
	}{
		"missing":       {holdingForm("investment", "2026-10-05", "5", "", ""), "Holding is required."},
		"empty new":     {holdingForm("investment", "2026-10-05", "5", "new", "  "), "Holding is required."},
		"unknown id":    {holdingForm("divestment", "2026-10-05", "5", "999", ""), "Holding is required."},
		"name too long": {holdingForm("investment", "2026-10-05", "5", "new", strings.Repeat("a", 51)), "Holding name must be at most 50 characters."},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			ta := newTestApp(t)
			r := ta.post("/transactions", c.form)
			if r.Status != http.StatusUnprocessableEntity || !strings.Contains(r.Body, c.msg) {
				t.Errorf("status %d, want 422 with %q", r.Status, c.msg)
			}
		})
	}
	ta := newTestApp(t)
	if r := ta.post("/transactions", holdingForm("investment", "2026-10-05", "5", "new", strings.Repeat("ä", 50))); r.Status != http.StatusSeeOther {
		t.Errorf("50-char holding: status %d, want 303", r.Status)
	}
}

func TestRejectedSubmissionCreatesNoHolding(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", holdingForm("investment", "2026-10-05", "0", "new", "Ghost Fund"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	if !strings.Contains(r.Body, `value="Ghost Fund"`) {
		t.Error("rejected form lost the entered Holding name")
	}
	if strings.Contains(ta.get("/").Body, ">Ghost Fund</option>") {
		t.Error("rejected submission created a Holding")
	}
}

func TestInvestmentRejectsCategoryDescriptionAndSubcategory(t *testing.T) {
	for _, typ := range []string{"investment", "divestment"} {
		for field, value := range map[string]string{
			"description": "Oops", "category": "new", "new_category": "Food",
			"subcategory": "new", "new_subcategory": "Restaurant",
		} {
			t.Run(typ+"/"+field, func(t *testing.T) {
				ta := newTestApp(t)
				f := holdingForm(typ, "2026-10-05", "5", "new", "Fund")
				f.Set(field, value)
				if r := ta.post("/transactions", f); r.Status != http.StatusUnprocessableEntity {
					t.Errorf("status %d, want 422", r.Status)
				}
				if strings.Contains(ta.get("/").Body, ">Fund</option>") {
					t.Error("rejected submission created a Holding")
				}
			})
		}
	}
}

func TestIncomeAndExpenseRejectHolding(t *testing.T) {
	for _, typ := range []string{"income", "expense"} {
		for field, value := range map[string]string{"holding": "new", "new_holding": "Fund"} {
			t.Run(typ+"/"+field, func(t *testing.T) {
				ta := newTestApp(t)
				f := validForm(typ, "2026-10-05", "5", "Thing")
				f.Set(field, value)
				if r := ta.post("/transactions", f); r.Status != http.StatusUnprocessableEntity {
					t.Errorf("status %d, want 422", r.Status)
				}
				if body := ta.get("/").Body; strings.Contains(body, ">Fund</option>") || strings.Contains(body, ">Misc</option>") {
					t.Error("rejected submission created a Holding, Category or Subcategory")
				}
			})
		}
		t.Run(typ+"/still requires fields", func(t *testing.T) {
			ta := newTestApp(t)
			f := validForm(typ, "2026-10-05", "5", " ")
			f.Set("category", "")
			f.Set("new_category", "")
			r := ta.post("/transactions", f)
			if r.Status != http.StatusUnprocessableEntity {
				t.Fatalf("status %d, want 422", r.Status)
			}
			for _, want := range []string{"Description is required.", "Category is required."} {
				if !strings.Contains(r.Body, want) {
					t.Errorf("missing %q", want)
				}
			}
		})
	}
}

func TestFormOffersAllFourTypes(t *testing.T) {
	body := newTestApp(t).get("/").Body
	for _, v := range allTypes {
		if !strings.Contains(body, `<option value="`+v+`"`) {
			t.Errorf("Type %s not offered", v)
		}
	}

}

// The form groups are shown or hidden by Type on the server side (and by a
// script on change); hidden inputs are disabled so they are not submitted.
func TestFormDisablesHiddenFieldGroupsByType(t *testing.T) {
	ta := newTestApp(t)
	for typ, wantDisabled := range map[string][]string{
		"expense":    {"holding", "new_holding"},
		"income":     {"holding", "new_holding"},
		"investment": {"description", "category", "new_category", "subcategory", "new_subcategory"},
		"divestment": {"description", "category", "new_category", "subcategory", "new_subcategory"},
	} {
		r := ta.post("/transactions", holdingForm(typ, "2026-10-05", "0", "", ""))
		for _, field := range []string{"description", "category", "new_category", "subcategory", "new_subcategory", "holding", "new_holding"} {
			tag := regexp.MustCompile(`<(?:input|select)[^>]*name="` + field + `"[^>]*>`).FindString(r.Body)
			if tag == "" {
				t.Fatalf("%s: no input %q", typ, field)
			}
			want := false
			for _, d := range wantDisabled {
				want = want || d == field
			}
			if got := regexp.MustCompile(`\sdisabled[\s>]`).MatchString(tag); got != want {
				t.Errorf("%s: %s disabled = %v, want %v", typ, field, got, want)
			}
		}
	}
}

func TestInvalidInvestmentKeepsEnteredValues(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", holdingForm("divestment", "2026-09-01", "0", "new", "Gold"))
	for _, want := range []string{`value="divestment" selected`, `value="2026-09-01"`, `value="Gold"`} {
		if !strings.Contains(r.Body, want) {
			t.Errorf("form lost %s", want)
		}
	}
}

func TestDuplicateDateAndDescriptionIsRejectedWithMessage(t *testing.T) {
	ta := newTestApp(t)
	if r := ta.post("/transactions", expenseForm("2026-10-01", "4", "Coffee", "new", "Food", "new", "Cafes")); r.Status != http.StatusSeeOther {
		t.Fatalf("first POST = %d, want 303", r.Status)
	}

	// Same date, description differing only in ASCII case, other Category: still
	// a duplicate, and the typed values are kept.
	dup := expenseForm("2026-10-01", "9", "coffee", "new", "Other", "new", "Misc")
	r := ta.post("/transactions", dup)
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate POST = %d, want 422", r.Status)
	}
	if !strings.Contains(r.Body, "An entry for this date and description already exists, edit it instead") {
		t.Errorf("missing duplicate message in:\n%s", r.Body)
	}
	if !strings.Contains(r.Body, `value="coffee"`) {
		t.Errorf("entered description not kept in:\n%s", r.Body)
	}
	if strings.Contains(ta.get("/").Body, "Misc") {
		t.Error("rejected submission created a Subcategory")
	}

	// An Income with the same date and description is accepted.
	if r := ta.post("/transactions", incomeForm("2026-10-01", "9", "Coffee", "new", "Other", "new", "Misc")); r.Status != http.StatusSeeOther {
		t.Errorf("income POST = %d, want 303", r.Status)
	}

	// A different date is accepted.
	if r := ta.post("/transactions", expenseForm("2026-10-02", "4", "Coffee", "new", "Food", "new", "Cafes")); r.Status != http.StatusSeeOther {
		t.Errorf("different date POST = %d, want 303", r.Status)
	}
}

func TestDuplicateDateAndHoldingIsRejectedWithMessage(t *testing.T) {
	ta := newTestApp(t)
	if r := ta.post("/transactions", holdingForm("investment", "2026-10-01", "50", "new", "ETF")); r.Status != http.StatusSeeOther {
		t.Fatalf("first POST = %d, want 303", r.Status)
	}

	// Same date and Holding (name matched ignoring case) collide, whichever the
	// Type; the typed values are kept.
	for _, typ := range []string{"investment", "divestment"} {
		r := ta.post("/transactions", holdingForm(typ, "2026-10-01", "70", "new", "etf"))
		if r.Status != http.StatusUnprocessableEntity {
			t.Fatalf("%s POST = %d, want 422", typ, r.Status)
		}
		if !strings.Contains(r.Body, "An entry for this date and holding already exists, edit it instead") {
			t.Errorf("%s: missing duplicate message in:\n%s", typ, r.Body)
		}
		if !strings.Contains(r.Body, `value="70"`) {
			t.Errorf("%s: entered amount not kept in:\n%s", typ, r.Body)
		}
	}

	// A different date or a different Holding is accepted.
	for _, form := range []url.Values{
		holdingForm("investment", "2026-10-02", "50", "new", "ETF"),
		holdingForm("investment", "2026-10-01", "50", "new", "Gold"),
	} {
		if r := ta.post("/transactions", form); r.Status != http.StatusSeeOther {
			t.Errorf("POST %v = %d, want 303", form, r.Status)
		}
	}
}

func TestIncomeAndExpenseAreNotHoldingDuplicates(t *testing.T) {
	ta := newTestApp(t)
	if r := ta.post("/transactions", holdingForm("investment", "2026-10-01", "50", "new", "ETF")); r.Status != http.StatusSeeOther {
		t.Fatalf("investment POST = %d, want 303", r.Status)
	}
	for i, form := range []url.Values{
		expenseForm("2026-10-01", "4", "Coffee", "new", "Food", "new", "Cafes"),
		incomeForm("2026-10-01", "9", "Tip", "new", "Other", "new", "Misc"),
	} {
		if r := ta.post("/transactions", form); r.Status != http.StatusSeeOther {
			t.Errorf("entry #%d POST = %d, want 303", i, r.Status)
		}
	}
}
