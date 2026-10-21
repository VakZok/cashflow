package web_test

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// carried is the Repeating page as it is left at the address the form's fields make.
func (ta *testApp) carried(f url.Values) response {
	ta.t.Helper()
	return ta.get("/repeating?" + f.Encode())
}

func TestTheRepeatingFormIsCarriedAndItsNavItemRemembersItsPage(t *testing.T) {
	ta := newTestApp(t)
	wantAll(t, ta.get("/repeating").Body, `action="/repeating" data-keep-scroll data-carry>`, `<a href="/repeating" data-page="repeating"`)
}

func TestRowsAndMonthComeBackFromTheAddress(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent"), repRow("expense", "9", "Gym")))
	f := repeatingForm(repRow("expense", "12", "Rent"), repRow("income", "5", "Pay"), [7]string{"expense"})
	f.Set("date", "2027-03")
	body := ta.carried(f).Body
	if n := rowCount(body); n != 3 {
		t.Fatalf("%d rows, want 3", n)
	}
	wantAll(t, body, `value="12"`, `value="Rent"`, `value="Pay"`, `value="2027-03"`, `<option value="income" selected>`)
	if strings.Contains(body, "Gym") {
		t.Error("the saved list is shown")
	}
}

func TestACarriedNewNameKeepsItsInputShown(t *testing.T) {
	ta := newTestApp(t)
	body := ta.carried(repeatingForm(repRow("expense", "8", "Rent"))).Body
	wantAll(t, body, `<option value="new" selected>new…</option>`, `value="Cat"`, `value="Sub"`)
	if strings.Contains(body, `name="new_category_0" maxlength="50" placeholder="New category name" aria-label="New category name" value="Cat" hidden`) {
		t.Error("the new category input is hidden")
	}
}

func TestCarriedLevyAndInvestmentRowsShowTheirOwnFieldGroups(t *testing.T) {
	ta := newTestApp(t)
	body := ta.carried(repeatingForm(repRow("levy", "250", "Income tax"), holdRow("investment", "5", "new", "ETF"))).Body
	rows := strings.Split(body, `class="rep-row"`)
	if len(rows) != 3 {
		t.Fatalf("%d rows", len(rows)-1)
	}
	if !strings.Contains(rows[1], `class="lists-default" hidden`) || strings.Contains(rows[1], `class="lists-levy" hidden`) {
		t.Errorf("Levy row shows the wrong lists:\n%s", rows[1])
	}
	if strings.Contains(rows[2], `class="lists-holding" hidden`) || !strings.Contains(rows[2], `value="ETF"`) {
		t.Errorf("Investment row shows the wrong lists:\n%s", rows[2])
	}
}

func TestACarriedListWithoutRowsShowsTheSavedList(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent")))
	for _, path := range []string{"/repeating", "/repeating?date=2027-03", "/repeating?saved=1"} {
		if body := ta.get(path).Body; !strings.Contains(body, `value="Rent"`) {
			t.Errorf("%s does not show the saved list", path)
		}
	}
}

func TestCarriedChoicesThatNoLongerExistFallBackToEmpty(t *testing.T) {
	ta := newTestApp(t)
	f := repeatingForm(repRow("expense", "8", "Rent"), repRow("levy", "5", "Tax"), holdRow("investment", "5", "9", ""))
	f.Set("category_0", "7")
	f.Set("subcategory_0", "7")
	f.Set("levy_category_1", "7")
	f.Set("levy_subcategory_1", "7")
	body := ta.carried(f).Body
	if strings.Contains(body, `value="7" selected`) || strings.Contains(body, `value="9" selected`) {
		t.Errorf("a missing choice is selected:\n%s", body)
	}
	wantAll(t, body, `value="Rent"`, `value="Tax"`)
}

func TestCarriedChoicesThatStillExistStaySelected(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent")))
	f := repeatingForm([7]string{"expense", "8", "Rent", "1", "", "1", ""})
	wantAll(t, ta.carried(f).Body, `<option value="1" selected>Cat</option>`, `<option value="1" selected>Sub</option>`)
}

func TestAnUnknownCarriedTypeFallsBackToExpense(t *testing.T) {
	ta := newTestApp(t)
	body := ta.carried(repeatingForm([7]string{"bogus", "8", "Rent", "", "", "", ""})).Body
	wantAll(t, body, `<option value="expense" selected>`)
}

func TestFiftyCarriedRowsAreRenderedAndAddIsDisabled(t *testing.T) {
	ta := newTestApp(t)
	var rows [][7]string
	for i := 0; i < 50; i++ {
		rows = append(rows, repRow("expense", "1", "Row "+strconv.Itoa(i)))
	}
	body := ta.carried(repeatingForm(rows...)).Body
	wantAll(t, body, "var nextIndex =  50 ;", "var maxRows =  50 ;")
	if n := rowCount(body); n != 50 {
		t.Fatalf("%d rows, want 50", n)
	}
}

func TestMoreThanFiftyCarriedRowsAreCutOff(t *testing.T) {
	ta := newTestApp(t)
	var rows [][7]string
	for i := 0; i < 55; i++ {
		rows = append(rows, repRow("expense", "1", "Row "+strconv.Itoa(i)))
	}
	if n := rowCount(ta.carried(repeatingForm(rows...)).Body); n != 50 {
		t.Errorf("%d rows, want 50", n)
	}
}

func TestCarriedProblemsAreNotShown(t *testing.T) {
	ta := newTestApp(t)
	body := ta.carried(repeatingForm(repRow("expense", "", "Rent"), repRow("expense", "1", "Rent"))).Body
	if highlightedInputs(body) != "" || len(markedRows(body)) != 0 {
		t.Errorf("problems shown: %q %v", highlightedInputs(body), markedRows(body))
	}
}

func TestSavingCarriedRowsSavesThemAndThePageShowsTheSavedList(t *testing.T) {
	ta := newTestApp(t)
	mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent")))
	body := mustSave(t, ta, repeatingForm(repRow("expense", "8", "Rent"), repRow("income", "5", "Pay"), [7]string{"expense"})).Body
	if n := rowCount(body); n != 2 {
		t.Errorf("%d rows, want the 2 saved ones", n)
	}
}
