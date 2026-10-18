package web

import (
	"encoding/json"
	"html/template"
	"strconv"
	"strings"

	"github.com/VakZok/money_manager_app/internal/store"
)

// typeLabel is the display name of a stored Transaction type.
func typeLabel(t string) string {
	if spec, ok := store.SpecFor(t); ok {
		return spec.Label
	}
	return t
}

// typeChoices reads "Income, Expense or Divestment" from the known Types.
func typeChoices() string {
	var labels []string
	for _, s := range store.Types {
		labels = append(labels, s.Label)
	}
	last := len(labels) - 1
	return strings.Join(labels[:last], ", ") + " or " + labels[last]
}

// specsJSON describes which fields each Type takes, for the form script.
func specsJSON() (template.JS, error) {
	type fields struct {
		Description bool `json:"description"`
		Lists       bool `json:"lists"`
		LevyLists   bool `json:"levyLists"`
		Holding     bool `json:"holding"`
	}
	m := map[string]fields{}
	for _, s := range store.Types {
		m[s.Type] = fields{s.Description, s.TakesDefaultLists(), s.TakesLevyLists(), s.Holding}
	}
	b, err := json.Marshal(m)
	return template.JS(b), err
}

// signedEuro renders a magnitude with an explicit Sign (+€50 or −€50, true
// minus U+2212) and the CSS class that colours it. The Report Net and the
// Transaction lists share it.
func signedEuro(amount int64, positive bool) (text, class string) {
	if positive {
		return "+" + formatEuro(amount), "net-positive"
	}
	return "−" + formatEuro(amount), "net-negative"
}

// signedAmount renders a Transaction amount with the Sign of its Type as a
// coloured span.
func signedAmount(typ string, amount int64) template.HTML {
	spec, _ := store.SpecFor(typ)
	text, class := signedEuro(amount, spec.Positive)
	return template.HTML(`<span class="amount ` + class + `">` + template.HTMLEscapeString(text) + `</span>`)
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
