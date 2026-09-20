// spec/rules/nbsp.md 3.1a — NarrowNbsp moves NARROW-TARGET, it does not post-process.
package polytypo_test

import (
	"strings"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
)

const (
	nbsp  = " "
	nnbsp = " "
)

func fr(t *testing.T, input string, narrowNbsp string) string {
	t.Helper()
	out, err := polytypo.Transform(input, polytypo.Options{Locale: "fr", NarrowNbsp: narrowNbsp})
	if err != nil {
		t.Fatalf("Transform(%q, NarrowNbsp=%q): %v", input, narrowNbsp, err)
	}
	return out
}

func TestNarrowNbspWhatItChanges(t *testing.T) {
	input := "Un délai ? Vraiment ! Et puis ; voilà."
	if got, want := fr(t, input, ""), "Un délai"+nnbsp+"? Vraiment"+nnbsp+"! Et puis"+nnbsp+"; voilà."; got != want {
		t.Fatalf("default: got %q, want %q", got, want)
	}
	if got, want := fr(t, input, "nbsp"), "Un délai"+nbsp+"? Vraiment"+nbsp+"! Et puis"+nbsp+"; voilà."; got != want {
		t.Fatalf("substituted: got %q, want %q", got, want)
	}

	// An authored narrow space at a claimed index is normalised to the target; elsewhere it is
	// left alone, because the option changes what is written and not which indices are claimed.
	if got := fr(t, "Oui"+nnbsp+"?", "nbsp"); got != "Oui"+nbsp+"?" {
		t.Fatalf("authored at a claimed index: got %q", got)
	}
	if got := fr(t, "mot"+nnbsp+"mot", "nbsp"); got != "mot"+nnbsp+"mot" {
		t.Fatalf("authored elsewhere: got %q", got)
	}
}

func TestNarrowNbspWhatItDoesNotChange(t *testing.T) {
	// The right-context guard still protects a time and a URL.
	if got := fr(t, "12:30 et http://x ; oui", "nbsp"); got != "12:30 et http://x"+nbsp+"; oui" {
		t.Fatalf("guards: got %q", got)
	}
	// N1 (the colon) and N8 (fr's primary pair, innerSpace "nbsp") already wrote U+00A0.
	want := "Il a dit" + nbsp + ": «" + nbsp + "oui" + nbsp + "»" + nbsp + "; puis" + nbsp + "?"
	if got := fr(t, "Il a dit : « oui » ; puis ?", "nbsp"); got != want {
		t.Fatalf("mixed sub-rules: got %q, want %q", got, want)
	}

	for _, locale := range []string{"en-US", "de-DE", "ru"} {
		input := "She said “hi” — really..."
		withOption, err := polytypo.Transform(input, polytypo.Options{Locale: locale, NarrowNbsp: "nbsp"})
		if err != nil {
			t.Fatal(err)
		}
		without, err := polytypo.Transform(input, polytypo.Options{Locale: locale})
		if err != nil {
			t.Fatal(err)
		}
		if withOption != without {
			t.Fatalf("%s: the option is not a no-op where nothing emits U+202F", locale)
		}
	}
}

func TestNarrowNbspIdempotency(t *testing.T) {
	for _, input := range []string{"Un délai ? Vraiment !", "Oui" + nnbsp + "?", "Il a dit : « oui » ;"} {
		once := fr(t, input, "nbsp")
		if twice := fr(t, once, "nbsp"); twice != once {
			t.Fatalf("%q: not a fixed point: %q then %q", input, once, twice)
		}
	}

	// Why the option moves the target instead of post-processing: a caller's replacement is
	// stable only as long as it always runs. Feed it back through the default pipeline and N2
	// converts it straight back.
	postProcessed := strings.ReplaceAll(fr(t, "Un délai ?", ""), nnbsp, nbsp)
	if postProcessed != "Un délai"+nbsp+"?" {
		t.Fatalf("premise: got %q", postProcessed)
	}
	if got := fr(t, postProcessed, ""); got != "Un délai"+nnbsp+"?" {
		t.Fatalf("post-processing was expected not to be a fixed point, got %q", got)
	}
}

func TestNarrowNbspValidation(t *testing.T) {
	codeOf := func(opts polytypo.Options) string {
		if _, err := polytypo.Transform("x", opts); err != nil {
			if perr, ok := err.(*polytypo.Error); ok {
				return string(perr.Code)
			}
			return "WRONG TYPE"
		}
		return "NO THROW"
	}

	cases := []struct {
		name string
		opts polytypo.Options
		want polytypo.ErrorCode
	}{
		{"unknown value", polytypo.Options{Locale: "fr", NarrowNbsp: "wide"}, polytypo.CodeInvalidOption},
		{"mode wins", polytypo.Options{Locale: "fr", Mode: "yaml", NarrowNbsp: "wide"}, polytypo.CodeInvalidMode},
		{"beats an unknown rule", polytypo.Options{Locale: "fr", NarrowNbsp: "wide", Rules: map[string]bool{"nope": true}}, polytypo.CodeInvalidOption},
		{"beats an unknown locale", polytypo.Options{Locale: "xx", NarrowNbsp: "wide"}, polytypo.CodeInvalidOption},
		// The check belongs to the call, not to the rule.
		{"raises with nbsp disabled", polytypo.Options{Locale: "fr", NarrowNbsp: "wide", Rules: map[string]bool{"nbsp": false}}, polytypo.CodeInvalidOption},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := codeOf(tc.opts); got != string(tc.want) {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}

	if got := fr(t, "Un délai ?", "narrow"); got != "Un délai"+nnbsp+"?" {
		t.Fatalf("explicit default: got %q", got)
	}
}
