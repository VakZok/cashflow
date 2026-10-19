package web_test

import "testing"

func TestListAmountsCarryTheSignOfTheirType(t *testing.T) {
	cases := []struct {
		typ, want string
	}{
		{"income", "+1.234"},
		{"rebate", "+1.234"},
		{"investment", "+1.234"},
		{"expense", "−1.234"},
		{"levy", "−1.234"},
		{"divestment", "−1.234"},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			ta := newTestApp(t)
			ta.seedType(c.typ, "1234")
			wantAll(t, ta.get("/transactions").Body, `<span class="amount">`+c.want+`</span>`)
		})
	}
}

// seedType records a Transaction of any Type with the form that Type takes.
func (ta *testApp) seedType(typ, amount string) {
	ta.t.Helper()
	f := validForm(typ, "2026-10-05", amount, "x")
	switch typ {
	case "levy":
		f = levyForm("2026-10-05", amount, "x", "new", "Tax", "new", "Income")
	case "rebate":
		f = rebateForm("2026-10-05", amount, "x", "new", "Tax", "new", "Income")
	}
	if r := ta.post("/transactions", f); r.Status != 303 {
		ta.t.Fatalf("seed %s = %d\n%s", typ, r.Status, r.Body)
	}
}
