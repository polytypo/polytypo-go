// spec/rules/modes.md 3.9 (spec 1.13.0): the protect option. The conformance fixtures cover the
// behaviour and the refusals Go can express (TestFixtures); what lives here is what a fixture
// cannot say: the validation order across modes, [] against absent, offsets in code points rather
// than bytes, idempotency under the ranges moved through the edits (3.9.5), and analyze's ranges
// (3.9.6, analyze.md 6).
package polytypo_test

import (
	"strings"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
	"github.com/polytypo/polytypo-go/internal/engine"
	"github.com/polytypo/polytypo-go/internal/modes"
)

func rng(start, end int) []polytypo.Range { return []polytypo.Range{{Start: start, End: end}} }

func wantCode(t *testing.T, err error, code polytypo.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s, got no error", code)
	}
	got, ok := codeOf(err)
	if !ok || got != string(code) {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func TestProtectRefusals(t *testing.T) {
	const input = "a -- b -- c"
	cases := []struct {
		name    string
		protect []polytypo.Range
	}{
		{"overlap", []polytypo.Range{{Start: 0, End: 4}, {Start: 3, End: 6}}},
		{"descending", []polytypo.Range{{Start: 5, End: 7}, {Start: 0, End: 2}}},
		{"empty range", rng(2, 2)},
		{"inverted range", rng(3, 2)},
		{"negative start", rng(-1, 2)},
		{"past the end", rng(9, 12)},
		{"one past the end", rng(0, len([]rune(input))+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := polytypo.Options{Locale: "en-US", Protect: tc.protect}
			_, err := polytypo.Transform(input, opts)
			wantCode(t, err, polytypo.CodeInvalidOption)
			_, err = polytypo.Analyze(input, opts)
			wantCode(t, err, polytypo.CodeInvalidOption)
		})
	}
}

// 3.9.2: the bound is the input's length in code points. An offset taken in bytes (what a Go
// template parser reports) overshoots as soon as the input holds one multi-byte character.
func TestProtectBoundIsCodePointsNotBytes(t *testing.T) {
	input := "é😀 -- x"
	if len(input) == len([]rune(input)) {
		t.Fatal("test input must be multi-byte")
	}
	n := len([]rune(input))
	if _, err := polytypo.Transform(input, polytypo.Options{Locale: "en-US", Protect: rng(0, n)}); err != nil {
		t.Fatalf("a range ending at the code-point length must be accepted: %v", err)
	}
	_, err := polytypo.Transform(input, polytypo.Options{Locale: "en-US", Protect: rng(0, len(input))})
	wantCode(t, err, polytypo.CodeInvalidOption)
}

func TestProtectNonEmptyIsRefusedOutsideText(t *testing.T) {
	cases := []struct {
		name  string
		input string
		opts  polytypo.Options
	}{
		{"html", "<p>a -- b</p>", polytypo.Options{Mode: "html"}},
		{"markdown", "a -- b\n", polytypo.Options{Mode: "markdown", Dialect: "commonmark"}},
		{"yaml", "title: a -- b\n", polytypo.Options{Mode: "yaml", Keys: []string{"title"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.opts.Locale = "en-US"
			plain, err := polytypo.Transform(tc.input, tc.opts)
			if err != nil {
				t.Fatal(err)
			}

			tc.opts.Protect = []polytypo.Range{}
			empty, err := polytypo.Transform(tc.input, tc.opts)
			if err != nil || empty != plain {
				t.Fatalf("[] must equal absent: %q, %v (want %q)", empty, err, plain)
			}
			if _, err := polytypo.Analyze(tc.input, tc.opts); err != nil {
				t.Fatalf("Analyze with []: %v", err)
			}

			tc.opts.Protect = rng(0, 1)
			_, err = polytypo.Transform(tc.input, tc.opts)
			wantCode(t, err, polytypo.CodeInvalidOption)
			_, err = polytypo.Analyze(tc.input, tc.opts)
			wantCode(t, err, polytypo.CodeInvalidOption)
		})
	}
}

func TestProtectEmptyEqualsAbsent(t *testing.T) {
	const input = `He said "hi" -- and left...`
	absent, err := polytypo.Transform(input, polytypo.Options{Locale: "en-US"})
	if err != nil {
		t.Fatal(err)
	}
	for _, protect := range [][]polytypo.Range{nil, {}} {
		got, err := polytypo.Transform(input, polytypo.Options{Locale: "en-US", Protect: protect})
		if err != nil || got != absent {
			t.Fatalf("Protect %#v: %q, %v (want %q)", protect, got, err, absent)
		}
	}
}

// 3.9.2 "Validation order": mode, narrowNbsp, rules, locale, the mode's own dialect and keys, then
// protect. A call with two bad options reports the earlier one.
func TestProtectValidationOrder(t *testing.T) {
	bad := rng(5, 2)
	nonEmpty := rng(0, 1)
	cases := []struct {
		name  string
		input string
		opts  polytypo.Options
		code  polytypo.ErrorCode
	}{
		{"mode before protect", "a", polytypo.Options{Locale: "en-US", Mode: "asciidoc", Protect: bad}, polytypo.CodeInvalidMode},
		{"narrowNbsp before protect", "a", polytypo.Options{Locale: "en-US", NarrowNbsp: "wide", Protect: bad}, polytypo.CodeInvalidOption},
		{"rules before protect", "a", polytypo.Options{Locale: "en-US", Rules: map[string]bool{"nosuchrule": true}, Protect: bad}, polytypo.CodeUnknownRule},
		{"locale before protect", "a", polytypo.Options{Locale: "xx", Protect: bad}, polytypo.CodeUnknownLocale},
		{"dialect before protect, text", "a", polytypo.Options{Locale: "en-US", Dialect: "commonmark", Protect: bad}, polytypo.CodeInvalidDialect},
		{"dialect before protect, html", "<p>a</p>", polytypo.Options{Locale: "en-US", Mode: "html", Dialect: "commonmark", Protect: nonEmpty}, polytypo.CodeInvalidDialect},
		{"missing dialect before protect, markdown", "a", polytypo.Options{Locale: "en-US", Mode: "markdown", Protect: nonEmpty}, polytypo.CodeInvalidDialect},
		{"unsupported dialect before protect, markdown", "a", polytypo.Options{Locale: "en-US", Mode: "markdown", Dialect: "mdx", Protect: nonEmpty}, polytypo.CodeInvalidDialect},
		{"locale before protect, html", "<p>a</p>", polytypo.Options{Locale: "xx", Mode: "html", Protect: nonEmpty}, polytypo.CodeUnknownLocale},
		{"rules before protect, yaml", "k: a\n", polytypo.Options{Locale: "en-US", Mode: "yaml", Keys: []string{"k"}, Rules: map[string]bool{"nosuchrule": true}, Protect: nonEmpty}, polytypo.CodeUnknownRule},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := polytypo.Transform(tc.input, tc.opts)
			wantCode(t, err, tc.code)
			_, err = polytypo.Analyze(tc.input, tc.opts)
			wantCode(t, err, tc.code)
		})
	}

	// keys and protect share POLYTYPO_INVALID_OPTION, so the order is read off the message.
	for _, fn := range []string{"Transform", "Analyze"} {
		opts := polytypo.Options{Locale: "en-US", Mode: "yaml", Protect: nonEmpty}
		var err error
		if fn == "Transform" {
			_, err = polytypo.Transform("k: a\n", opts)
		} else {
			_, err = polytypo.Analyze("k: a\n", opts)
		}
		wantCode(t, err, polytypo.CodeInvalidOption)
		if !strings.Contains(err.Error(), "keys") {
			t.Fatalf("%s: a missing keys option must be reported before protect, got %v", fn, err)
		}
	}
}

// 3.9.1: touching ranges are one run, and a range written as two touching halves behaves as the
// single range.
func TestProtectTouchingRangesAreOneRun(t *testing.T) {
	const input = "a ${x}${y} - b"
	whole, err := polytypo.Transform(input, polytypo.Options{Locale: "en-GB", Protect: rng(2, 10)})
	if err != nil {
		t.Fatal(err)
	}
	split, err := polytypo.Transform(input, polytypo.Options{Locale: "en-GB", Protect: []polytypo.Range{{Start: 2, End: 6}, {Start: 6, End: 10}}})
	if err != nil {
		t.Fatal(err)
	}
	if whole != split || whole != "a ${x}${y} – b" {
		t.Fatalf("whole %q, split %q", escapeNonASCII(whole), escapeNonASCII(split))
	}
}

// 3.9.2: offsets are code points. An astral character before the hole and inside it, in the
// output and in analyze's offsets.
func TestProtectAstralOffsets(t *testing.T) {
	const input = "\U0001F600 -- \U0001F600${\U0001F600x} -- \U0001F600"
	cp := []rune(input)
	start := strings.Index(input, "${")
	start = len([]rune(input[:start]))
	end := start + len([]rune("${\U0001F600x}"))
	opts := polytypo.Options{Locale: "en-US", Protect: rng(start, end)}

	out, err := polytypo.Transform(input, opts)
	if err != nil {
		t.Fatal(err)
	}
	if want := "\U0001F600—\U0001F600${\U0001F600x}—\U0001F600"; out != want {
		t.Fatalf("out = %q, want %q", escapeNonASCII(out), escapeNonASCII(want))
	}

	changes := mustAnalyze(t, input, opts)
	if len(changes) != 2 {
		t.Fatalf("want 2 changes, got %+v", changes)
	}
	for _, c := range changes {
		if c.Start < 0 || c.End > len(cp) || c.Start > c.End {
			t.Fatalf("offset outside the input: %+v", c)
		}
		if got := string(cp[c.Start:c.End]); got != c.Before {
			t.Fatalf("offsets %d..%d select %q, change says Before %q", c.Start, c.End, got, c.Before)
		}
		if c.End > start && c.Start < end {
			t.Fatalf("change %+v overlaps the protected run %d..%d", c, start, end)
		}
	}
	if changes[0].Start != 1 || changes[0].End != 5 {
		t.Fatalf("first change (the spaced dash): want 1..5, got %d..%d", changes[0].Start, changes[0].End)
	}
}

// analyze.md 2 and 6, since 1.13.0: a non-insertion's end is one past the last replaced code point
// that has an origin. 1.12.0 reported this document's "--" as 4..21, across the skipped <code>.
func TestAnalyzeEndDoesNotCrossASkippedRegion(t *testing.T) {
	cases := []struct {
		name  string
		input string
		opts  polytypo.Options
	}{
		{"html", "<p>x--<code>cc</code>y</p>", polytypo.Options{Locale: "en-US", Mode: "html"}},
		{"markdown", "x--`cc`y\n", polytypo.Options{Locale: "en-US", Mode: "markdown", Dialect: "commonmark"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp := []rune(tc.input)
			changes := mustAnalyze(t, tc.input, tc.opts)
			if len(changes) == 0 {
				t.Fatal("expected a change")
			}
			first := changes[0]
			if first.RuleID != "dashes" || first.Before != "--" || first.End-first.Start != 2 {
				t.Fatalf("want the dashes change over exactly the two hyphens, got %+v", first)
			}
			if got := string(cp[first.Start:first.End]); got != "--" {
				t.Fatalf("offsets select %q", got)
			}
		})
	}

	changes := mustAnalyze(t, "<p>x--<code>cc</code>y</p>", polytypo.Options{Locale: "en-US", Mode: "html"})
	if len(changes) != 1 || changes[0].Start != 4 || changes[0].End != 6 {
		t.Fatalf("want exactly 4..6, got %+v", changes)
	}
}

// 3.9.5: transform(out, protect') == out, where protect' locates in out the code points protect
// located in in. A deterministic sweep over random documents with holes, also checking analyze
// (A3, A5, and no reported range overlapping a hole). Kept at a few thousand documents: CI runs
// this under -race.
func TestProtectSweep(t *testing.T) {
	tokens := []string{
		"a", "word", "Ab", "x", "5", "12", " ", " ", " ", "  ", "-", "--", "---", "...", ".", ",", "!", "?",
		":", ";", `"`, "'", "(", ")", "\n", "–", "—", "'s", "n't", "km", "%", "\U0001F600", "é",
		"«", "»", "„", "“", "”",
	}
	holeTexts := []string{"${x}", "{n}", "<b>", "{a, plural, one {#}}", "'{b}'", "{\n}", " "}
	locales := []string{"en-US", "en-GB", "de-DE", "fr", "ru", "fi", "pl", "tr"}
	const perLocale = 300

	seed := uint32(20261022)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int((seed >> 8) % uint32(n))
	}

	checked, withHoles := 0, 0
	for _, locale := range locales {
		_, localeData, plan, err := engine.Prepare(locale, nil)
		if err != nil {
			t.Fatal(err)
		}
		narrow, err := engine.ResolveNarrowTarget("")
		if err != nil {
			t.Fatal(err)
		}
		ctx := engine.RuleContext{Mode: "text", Locale: locale, NarrowTarget: narrow}

		for n := 0; n < perLocale; n++ {
			var doc strings.Builder
			var holes []polytypo.Range
			length := 0
			for i, k := 0, 3+next(8); i < k; i++ {
				if next(5) == 0 {
					h := holeTexts[next(len(holeTexts))]
					size := len([]rune(h))
					holes = append(holes, polytypo.Range{Start: length, End: length + size})
					doc.WriteString(h)
					length += size
					continue
				}
				tok := tokens[next(len(tokens))]
				doc.WriteString(tok)
				length += len([]rune(tok))
			}
			input := doc.String()
			cp := []rune(input)
			opts := polytypo.Options{Locale: locale, Protect: holes}
			checked++
			if len(holes) > 0 {
				withHoles++
			}

			out, err := polytypo.Transform(input, opts)
			if err != nil {
				t.Fatalf("%s %q %v: %v", locale, input, holes, err)
			}
			runs, err := engine.ResolveProtect(holes, length)
			if err != nil {
				t.Fatal(err)
			}
			mapped, moved := out, runs
			if len(runs) > 0 {
				outCP, m, err := modes.RunProtectedMapped(cp, runs, plan, localeData, ctx)
				if err != nil {
					t.Fatal(err)
				}
				mapped, moved = string(outCP), m
				for i, run := range runs {
					if string(outCP[moved[i].Start:moved[i].End]) != string(cp[run.Start:run.End]) {
						t.Fatalf("%s %q: protected run %d was not returned byte for byte", locale, input, i)
					}
				}
			}
			if out != mapped {
				t.Fatalf("%s %q: Transform %q != mapped %q", locale, input, out, mapped)
			}

			again, err := polytypo.Transform(out, polytypo.Options{Locale: locale, Protect: moved})
			if err != nil {
				t.Fatal(err)
			}
			if again != out {
				t.Fatalf("%s %q holes %v: not idempotent under the moved ranges: %q -> %q", locale, input, holes, escapeNonASCII(out), escapeNonASCII(again))
			}

			changes, err := polytypo.Analyze(input, opts)
			if err != nil {
				t.Fatal(err)
			}
			if (len(changes) == 0) != (out == input) {
				t.Fatalf("%s %q: A3 violated, %d changes, out == input is %v", locale, input, len(changes), out == input)
			}
			for _, c := range changes {
				if c.Start < 0 || c.Start > c.End || c.End > length {
					t.Fatalf("%s %q: A5 violated, %+v", locale, input, c)
				}
				for _, run := range runs {
					overlaps := c.Start < run.End && c.End > run.Start
					if c.Start == c.End {
						overlaps = c.Start > run.Start && c.Start < run.End
					}
					if overlaps {
						t.Fatalf("%s %q: change %+v overlaps protected run %v", locale, input, c, run)
					}
				}
			}
		}
	}
	if withHoles < checked/2 {
		t.Fatalf("sweep is not exercising holes: %d of %d", withHoles, checked)
	}
	t.Logf("%d documents, %d with holes", checked, withHoles)
}
