package web

import "strconv"

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
