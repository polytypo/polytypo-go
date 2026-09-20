// spec/rules/analyze.md — the contract is A1...A5; the decomposition is observation.
//
// Mirrors polytypo-js's tests/engine/analyze.test.ts and the Python port's
// tests/engine/test_analyze.py case for case, including the two that are cheap to get wrong
// (analyze.md section 6): A3 over the whole embedded corpus, and document offsets under the mode
// adapters.
package polytypo_test

import (
	"strings"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
	"github.com/polytypo/polytypo-go/internal/spec"
)

var ruleOrder = []string{
	"spaces", "ellipsis", "ranges", "dashes", "hyphen", "quotes", "apostrophe", "symbols", "nbsp",
}

func orderOf(ruleID string) int {
	for i, id := range ruleOrder {
		if id == ruleID {
			return i
		}
	}
	return -1
}

func ruleIDs(changes []polytypo.Change) []string {
	ids := make([]string, 0, len(changes))
	for _, c := range changes {
		ids = append(ids, c.RuleID)
	}
	return ids
}

func mustAnalyze(t *testing.T, input string, opts polytypo.Options) []polytypo.Change {
	t.Helper()
	changes, err := polytypo.Analyze(input, opts)
	if err != nil {
		t.Fatalf("Analyze(%q): unexpected error %v", input, err)
	}
	return changes
}

// A1 — accepts and rejects exactly what Transform does.
func TestAnalyzeRejectsWhatTransformRejects(t *testing.T) {
	cases := []struct {
		name string
		opts polytypo.Options
		code polytypo.ErrorCode
	}{
		{"unknown locale", polytypo.Options{Locale: "xx"}, polytypo.CodeUnknownLocale},
		{"unknown rule wins over unknown locale",
			polytypo.Options{Locale: "xx", Rules: map[string]bool{"nope": true}},
			polytypo.CodeUnknownRule},
		{"unknown mode", polytypo.Options{Locale: "en-US", Mode: "asciidoc"}, polytypo.CodeInvalidMode},
		{"markdown requires a dialect",
			polytypo.Options{Locale: "en-US", Mode: "markdown"}, polytypo.CodeInvalidDialect},
		{"dialect outside markdown",
			polytypo.Options{Locale: "en-US", Dialect: "commonmark"}, polytypo.CodeInvalidDialect},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := polytypo.Analyze("x", tc.opts)
			if err == nil {
				t.Fatal("expected an error")
			}
			code, ok := codeOf(err)
			if !ok || code != string(tc.code) {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
		})
	}
}

// A2 — pure.
func TestAnalyzeIsPure(t *testing.T) {
	input := `She said "hi" -- really...`
	opts := polytypo.Options{Locale: "en-US"}

	before, err := polytypo.Transform(input, opts)
	if err != nil {
		t.Fatal(err)
	}

	once := mustAnalyze(t, input, opts)
	twice := mustAnalyze(t, input, opts)
	if len(once) != len(twice) {
		t.Fatalf("two calls disagree: %d vs %d changes", len(once), len(twice))
	}
	for i := range once {
		if once[i] != twice[i] {
			t.Fatalf("change %d differs: %+v vs %+v", i, once[i], twice[i])
		}
	}

	after, err := polytypo.Transform(input, opts)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("Analyze changed what Transform returns: %q then %q", before, after)
	}
}

// A3 — empty exactly when Transform changes nothing.
func TestAnalyzeIsEmptyExactlyWhenTransformChangesNothing(t *testing.T) {
	if got := mustAnalyze(t, "Nothing to do here.", polytypo.Options{Locale: "en-US"}); len(got) != 0 {
		t.Fatalf("expected no changes, got %+v", got)
	}
	if got := mustAnalyze(t, "Wait...", polytypo.Options{Locale: "en-US"}); len(got) == 0 {
		t.Fatal("expected at least one change")
	}
}

// analyze.md section 6: the whole embedded corpus, the cheap strong version of A3.
func TestAnalyzeAgreesWithTransformOnEveryFixture(t *testing.T) {
	locales, err := spec.FixtureLocales()
	if err != nil {
		t.Fatal(err)
	}
	offenders := make([]string, 0)
	for _, loc := range locales {
		fx, err := spec.LoadFixtures(loc)
		if err != nil {
			t.Fatalf("loading fixtures for %s: %v", loc, err)
		}
		for _, c := range fx.Cases {
			if c.Throws != "" || c.Dialect == "mdx" {
				continue
			}
			opts := polytypo.Options{Locale: fx.Locale, Mode: c.Mode, Dialect: c.Dialect, Rules: c.Rules, Keys: c.Keys}
			out, err := polytypo.Transform(c.In, opts)
			if err != nil {
				t.Fatalf("%s/%s: Transform failed: %v", fx.Locale, c.ID, err)
			}
			changes, err := polytypo.Analyze(c.In, opts)
			if err != nil {
				t.Fatalf("%s/%s: Analyze failed: %v", fx.Locale, c.ID, err)
			}
			if (out != c.In) != (len(changes) > 0) {
				offenders = append(offenders, fx.Locale+"/"+c.ID)
			}
		}
	}
	if len(offenders) != 0 {
		t.Fatalf("Analyze and Transform disagree on: %s", strings.Join(offenders, ", "))
	}
}

// A4 — only rules that were enabled for the call.
func TestAnalyzeReportsOnlyEnabledRules(t *testing.T) {
	ids := ruleIDs(mustAnalyze(t, `She said "hi"...`,
		polytypo.Options{Locale: "en-US", Rules: map[string]bool{"quotes": false}}))
	if slicesContains(ids, "quotes") {
		t.Fatalf("reported a disabled rule: %v", ids)
	}
	if !slicesContains(ids, "ellipsis") {
		t.Fatalf("expected ellipsis among %v", ids)
	}

	const ranges = "chapters 3-5"
	if ids := ruleIDs(mustAnalyze(t, ranges, polytypo.Options{Locale: "en-US"})); slicesContains(ids, "ranges") {
		t.Fatalf("reported ranges without it being turned on: %v", ids)
	}
	turnedOn := mustAnalyze(t, ranges, polytypo.Options{Locale: "en-US", Rules: map[string]bool{"ranges": true}})
	if !slicesContains(ruleIDs(turnedOn), "ranges") {
		t.Fatalf("expected ranges once turned on, got %v", ruleIDs(turnedOn))
	}
}

// A5 — offsets are code points inside the input.
func TestAnalyzeReportsCodePointOffsets(t *testing.T) {
	t.Run("stays within bounds with astral characters", func(t *testing.T) {
		input := `A 😀 says "hi" and waits...`
		length := len([]rune(input))
		for _, c := range mustAnalyze(t, input, polytypo.Options{Locale: "en-US"}) {
			if c.Start < 0 || c.End > length || c.Start > c.End {
				t.Fatalf("offsets out of bounds for length %d: %+v", length, c)
			}
		}
	})

	t.Run("reports code-point offsets, not byte offsets", func(t *testing.T) {
		// The emoji is four UTF-8 bytes and one code point, so the opening quotation mark sits
		// at code point 6 and at byte index 9. A runtime reporting native string offsets — which
		// in Go means bytes — says 9.
		input := `😀 and "this"`
		changes := mustAnalyze(t, input, polytypo.Options{Locale: "en-US"})
		if len(changes) == 0 {
			t.Fatal("expected a change")
		}
		if changes[0].RuleID != "quotes" || changes[0].Start != 6 {
			t.Fatalf("expected quotes at code point 6, got %+v", changes[0])
		}
		if got := strings.Index(input, `"`); got != 9 {
			t.Fatalf("test premise broken: byte index of the quotation mark is %d, not 9", got)
		}
	})

	// analyze.md section 6: the mistake that passes every text-mode test.
	t.Run("html mode reports document offsets", func(t *testing.T) {
		input := `<p class="x">Wait...</p>`
		changes := mustAnalyze(t, input, polytypo.Options{Locale: "en-US", Mode: "html"})
		if len(changes) == 0 {
			t.Fatal("expected a change")
		}
		want := len([]rune(input[:strings.Index(input, "...")]))
		got := changes[0]
		if got.RuleID != "ellipsis" || got.Start != want || got.Before != "..." || got.After != "…" {
			t.Fatalf("expected ellipsis at %d replacing ...: %+v", want, got)
		}
	})

	t.Run("html mode reports document offsets in a later span", func(t *testing.T) {
		// Three spans, a non-ASCII character before the change, and the change in the third
		// span: the two markers are the only code points in the joined array with no origin, so
		// a doubled or dropped one shifts this offset and nothing in a one- or two-span document
		// would notice. `café` then puts the byte offset one ahead of the code-point offset, so
		// a byte offset leaking out of the span adapter cannot pass this either — which is the
		// mistake available to Go specifically.
		input := `<p>café</p><p>two</p><p>Wait... three</p>`
		changes := mustAnalyze(t, input, polytypo.Options{Locale: "en-US", Mode: "html"})
		if len(changes) == 0 {
			t.Fatal("expected a change")
		}
		bytes := strings.Index(input, "...")
		want := len([]rune(input[:bytes]))
		if want != bytes-1 {
			t.Fatalf("test premise broken: code-point offset %d, byte offset %d", want, bytes)
		}
		if changes[0].RuleID != "ellipsis" || changes[0].Start != want {
			t.Fatalf("expected ellipsis at %d, got %+v", want, changes[0])
		}
	})

	t.Run("markdown mode reports document offsets", func(t *testing.T) {
		input := "# Title\n\nWait... here\n"
		changes := mustAnalyze(t, input,
			polytypo.Options{Locale: "en-US", Mode: "markdown", Dialect: "commonmark"})
		if len(changes) == 0 {
			t.Fatal("expected a change")
		}
		want := len([]rune(input[:strings.Index(input, "...")]))
		if changes[0].Start != want {
			t.Fatalf("expected a change at %d, got %+v", want, changes[0])
		}
	})
}

// Section 3 — pipeline order, and section 5 — overlap.
func TestAnalyzeReportsInPipelineOrder(t *testing.T) {
	ids := ruleIDs(mustAnalyze(t, `She said "hi" -- wait...`, polytypo.Options{Locale: "en-US"}))
	for i := 1; i < len(ids); i++ {
		if orderOf(ids[i-1]) > orderOf(ids[i]) {
			t.Fatalf("changes are not in spec order: %v", ids)
		}
	}
}

func TestAnalyzeReportsBothRulesOnTheSameOriginalRange(t *testing.T) {
	// The French case analyze.md section 5 is written around: two rules, one original index.
	// `spaces` removes the space at 3–4 and `nbsp` inserts at 4–4, in front of the colon the
	// caller wrote at 4 — the second change's position is the colon's, not the deleted space's.
	changes := mustAnalyze(t, "Oui : non", polytypo.Options{Locale: "fr"})
	if got := ruleIDs(changes); len(got) != 2 || got[0] != "spaces" || got[1] != "nbsp" {
		t.Fatalf("expected [spaces nbsp], got %v", got)
	}
	if changes[0].Before != " " || changes[0].After != "" ||
		changes[0].Start != 3 || changes[0].End != 4 {
		t.Fatalf("expected the space deleted at 3–4, got %+v", changes[0])
	}
	if changes[1].After != " " || changes[1].Start != 4 || changes[1].End != 4 {
		t.Fatalf("expected U+00A0 inserted at 4–4, got %+v", changes[1])
	}
}

func slicesContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
