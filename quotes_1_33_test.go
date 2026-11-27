// spec/rules/quotes.md 3.1, 3.2 and 3.5 (spec 1.33.0) -- inverted-mark OPENISH, the quoted-lone-mark
// veto and the bounded certification gate.
package polytypo_test

import (
	"strings"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
)

func TestQuotes133(t *testing.T) {
	cases := []struct{ name, locale, in, want string }{
		{"opens after inverted question mark", "es", `¿"Sí"?`, "¿«Sí»?"},
		{"opens after inverted exclamation mark", "es", `¡"Hola"!`, "¡«Hola»!"},
		{"inverted mark in English prose", "en-US", `¡"Hola"!`, "¡“Hola”!"},
		{"lone mark in single quotes is left", "en-US", `Press '"' for a quote and 'x' for exit.`, `Press '"' for a quote and “x” for exit.`},
		{"lone mark does not flip the pair before it", "en-US", `'he' '"'`, `“he” '"'`},
		{"lone single quote in double quotes", "en-US", `"'"`, `"'"`},
		{"lone mark in en-GB", "en-GB", `'he' '"'`, `‘he’ '"'`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := polytypo.Transform(c.in, polytypo.Options{Locale: c.locale, Mode: "text", Rules: map[string]bool{"ranges": false}})
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("Transform(%q, %s) = %q, want %q", c.in, c.locale, got, c.want)
			}
			again, err := polytypo.Transform(got, polytypo.Options{Locale: c.locale, Mode: "text", Rules: map[string]bool{"ranges": false}})
			if err != nil || again != got {
				t.Errorf("not idempotent: %q -> %q (%v)", got, again, err)
			}
		})
	}
}

// The gate runs at most eight rounds; the long input must finish and stay a fixed point.
func TestQuotesGateRoundBound(t *testing.T) {
	in := strings.Repeat(`'"a"`, 400)
	got, err := polytypo.Transform(in, polytypo.Options{Locale: "en-US", Mode: "text", Rules: map[string]bool{"ranges": false}})
	if err != nil {
		t.Fatal(err)
	}
	again, err := polytypo.Transform(got, polytypo.Options{Locale: "en-US", Mode: "text", Rules: map[string]bool{"ranges": false}})
	if err != nil || again != got {
		t.Fatalf("not idempotent on a long alternating run")
	}
}
