// spec/rules/nbsp.md 3.7 (spec 1.22.0) -- N5: spacedUnits insert, closedUnits delete, beforeUnits
// convert only.
package polytypo_test

import (
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
)

const unitNBSP = " "

func unitsText(t *testing.T, input, locale string) string {
	t.Helper()
	out, err := polytypo.Transform(input, polytypo.Options{Locale: locale, Mode: "text"})
	if err != nil {
		t.Fatalf("Transform(%q, %s): %v", input, locale, err)
	}
	return out
}

func TestNbspUnits(t *testing.T) {
	cases := []struct{ name, locale, in, want string }{
		{"spacedUnits inserts", "de-DE", "Es sind 20km", "Es sind 20" + unitNBSP + "km"},
		{"closedUnits deletes", "en-US", "Rose 5 %", "Rose 5%"},
		{"closedUnits deletes nbsp", "ru", "Rose 5" + unitNBSP + "%", "Rose 5%"},
		{"beforeUnits converts only", "cs", "14 % nebo 20%", "14" + unitNBSP + "% nebo 20%"},
		{"file name", "de-DE", "foto-5kg.png", "foto-5kg.png"},
		{"compound", "de-DE", "15cm-Norm", "15cm-Norm"},
		{"path", "de-DE", "/5km", "/5km"},
		{"identifier", "de-DE", "x_5kg", "x_5kg"},
		{"letter before number", "de-DE", "H2km", "H2km"},
		{"modulo", "en-US", "10 % 3", "10 % 3"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := unitsText(t, c.in, c.locale); got != c.want {
				t.Errorf("Transform(%q, %s) = %q, want %q", c.in, c.locale, got, c.want)
			}
		})
	}
}

// Sized by document count, not run time: CI runs this under -race, which is ~350x slower.
func TestNbspUnitsIdempotent(t *testing.T) {
	atoms := []string{"5", " ", unitNBSP, "%", "km", "°C", "-", ".", "a", "_", "–", "×"}
	for _, locale := range []string{"de-DE", "en-US", "cs"} {
		var walk func(prefix string, depth int)
		walk = func(prefix string, depth int) {
			if prefix != "" {
				once := unitsText(t, prefix, locale)
				if twice := unitsText(t, once, locale); twice != once {
					t.Fatalf("%s: %q -> %q -> %q", locale, prefix, once, twice)
				}
			}
			if depth == 0 {
				return
			}
			for _, a := range atoms {
				walk(prefix+a, depth-1)
			}
		}
		walk("", 3)
	}
}

func TestNbspUnitsMultiplicationChainIdempotent(t *testing.T) {
	atoms := []string{"2", "3", " ", unitNBSP, "\u202f", "x", "\u0445", "kg", "\u043a\u043c", "%"}
	for _, locale := range []string{"de-DE", "en-US", "ru"} {
		var walk func(prefix string, depth int)
		walk = func(prefix string, depth int) {
			if prefix != "" {
				once := unitsText(t, prefix, locale)
				if twice := unitsText(t, once, locale); twice != once {
					t.Fatalf("%s: %q -> %q -> %q", locale, prefix, once, twice)
				}
			}
			if depth == 0 {
				return
			}
			for _, a := range atoms {
				walk(prefix+a, depth-1)
			}
		}
		walk("", 4)
	}
}
