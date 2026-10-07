package web

import (
	"strconv"

	"github.com/VakZok/money_manager_app/internal/store"
)

// typeLabel is the display name of a stored Transaction type.
func typeLabel(t string) string {
	switch t {
	case store.TypeIncome:
		return "Income"
	case store.TypeExpense:
		return "Expense"
	case store.TypeInvestment:
		return "Investment"
	case store.TypeDivestment:
		return "Divestment"
	}
	return t
}

// formatEuro renders whole euros as €1.234 (dot as thousands separator).
func formatEuro(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	digits := strconv.FormatInt(amount, 10)
	var out []byte
	for i := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, digits[i])
	}
	s := "€" + string(out)
	if neg {
		s = "-" + s
	}
	return s
}
