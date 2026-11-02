// spec/rules/dashes.md §3.4a (spec 1.18.0) — dash.parenthetical = "em-paired", exercised on es.
package polytypo_test

import (
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
)

func TestEmPairedDashes(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"spaced hyphens", "Llegó - por fin - a casa", "Llegó —por fin— a casa"},
		{"tight hyphens", "Llegó-por fin-a casa", "Llegó-por fin-a casa"},
		{"already paired", "Llegó —por fin— a casa", "Llegó —por fin— a casa"},
		{"lone stroke", "Madrid - la capital", "Madrid - la capital"},
		{"three strokes", "a - b - c - d", "a - b - c - d"},
		{"dialogue dash", "—Hola —dijo—.", "—Hola —dijo—."},
		{"terminal splits segment", "Fue - sí. Y - no", "Fue - sí. Y - no"},
		{"opener shaped second stroke", "Llegó - por fin -.", "Llegó - por fin -."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := polytypo.Transform(c.in, polytypo.Options{Locale: "es"})
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("Transform(%q) = %q, want %q", c.in, got, c.want)
			}
			again, err := polytypo.Transform(got, polytypo.Options{Locale: "es"})
			if err != nil || again != got {
				t.Errorf("not idempotent: %q -> %q (%v)", got, again, err)
			}
		})
	}
}
