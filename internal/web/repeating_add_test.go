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
	wantAll(t, ta.get("/repeating").Body, `type="date"`, `value="2026-10-01"`, "Add Transactions")
}

func TestAddTransactionsBooksOneTransactionPerRowAndStaysOnThePage(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-09-15", repRow("income", "2000", "Salary"), repRow("expense", "800", "Rent"))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "2 Transactions added for 2026-09-15.", `value="Salary"`, `value="Rent"`, `value="2026-09-15"`)

	home := ta.get("/").Body
	wantAll(t, home, "Salary", "Rent", "2026-09-15")
	report := ta.mustReport("/report?period=month&date=2026-09")
	wantAll(t, report, "2.000", "800")

	// The list is unchanged and the notice is gone on the next load.
	again := ta.get("/repeating").Body
	if rowCount(again) != 2 || strings.Contains(again, "added for") {
		t.Errorf("reload:\n%s", again)
	}
}

func TestAddTransactionsSaysTransactionInTheSingular(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent")).Body, "1 Transaction added for 2026-10-01.")
}

func TestAddTransactionsNoticeComesAfterTheButtons(t *testing.T) {
	ta := newTestApp(t)
	body := ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent")).Body
	b, n := strings.Index(body, "Add Transactions"), strings.Index(body, "Transaction added")
	if b < 0 || n < b {
		t.Errorf("button at %d, notice at %d", b, n)
	}
}

func TestAddTransactionsSavesTheListAndCreatesNewNames(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent"))
	body := ta.get("/repeating").Body
	wantAll(t, body, `value="Rent"`, ">Cat</option>")
}

func TestAddTransactionsTwiceForTheSameMonthReportsDuplicatesAndKeepsTheList(t *testing.T) {
	ta := newTestApp(t)
	rows := [][7]string{repRow("expense", "800", "Rent"), repRow("expense", "30", "Gym"), repRow("income", "5", "Salary")}
	ta.addTransactions("2026-10-01", rows...)
	// The second press drops Salary from the list; Rent and Gym collide.
	r := ta.addTransactions("2026-10-01", rows[:2]...)
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "Already exists on 2026-10-01: Rent, Gym")

	// Salary is not booked a second time, and the saved list is the one shown.
	if n := strings.Count(ta.get("/").Body, "Salary"); n != 1 {
		t.Errorf("Salary on home %d times, want 1", n)
	}
	list := ta.get("/repeating").Body
	if rowCount(list) != 2 || strings.Contains(list, "Salary") {
		t.Errorf("list not saved as shown:\n%s", list)
	}
}

func TestDuplicateListsEveryCollidingRowAndBooksNothing(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "5", "rent", "new", "Cat", "new", "Sub"))
	r := ta.addTransactions("2026-10-01", repRow("income", "5", "Salary"), repRow("expense", "800", "Rent"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", r.Status)
	}
	wantAll(t, r.Body, "Already exists on 2026-10-01: Rent", `value="Salary"`)
	if strings.Contains(ta.get("/").Body, "Salary") {
		t.Error("Salary was booked despite the Duplicate")
	}
}

func TestDuplicateErrorComesAfterTheAddTransactionsButton(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent"))
	body := ta.addTransactions("2026-10-01", repRow("expense", "8", "Rent")).Body
	b, e := strings.Index(body, "Add Transactions"), strings.Index(body, "Already exists")
	if b < 0 || e < b {
		t.Errorf("button at %d, error at %d", b, e)
	}
}

func TestAddTransactionsRejectsRowsCollidingWithEachOtherBeforeSavingOrBooking(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", repRow("income", "5", "Salary"), repRow("expense", "7", "Rent"), repRow("expense", "8", "rent"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "Duplicate rows in the list: Rent")
	if body := ta.get("/").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was booked:\n%s", body)
	}
	if rowCount(ta.get("/repeating").Body) != 1 {
		t.Error("list was saved")
	}
}

func TestAddTransactionsWithAnIncompleteListDoesNothing(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", repRow("expense", "9", "Fine"), [7]string{"income", "", "Salary", "new", "C", "new", "S"})
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", r.Status)
	}
	wantAll(t, r.Body, errIncomplete, `value="Fine"`, `value="2026-10-01"`)
	b, e := strings.Index(r.Body, "Add Transactions"), strings.Index(r.Body, errIncomplete)
	if b < 0 || e < b {
		t.Errorf("button at %d, error at %d", b, e)
	}
	if strings.Contains(ta.get("/repeating").Body, "Fine") {
		t.Error("the list was saved")
	}
	if body := ta.get("/").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was created:\n%s", body)
	}
}

func TestAddTransactionsWithAnInvalidDateBooksNothingButSavesTheList(t *testing.T) {
	ta := newTestApp(t)
	for date, want := range map[string]string{
		"":           "Date must be in the format YYYY-MM-DD.",
		"01.10.2026": "Date must be in the format YYYY-MM-DD.",
		"2026-02-30": "Date must be in the format YYYY-MM-DD.",
		"1999-12-31": "Date must be between 2000 and 2200.",
		"2201-01-01": "Date must be between 2000 and 2200.",
	} {
		r := ta.addTransactions(date, repRow("expense", "8", "Rent"))
		if r.Status != http.StatusUnprocessableEntity {
			t.Fatalf("date %q: status %d", date, r.Status)
		}
		wantAll(t, r.Body, want, `value="Rent"`)
		if body := ta.get("/").Body; !strings.Contains(body, "No Transactions yet.") {
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
	home := ta.get("/").Body
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
	if body := ta.get("/").Body; !strings.Contains(body, "No Transactions yet.") {
		t.Errorf("a Transaction was created:\n%s", body)
	}
}

func TestAddTransactionsBooksLevyAndRebateWithTheirLevyLists(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-09-15",
		[7]string{"levy", "250", "Income tax", "new", "Tax", "new", "Federal"},
		[7]string{"rebate", "40", "Tax refund", "new", "Tax", "new", "Federal"},
		repRow("income", "2000", "Salary"))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "3 Transactions added for 2026-09-15.")

	home := ta.get("/").Body
	wantAll(t, home, "Income tax", "€250", "Tax refund", "€40", "Tax", "Federal")

	// Levy and Rebate stay out of the Report, as for ordinary ones: only the Income is counted.
	report := ta.mustReport("/report?period=month&date=2026-09")
	wantAll(t, report, "2.000")
	if strings.Contains(report, "250") || strings.Contains(report, "Federal") {
		t.Errorf("Levy leaked into the report:\n%s", report)
	}
}

func TestAddTransactionsReportsLevyDuplicatesAndBooksNothing(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", levyForm("2026-10-01", "5", "income TAX", "new", "Tax", "new", "Federal"))
	r := ta.addTransactions("2026-10-01",
		repRow("income", "5", "Salary"), repRow("levy", "250", "Income tax"), repRow("rebate", "3", "Refund"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "Already exists on 2026-10-01: Income tax")
	if body := ta.get("/").Body; strings.Contains(body, "Salary") || strings.Contains(body, "Refund") {
		t.Errorf("rows were booked despite the Duplicate:\n%s", body)
	}
}

func TestRebateIsNotADuplicateOfAnIncomeWithTheSameDescriptionAndDate(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", expenseForm("2026-10-01", "5", "Refund", "new", "Cat", "new", "Sub"))
	ta.post("/transactions", url.Values{"type": {"income"}, "date": {"2026-10-01"}, "amount": {"40"}, "description": {"Refund"},
		"category": {"new"}, "new_category": {"Cat2"}, "subcategory": {"new"}, "new_subcategory": {"Sub2"}})
	r := ta.addTransactions("2026-10-01", repRow("rebate", "40", "refund"))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d, want 200\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "1 Transaction added for 2026-10-01.")
}

func TestAddTransactionsWithAnIncompleteLevyRowBooksNothing(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", repRow("income", "5", "Salary"), [7]string{"levy", "250", "Tax", "", "", "", ""})
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	wantAll(t, r.Body, errIncomplete)
	if strings.Contains(ta.get("/").Body, "Salary") {
		t.Error("Salary was booked")
	}
}

func TestAddTransactionsBooksInvestmentAndDivestmentAgainstTheirHolding(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-09-15",
		holdRow("investment", "300", "new", "ETF World"), holdRow("divestment", "100", "new", "Bonds"), repRow("income", "2000", "Salary"))
	if r.Status != http.StatusOK {
		t.Fatalf("status %d\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "3 Transactions added for 2026-09-15.")
	wantAll(t, ta.get("/").Body, "ETF World", "€300", "Bonds", "€100", "Investment", "Divestment")
}

func TestAddTransactionsReportsHoldingDuplicatesByHoldingNameWhateverTheType(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", holdingForm("divestment", "2026-10-01", "40", "new", "ETF"))
	r := ta.addTransactions("2026-10-01", repRow("income", "5", "Salary"), holdRow("investment", "50", "1", ""))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422\n%s", r.Status, r.Body)
	}
	wantAll(t, r.Body, "Already exists on 2026-10-01: ETF")
	if strings.Contains(ta.get("/").Body, "Salary") {
		t.Error("Salary was booked despite the Duplicate")
	}
}

func TestAddTransactionsBooksNothingWhenOnlySomeHoldingRowsCollide(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", holdingForm("investment", "2026-10-01", "40", "new", "ETF"))
	r := ta.addTransactions("2026-10-01", holdRow("investment", "5", "new", "Gold"), holdRow("divestment", "6", "1", ""))
	wantAll(t, r.Body, "Already exists on 2026-10-01: ETF")
	if body := ta.get("/").Body; strings.Contains(body, "€5") || strings.Contains(body, "€6") {
		t.Error("the Gold or the colliding row was booked")
	}
}

func TestAddTransactionsRejectsInvestmentAndDivestmentOfTheSameHolding(t *testing.T) {
	ta := newTestApp(t)
	r := ta.addTransactions("2026-10-01", holdRow("investment", "5", "new", "ETF"), holdRow("divestment", "6", "new", "ETF"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, want 422", r.Status)
	}
	wantAll(t, r.Body, "Duplicate rows in the list: ETF")
	if strings.Contains(ta.get("/").Body, "ETF") || strings.Contains(ta.get("/repeating").Body, ">ETF</option>") {
		t.Error("something was saved or booked")
	}
}

func TestAddTransactionsTwiceReportsTheHoldingNameAgain(t *testing.T) {
	ta := newTestApp(t)
	ta.addTransactions("2026-10-01", holdRow("investment", "5", "new", "ETF"))
	r := ta.addTransactions("2026-10-01", holdRow("investment", "5", "1", ""))
	wantAll(t, r.Body, "Already exists on 2026-10-01: ETF")
}
