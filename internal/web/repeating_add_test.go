package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// addForm builds an Add Transactions submission: a list plus the date.
func addForm(date string, rows ...[7]string) url.Values {
	f := repeatingForm(rows...)
	f.Set("date", date)
	f.Set("action", "add")
	return f
}

func (ta *testApp) addTransactions(date string, rows ...[7]string) response {
	ta.t.Helper()
	return ta.post("/repeating", addForm(date, rows...))
}

func TestAddTransactionsDateDefaultsToTheFirstOfTheCurrentMonth(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/repeating").Body, `type="month"`, `value="2026-10"`, "Add Transactions")
}

func TestAddTransactionsBooksOneTransactionPerRowAndStaysOnThePage(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-09-15", repRow("income", "2000", "Salary"), repRow("expense", "800", "Rent"))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, `value="Salary"`, `value="Rent"`, `value="2026-09"`, `data-resting="Add Transactions" data-flash>`)
	if b := addButtonText(t, r.Body); b != "Added 2 Transactions and saved List!" {
		t.Errorf("button %q", b)
	}

	home := ta.get("/transactions?year=all").Body
	wantAll(t, home, "Salary", "Rent", "2026-09-15")
	report := ta.mustReport("/report?period=month&date=2026-09")
	wantAll(t, report, "2.000", "800")

	// The list is unchanged and the success text is gone on the next load.
	again := ta.get("/repeating").Body
	if rowCount(again) != 2 || addButtonText(t, again) != "Add Transactions" {
		t.Errorf("reload:\n%s", again)
	}
}

func TestAddTransactionsFromTheMonthPickerBooksOnTheFirst(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-09", repRow("expense", "8", "Rent"))
	wantAll(t, ta.get("/transactions?year=all").Body, "2026-09-01")
}

func TestAddTransactionsSaysTransactionInTheSingular(t *testing.T) {
	ta := newTestApp(t)
	if b := addButtonText(t, ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent")).Body); b != "Added 1 Transaction and saved List!" {
		t.Errorf("button %q", b)
	}
}

func TestAddTransactionsSavesTheListAndCreatesNewNames(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent"))
	body := ta.get("/repeating").Body
	wantAll(t, body, `value="Rent"`, ">Cat</option>")
}

// wantAdded checks a successful Add Transactions: status 200 and the Add
// Transactions button showing text as a success text.
func wantAdded(t *testing.T, r response, text string) {
	t.Helper()
	if r.Status != http.StatusOK {
		t.Fatalf("status %d, want 200\n%s", r.Status, r.Body)
	}
	if b := addButtonText(t, r.Body); b != text {
		t.Errorf("button %q, want %q", b, text)
	}
	wantAll(t, r.Body, `data-resting="Add Transactions" data-flash>`)
}

func TestAddTransactionsTwiceForTheSameMonthSkipsEverythingAndSavesTheList(t *testing.T) {
	ta := newTestApp(t)
	rows := [][7]string{repRow("expense", "800", "Rent"), repRow("expense", "30", "Gym"), repRow("income", "5", "Salary")}
	ta.addTransactions("2026-10-01", rows...)
	// The second press drops Salary from the list; Rent and Gym exist.
	r := ta.addTransactions("2026-10-01", rows[:2]...)
	wantAdded(t, r, "Transactions already added. Skipping and saving List.")

	home := ta.get("/transactions").Body
	for _, d := range []string{"Salary", "Rent", "Gym"} {
		if n := strings.Count(home, d); n != 1 {
			t.Errorf("%s on home %d times, want 1", d, n)
		}
	}
	list := ta.get("/repeating").Body
	if rowCount(list) != 2 || strings.Contains(list, "Salary") {
		t.Errorf("list not saved as shown:\n%s", list)
	}
}

func TestAddTransactionsSkipsExistingOnesAndBooksTheRest(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "5", "rent", "new", "Cat", "new", "Sub"))
	r := ta.addTransactions("2026-10-01", repRow("income", "2000", "Salary"), repRow("expense", "800", "Rent"), repRow("expense", "30", "Gym"))
	wantAdded(t, r, "Added 2 Transactions, skipped 1 existing one, and saved List!")

	home := ta.get("/transactions").Body
	wantAll(t, home, "Salary", "Gym", ">−5<")
	if strings.Contains(home, ">−800<") {
		t.Error("Rent was booked over the existing rent")
	}
	if rowCount(ta.get("/repeating").Body) != 3 {
		t.Error("list not saved")
	}
}

func TestAddTransactionsCountsSkippedOnesInThePlural(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-10-01", repRow("expense", "800", "Rent"), repRow("expense", "30", "Gym"))
	r := ta.addTransactions("2026-10-01", repRow("expense", "800", "Rent"), repRow("expense", "30", "Gym"), repRow("income", "5", "Salary"))
	wantAdded(t, r, "Added 1 Transaction, skipped 2 existing ones, and saved List!")
}

func TestAddTransactionsWithAnEmptyListSaysSo(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent")))
	r := ta.addTransactions("2026-10-01", [7]string{"expense", "", "", "", "", "", ""})
	wantRejected(t, r, "add", "List is empty", "")
	if rowCount(ta.get("/repeating").Body) != 1 || strings.Contains(ta.get("/repeating").Body, "Rent") {
		t.Error("the empty list was not saved")
	}
}

func TestAddTransactionsRejectsRowsCollidingWithEachOtherBeforeSavingOrBooking(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", repRow("income", "5", "Salary"), repRow("expense", "7", "Rent"), repRow("expense", "8", "rent"))
	wantRejected(t, r, "add", "Duplicate rows", "", 2)
	if body := ta.get("/transactions").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was booked:\n%s", body)
	}
	if rowCount(ta.get("/repeating").Body) != 1 {
		t.Error("list was saved")
	}
}

func TestAddTransactionsWithAnIncompleteListDoesNothing(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", repRow("expense", "9", "Fine"), [7]string{"income", "", "Salary", "new", "C", "new", "S"})
	wantRejected(t, r, "add", "Missing fields", "amount_1")
	wantAll(t, r.Body, `value="Fine"`, `value="2026-10"`)
	if strings.Contains(ta.get("/repeating").Body, "Fine") {
		t.Error("the list was saved")
	}
	if body := ta.get("/transactions").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was created:\n%s", body)
	}
}

func TestAddTransactionsWithAnInvalidDateBooksNothingButSavesTheList(t *testing.T) {
	ta := newTestApp(t)
	for date, button := range map[string]string{
		"":            "Missing fields",
		"01.10.2026":  "Invalid input",
		"2026-02-30":  "Invalid input",
		"0000-12-31":  "Invalid input",
		"10000-01-01": "Invalid input",
	} {
		r := ta.addTransactions(date, repRow("expense", "8", "Rent"))
		inputs := ""
		if date == "" {
			inputs = "date"
		}
		wantRejected(t, r, "add", button, inputs)
		wantAll(t, r.Body, `value="Rent"`)
		if body := ta.get("/transactions").Body; !strings.Contains(body, "No Transactions yet.") {
			t.Errorf("date %q: a Transaction was created", date)
		}
		if rowCount(ta.get("/repeating").Body) != 1 || !strings.Contains(ta.get("/repeating").Body, `value="Rent"`) {
			t.Errorf("date %q: list not saved", date)
		}
	}
}

func TestBookedTransactionsAreUnaffectedByLaterListEdits(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-10-01", repRow("expense", "800", "Rent"))
	mustSave(t, ta, repeatingForm(repRow("expense", "900", "Flat")))
	home := ta.get("/transactions").Body
	wantAll(t, home, "Rent")
	if strings.Contains(home, "Flat") {
		t.Error("an edit reached a booked Transaction")
	}
}

func TestSaveListStillCreatesNothing(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(repRow("expense", "8", "Rent"))
	f.Set("date", "2026-10-01")
	f.Set("action", "save")
	if r := ta.post("/repeating", f); r.Status != http.StatusSeeOther {
		t.Fatalf("status %d", r.Status)
	}
	if body := ta.get("/transactions").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was created:\n%s", body)
	}
}

func TestAddTransactionsBooksLevyAndRebateWithTheirLevyLists(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-09-15",
		[7]string{"levy", "250", "Income tax", "new", "Tax", "new", "Federal"},
		[7]string{"rebate", "40", "Tax refund", "new", "Tax", "new", "Federal"},
		repRow("income", "2000", "Salary"))
	if r.Status != http.StatusOK || addButtonText(t, r.Body) != "Added 3 Transactions and saved List!" {
		t.Fatalf("status %d, button %q", r.Status, addButtonText(t, r.Body))
	}

	home := ta.get("/transactions?year=all").Body
	wantAll(t, home, "Income tax", ">−250<", "Tax refund", ">+40<", "Tax", "Federal")

	// Levy and Rebate stay out of the Report, as for ordinary ones: only the Income is counted.
	report := ta.mustReport("/report?period=month&date=2026-09")
	wantAll(t, report, "2.000")
	if strings.Contains(report, "250") || strings.Contains(report, "Federal") {
		t.Errorf("Levy leaked into the report:\n%s", report)
	}
}

func TestAddTransactionsSkipsAnExistingLevy(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", levyForm("2026-10-01", "5", "income TAX", "new", "Tax", "new", "Federal"))
	r := ta.addTransactions("2026-10-01",
		repRow("income", "5", "Salary"), repRow("levy", "250", "Income tax"), repRow("rebate", "3", "Refund"))
	wantAdded(t, r, "Added 2 Transactions, skipped 1 existing one, and saved List!")
	if body := ta.get("/transactions").Body; strings.Contains(body, ">−250<") {
		t.Errorf("the Levy was booked over the existing one:\n%s", body)
	}
}

func TestRebateIsNotADuplicateOfAnIncomeWithTheSameDescriptionAndDate(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "5", "Refund", "new", "Cat", "new", "Sub"))
	ta.post("/transactions", url.Values{"type": {"income"}, "date": {"2026-10-01"}, "amount": {"40"}, "description": {"Refund"},
		"category": {"new"}, "new_category": {"Cat2"}, "subcategory": {"new"}, "new_subcategory": {"Sub2"}})
	r := ta.addTransactions("2026-10-01", repRow("rebate", "40", "refund"))
	if r.Status != http.StatusOK || addButtonText(t, r.Body) != "Added 1 Transaction and saved List!" {
		t.Fatalf("status %d, button %q", r.Status, addButtonText(t, r.Body))
	}
}

func TestAddTransactionsWithAnIncompleteLevyRowBooksNothing(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", repRow("income", "5", "Salary"), [7]string{"levy", "250", "Tax", "", "", "", ""})
	wantRejected(t, r, "add", "Missing fields", "levy_category_1,levy_subcategory_1")
	if strings.Contains(ta.get("/transactions").Body, "Salary") {
		t.Error("Salary was booked")
	}
}

func TestAddTransactionsBooksInvestmentAndDivestmentAgainstTheirHolding(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-09-15",
		holdRow("investment", "300", "new", "ETF World"), holdRow("divestment", "100", "new", "Bonds"), repRow("income", "2000", "Salary"))
	if r.Status != http.StatusOK || addButtonText(t, r.Body) != "Added 3 Transactions and saved List!" {
		t.Fatalf("status %d, button %q", r.Status, addButtonText(t, r.Body))
	}
	wantAll(t, ta.get("/transactions?year=all").Body, "ETF World", ">+300<", "Bonds", ">−100<", "Investment", "Divestment")
}

func TestAddTransactionsSkipsAHoldingBookedWhateverTheType(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", holdingForm("divestment", "2026-10-01", "40", "new", "ETF"))
	r := ta.addTransactions("2026-10-01", repRow("income", "5", "Salary"), holdRow("investment", "50", "1", ""))
	wantAdded(t, r, "Added 1 Transaction, skipped 1 existing one, and saved List!")
	body := ta.get("/transactions").Body
	wantAll(t, body, "Salary")
	if strings.Contains(body, ">+50<") {
		t.Error("the ETF Investment was booked over the Divestment")
	}
}

func TestAddTransactionsBooksTheHoldingRowsThatDoNotCollide(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", holdingForm("investment", "2026-10-01", "40", "new", "ETF"))
	r := ta.addTransactions("2026-10-01", holdRow("investment", "5", "new", "Gold"), holdRow("divestment", "6", "1", ""))
	wantAdded(t, r, "Added 1 Transaction, skipped 1 existing one, and saved List!")
	body := ta.get("/transactions").Body
	wantAll(t, body, ">+5<")
	if strings.Contains(body, ">−6<") {
		t.Error("the colliding row was booked")
	}
}

func TestAddTransactionsRejectsInvestmentAndDivestmentOfTheSameHolding(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", holdRow("investment", "5", "new", "ETF"), holdRow("divestment", "6", "new", "ETF"))
	wantRejected(t, r, "add", "Duplicate rows", "", 1)
	if strings.Contains(ta.get("/transactions").Body, "ETF") || strings.Contains(ta.get("/repeating").Body, ">ETF</option>") {
		t.Error("something was saved or booked")
	}
}

func TestAddTransactionsTwiceWithAPickedHoldingFindsTheDuplicate(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-10-01", holdRow("investment", "5", "new", "ETF"))
	wantAdded(t, ta.addTransactions("2026-10-01", holdRow("investment", "5", "1", "")), "Transactions already added. Skipping and saving List.")
}

func TestMonthPickerHasAnAccessibleNameButNoVisibleLabel(t *testing.T) {
	ta := newTestApp(t)
	body := ta.get("/repeating").Body
	wantAll(t, body, `aria-label="Month"`)
	if strings.Contains(body, "<label") {
		t.Errorf("visible label on the Repeating page:\n%s", body)
	}
}
