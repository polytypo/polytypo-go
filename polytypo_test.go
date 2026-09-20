// The conformance runner (docs/ARCHITECTURE.md section 6). Every case in internal/spec/fixtures/
// is driven through the public polytypo.Transform, and every non-throwing case is also an
// idempotency case. Nothing here knows about individual locales or rules: fixtures are
// discovered at test time, mirroring the JS and Python reference implementations' own runners.
package polytypo_test

import (
	"fmt"
	"strings"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
	"github.com/polytypo/polytypo-go/internal/spec"
)

// escapeNonASCII: a diff full of invisible U+00A0 and U+202F is unreviewable
// (ARCHITECTURE.md 6.1) — used only in test failure messages, never in the comparison itself.
func escapeNonASCII(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, "\\u%04x", r)
		}
	}
	return b.String()
}

func codeOf(err error) (string, bool) {
	perr, ok := err.(*polytypo.Error)
	if !ok {
		return "", false
	}
	return string(perr.Code), true
}

func TestFixtures(t *testing.T) {
	locales, err := spec.FixtureLocales()
	if err != nil {
		t.Fatal(err)
	}
	if len(locales) == 0 {
		t.Fatal("no fixture locales found")
	}

	total := 0
	for _, loc := range locales {
		fx, err := spec.LoadFixtures(loc)
		if err != nil {
			t.Fatalf("loading fixtures for %s: %v", loc, err)
		}
		for _, c := range fx.Cases {
			c := c
			total++
			t.Run(loc+"/"+c.ID, func(t *testing.T) {
				if c.Dialect == "mdx" {
					t.Skip(`mdx dialect is not supported by this runtime (POLYTYPO_INVALID_DIALECT) -- ` +
						`an accepted, narrower conformance claim; see spec/CONFORMANCE.md`)
				}

				// The SAME opts value is used below for the idempotency re-run. That is contract
				// as of spec 1.3.0 (ARCHITECTURE.md 6.1), not convenience: a case carrying
				// NarrowNbsp or Keys is a fixed point under those options and not under the
				// defaults -- and with Keys the re-run would not even run, the option having no
				// default.
				opts := polytypo.Options{
					Locale:     fx.Locale,
					Mode:       c.Mode,
					Dialect:    c.Dialect,
					Rules:      c.Rules,
					NarrowNbsp: c.NarrowNbsp,
					Keys:       c.Keys,
				}

				if c.Throws != "" {
					out, err := polytypo.Transform(c.In, opts)
					if err == nil {
						t.Fatalf("expected error %s, got output %q", c.Throws, out)
					}
					code, ok := codeOf(err)
					if !ok {
						t.Fatalf("error is not *polytypo.Error: %#v", err)
					}
					if code != c.Throws {
						t.Fatalf("code = %s, want %s", code, c.Throws)
					}
					return
				}

				if c.Out == nil {
					t.Fatal("fixture case has neither out nor throws")
				}
				expected := *c.Out

				got, err := polytypo.Transform(c.In, opts)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != expected {
					t.Fatalf("out = %q, want %q", escapeNonASCII(got), escapeNonASCII(expected))
				}

				// Free coverage, and the most common port bug (ARCHITECTURE.md 6.1).
				twice, err := polytypo.Transform(expected, opts)
				if err != nil {
					t.Fatalf("idempotency transform errored: %v", err)
				}
				if twice != expected {
					t.Fatalf("not idempotent: transform(out) = %q, out = %q",
						escapeNonASCII(twice), escapeNonASCII(expected))
				}
			})
		}
	}
	if total == 0 {
		t.Fatal("conformance suite is empty")
	}
}

var resolutionProbes = []string{
	`He said "so" -- and left...`,
	"Pages 1999-2005, see  p. 7 .",
	"Really?.. 50 % (c) 2026",
}

func TestLocaleResolution(t *testing.T) {
	res, err := spec.LoadResolutionFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cases) == 0 {
		t.Fatal("locale-resolution fixture is empty")
	}

	for _, c := range res.Cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			tag := c.Tag // zero value "" for a tagAbsent case -- Go's static Options.Locale
			// string has no separate "absent" representation, and "" already has no accepted
			// shape per locale-resolution.md 3.3, so it throws the same way.

			if c.Throws != "" {
				_, err := polytypo.Transform("plain text", polytypo.Options{Locale: tag})
				if err == nil {
					t.Fatalf("expected error %s, got no error", c.Throws)
				}
				code, ok := codeOf(err)
				if !ok {
					t.Fatalf("error is not *polytypo.Error: %#v", err)
				}
				if code != c.Throws {
					t.Fatalf("code = %s, want %s", code, c.Throws)
				}
				return
			}

			for _, probe := range resolutionProbes {
				viaTag, err := polytypo.Transform(probe, polytypo.Options{Locale: tag})
				if err != nil {
					t.Fatalf("transform via tag %q: %v", tag, err)
				}
				viaCanonical, err := polytypo.Transform(probe, polytypo.Options{Locale: c.Resolves})
				if err != nil {
					t.Fatalf("transform via canonical %q: %v", c.Resolves, err)
				}
				if viaTag != viaCanonical {
					t.Fatalf("tag %q resolved differently than canonical %q for probe %q:\n via tag: %q\n via canonical: %q",
						tag, c.Resolves, probe, escapeNonASCII(viaTag), escapeNonASCII(viaCanonical))
				}
			}
		})
	}
}

// TestCharacterReferenceGuard pins spec/rules/modes.md's degenerate text-mode case against
// nbsp.md 3.3 step 4 (spec 1.3.0): fr lists ";" in narrowBeforePunctuation, and before the guard
// this rule inserted U+202F before the ";" that *ends* a character reference, so
// "Bonjour&#160;: oui" stopped being a reference at all. The conformance fixtures cover this too
// (fr-nbsp-character-reference-*), but only once the vendored spec is refreshed; this test is the
// one that fails today if the guard is removed.
func TestCharacterReferenceGuard(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Bonjour&#160;: oui", "Bonjour&#160;: oui"},
		{"Tom &amp; Jerry", "Tom &amp; Jerry"},
		{"a &notaname; b", "a &notaname; b"},
		{"Oui ; non", "Oui ; non"},
		{"Section 4; suite", "Section 4 ; suite"},
	}
	for _, c := range cases {
		got, err := polytypo.Transform(c.in, polytypo.Options{Locale: "fr"})
		if err != nil {
			t.Fatalf("Transform(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("Transform(%q) = %q, want %q", c.in, escapeNonASCII(got), escapeNonASCII(c.want))
		}
	}
}
