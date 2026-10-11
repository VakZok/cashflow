package web

import (
	"encoding/json"
	"html/template"
	"strconv"

	"github.com/VakZok/money_manager_app/internal/store"
)

// typeLabel is the display name of a stored Transaction type.
func typeLabel(t string) string {
	if spec, ok := store.SpecFor(t); ok {
		return spec.Label
	}
	return t
}

// specsJSON describes which fields each Type takes, for the form script.
func specsJSON() (template.JS, error) {
	type fields struct {
		Description bool `json:"description"`
		Lists       bool `json:"lists"`
		Holding     bool `json:"holding"`
	}
	m := map[string]fields{}
	for _, s := range store.Types {
		m[s.Type] = fields{s.Description, s.Lists != nil, s.Holding}
	}
	b, err := json.Marshal(m)
	return template.JS(b), err
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
