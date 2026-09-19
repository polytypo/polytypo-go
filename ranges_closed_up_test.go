// spec/rules/ranges.md §3.2a (spec 1.3.0) — a symbol repeated closed up on both members, and
// the dashes.md §3.2 step 8 amendment it forced.
package polytypo_test

import (
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
)

const (
	wj = "\u2060"
	en = "–"
	em = "—"
)

func ranges(t *testing.T, input, locale string) string {
	t.Helper()
	out, err := polytypo.Transform(input, polytypo.Options{Locale: locale, Rules: map[string]bool{"ranges": true}})
	if err != nil {
		t.Fatalf("Transform(%q, %s): %v", input, locale, err)
	}
	return out
}

func TestClosedUpSymbolRanges(t *testing.T) {
	cases := []struct{ name, locale, in, want string }{
		{"repeated prefix", "en-US", "$15-$20", "$15" + wj + en + wj + "$20"},
		{"repeated prefix, euro", "de-DE", "€15-€20", "€15" + wj + en + wj + "€20"},
		{"repeated suffix", "en-US", "35%-50%", "35%" + wj + en + wj + "50%"},
		{"repeated suffix, em locale", "ru", "35%-50%", "35%" + wj + em + wj + "50%"},
		{"degree sign", "en-US", "15°-20°", "15°" + wj + en + wj + "20°"},
		{"mismatched symbols decline", "en-US", "$15-€20", "$15-€20"},
		{"half-written declines", "en-US", "15-$20", "15-$20"},
		{"half-written suffix declines", "en-US", "15%-20", "15%-20"},
		{"elided prefix still converts", "en-US", "$15-20", "$15" + wj + en + wj + "20"},
		{"elided suffix still converts", "en-US", "15-20%", "15" + wj + en + wj + "20%"},
		{"multi-code-point prefix declines", "en-US", "US$15-US$20", "US$15-US$20"},
		{"multi-code-point suffix declines", "en-US", "15°C-20°C", "15°C-20°C"},
		{"both flanks decided together", "en-US", "%15%-%20%", "%15%" + wj + en + wj + "%20%"},
		{"spaced token is now a range", "en-US", "$15 - $20", "$15" + wj + en + wj + "$20"},
		{"hyphen between an existing joiner pair", "en-US", "$15" + wj + "-" + wj + "$20", "$15" + wj + en + wj + "$20"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ranges(t, tc.in, tc.locale); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// CLOSED-SYMBOL is the U+20A0-U+20CF block by its bounds plus a fixed list, never an Sc category
// test: a port enumerating only the symbols it saw in tests fails the first two, and a port using
// a category test fails the last two.
func TestClosedUpSymbolSetBoundary(t *testing.T) {
	cases := []struct{ in, want string }{
		{"₹15-₹20", "₹15" + wj + en + wj + "₹20"},
		{"₴100-₴200", "₴100" + wj + en + wj + "₴200"},
		{"֏15-֏20", "֏15-֏20"},
		{"￥15-￥20", "￥15-￥20"},
	}
	for _, tc := range cases {
		if got := ranges(t, tc.in, "en-US"); got != tc.want {
			t.Fatalf("%q: got %q, want %q", tc.in, got, tc.want)
		}
	}
}

// dashes.md §3.2 step 8: widening a range member widens what a `dashes` edit elsewhere can
// disturb. Each witness drifted on the second pass before T1's reach became
// CLOSED-SYMBOL-transparent, and each is inert in its all-digit shape.
func TestClosedUpSymbolT1(t *testing.T) {
	witnesses := []string{"a—$15-$20", "35%-50%—b", "a--15% - 20%", "$1 - $1--a"}
	for _, locale := range []string{"de-DE", "ru", "en-GB", "fi"} {
		for _, input := range witnesses {
			once := ranges(t, input, locale)
			if twice := ranges(t, once, locale); twice != once {
				t.Fatalf("%s %q: not a fixed point: %q then %q", locale, input, once, twice)
			}
		}
	}

	// Both transparency positions on one side at once — eleven tokens, out of reach of the
	// canonical exhaustive sweep.
	for _, input := range []string{"a--$15% - $20%", "$15% - $20%--a", "a--$15% - $20%--b"} {
		once := ranges(t, input, "de-DE")
		if twice := ranges(t, once, "de-DE"); twice != once {
			t.Fatalf("%q: not a fixed point: %q then %q", input, once, twice)
		}
	}
}

func TestClosedUpSymbolT1Boundary(t *testing.T) {
	// T1 applies only when the chosen form is spaced, which is why the repair is T1 and not the
	// unconditional cluster guard: that one would have taken en-US with it.
	defaults := []struct{ locale, in, want string }{
		{"en-US", "price--$50--drop", "price—$50—drop"},
		{"de-DE", "Anstieg--50%--war", "Anstieg--50%--war"},
		{"de-DE", "Anstieg--50--war", "Anstieg--50--war"},
		{"en-US", "$15 - $20", "$15 - $20"},
		{"en-US", "$15 - €20", "$15—€20"},
	}
	for _, tc := range defaults {
		got, err := polytypo.Transform(tc.in, polytypo.Options{Locale: tc.locale})
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("%s %q: got %q, want %q", tc.locale, tc.in, got, tc.want)
		}
	}

	// dashes.md §3.2 step 8 disagreed with §3.2b in its own text through spec 1.2.0; no
	// implementation ever did. The space ends the cluster, so step 7 does not cover this.
	for _, input := range []string{"a--15" + wj + " - 20", "a--$15" + wj + "-" + wj + "$20"} {
		if got := ranges(t, input, "de-DE"); got != input {
			t.Fatalf("%q: got %q, want it unchanged", input, got)
		}
	}
}
