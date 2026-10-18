package web_test

import (
	"net/http"
	"net/url"
	"testing"
)

func rebateForm(date, amount, desc, levyCategory, newLevyCategory, levySubcategory, newLevySubcategory string) url.Values {
	f := levyForm(date, amount, desc, levyCategory, newLevyCategory, levySubcategory, newLevySubcategory)
	f.Set("type", "rebate")
	return f
}

func TestAddRebateWithInlineLevyCategoriesShowsInList(t *testing.T) {
	ta := newTestApp(t)
	r := ta.post("/transactions", rebateForm("2026-10-01", "250", "Tax refund", "new", "Tax", "new", "Federal"))
	if r.Status != http.StatusSeeOther {
		t.Fatalf("status %d, want 303\n%s", r.Status, r.Body)
	}
	wantAll(t, ta.follow(r).Body, "Tax refund", ">250<", "Rebate", "Tax", "Federal")
}

func TestRebateUsesExistingLevyCategories(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", levyForm("2026-10-01", "100", "Income tax", "new", "Tax", "new", "Federal"))
	body := ta.get("/").Body
	wantAll(t, body, `name="levy_category"`, ">Tax</option>", ">Federal</option>")
	if r := ta.post("/transactions", rebateForm("2026-10-02", "5", "Refund", "1", "", "1", "")); r.Status != http.StatusSeeOther {
		t.Fatalf("status %d, want 303\n%s", r.Status, r.Body)
	}
}

func TestRebateDuplicates(t *testing.T) {
	ta := newTestApp(t)
	if r := ta.post("/transactions", rebateForm("2026-10-01", "5", "Refund", "new", "T", "new", "S")); r.Status != http.StatusSeeOther {
		t.Fatalf("rebate: %d", r.Status)
	}
	if r := ta.post("/transactions", validForm("income", "2026-10-01", "5", "Refund")); r.Status != http.StatusSeeOther {
		t.Errorf("income with the same date and description: %d, want 303\n%s", r.Status, r.Body)
	}
	if r := ta.post("/transactions", levyForm("2026-10-01", "5", "refund", "new", "T", "new", "S")); r.Status != http.StatusSeeOther {
		t.Errorf("levy with the same date and description: %d, want 303\n%s", r.Status, r.Body)
	}
	r := ta.post("/transactions", rebateForm("2026-10-01", "9", "REFUND", "new", "T", "new", "S"))
	if r.Status != http.StatusUnprocessableEntity {
		t.Errorf("second rebate: %d, want 422", r.Status)
	}
}

func TestRebateLargerThanAnyLevyIsAccepted(t *testing.T) {
	ta := newTestApp(t)
	ta.post("/transactions", levyForm("2026-10-01", "100", "Income tax", "new", "Tax", "new", "Federal"))
	if r := ta.post("/transactions", rebateForm("2026-10-02", "5000", "Refund", "1", "", "1", "")); r.Status != http.StatusSeeOther {
		t.Fatalf("status %d, want 303\n%s", r.Status, r.Body)
	}
}

func TestRebateDoesNotChangeReport(t *testing.T) {
	ta := newTestApp(t)
	ta.seedIn("expense", "2026-10-02", "300", "Food", "Restaurant")
	ta.seedIn("income", "2026-10-03", "900", "Salary", "Work")
	const path = "/report?period=month&date=2026-10"
	before := ta.mustReport(path)
	beforeExpense, beforeExpenseAmts := ta.chart(path)

	if r := ta.post("/transactions", rebateForm("2026-10-04", "400", "Refund", "new", "Tax", "new", "Federal")); r.Status != http.StatusSeeOther {
		t.Fatalf("rebate POST = %d\n%s", r.Status, r.Body)
	}
	if after := ta.mustReport(path); after != before {
		t.Errorf("report changed after a Rebate:\n%s", after)
	}
	for _, p := range []string{path, path + "&type=rebate&group=category"} {
		l, a := ta.chart(p)
		wantChart(t, l, a, beforeExpense, beforeExpenseAmts)
	}
}
