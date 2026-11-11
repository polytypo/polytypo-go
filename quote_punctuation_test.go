// spec/rules/quote-punctuation.md (spec 1.21.0): the opt-in rule that moves a comma or full stop
// before a closing primary quotation mark. The conformance fixtures cover the worked examples
// (TestFixtures); what lives here is the edge cases they leave out and the idempotency invariant
// with the rule enabled, which idempotency_test.go never exercises because the rule is off by
// default.
package polytypo_test

import (
	"strings"
	"testing"

	"pgregory.net/rapid"

	polytypo "github.com/polytypo/polytypo-go"
	"github.com/polytypo/polytypo-go/internal/engine"
	"github.com/polytypo/polytypo-go/internal/spec"
)

func qpOn(locale string) polytypo.Options {
	return polytypo.Options{Locale: locale, Rules: map[string]bool{"quote-punctuation": true}}
}

func TestQuotePunctuationRegistration(t *testing.T) {
	order, err := engine.RuleOrder()
	if err != nil {
		t.Fatal(err)
	}
	idx := map[string]int{}
	for i, id := range order {
		idx[id] = i
	}
	if idx["quote-punctuation"] != idx["quotes"]+1 || idx["apostrophe"] != idx["quote-punctuation"]+1 {
		t.Fatalf("quote-punctuation must sit between quotes and apostrophe: %v", order)
	}
	defaults, err := engine.RuleDefaults()
	if err != nil {
		t.Fatal(err)
	}
	if defaults["quote-punctuation"] {
		t.Fatal("quote-punctuation must default to off")
	}
	if engine.GetRule("quote-punctuation") == nil {
		t.Fatal("quote-punctuation is not registered")
	}
}

func TestQuotePunctuationOnlyEnUSDeclaresInside(t *testing.T) {
	for _, id := range allLocales(t) {
		data, err := spec.LoadLocale(id)
		if err != nil {
			t.Fatal(err)
		}
		want := "none"
		if id == "en-US" {
			want = "inside"
		}
		if got := data.Quotes.PunctuationPlacement; got != want {
			t.Errorf("%s: punctuationPlacement = %q, want %q", id, got, want)
		}
	}
}

func TestQuotePunctuationCases(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"comma", `She said "hello", then left.`, `She said “hello,” then left.`},
		{"stop then space", `He called it "done". Then he left.`, `He called it “done.” Then he left.`},
		{"end of text", `Call it "ready".`, `Call it “ready.”`},
		{"line break", "Call it \"ready\".\nNext", "Call it “ready.”\nNext"},
		{"already inside", `She said “hello,” then left.`, `She said “hello,” then left.`},
		{"digit", `Route "66".`, `Route “66”.`},
		{"inch mark", `6’ 2”, tall`, `6’ 2”, tall`},
		{"terminal", `"Stop!".`, `“Stop!”.`},
		{"ellipsis", `It said "wait…".`, `It said “wait…”.`},
		{"abbreviation", `Use "e.g.", then`, `Use “e.g.”, then`},
		{"nested", `"He said 'hi'".`, `“He said ‘hi’”.`},
		{"semicolon colon question", `Say "a"; say "b": say "c"?`, `Say “a”; say “b”: say “c”?`},
		{"letter follows", `"file".txt`, `“file”.txt`},
		{"bracket follows", `He said ("hi".)`, `He said (“hi”.)`},
		{"dots follow", `He said "wait"...`, `He said “wait”…`},
		{"secondary close", `Check the dogs’, bowls and cats.`, `Check the dogs’, bowls and cats.`},
		{"two edits", `The "first", the "second".`, `The “first,” the “second.”`},
		{"middle untouched", `Both "a", "b" and "c".`, `Both “a,” “b” and “c.”`},
		{"unpaired", `Broken pair”, then`, `Broken pair,” then`},
		{"mark and stop only", `”.`, `”.`},
		{"empty", ``, ``},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := polytypo.Transform(c.in, qpOn("en-US"))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("Transform(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestQuotePunctuationOffByDefault(t *testing.T) {
	got, err := polytypo.Transform(`She said "hello", then left.`, polytypo.Options{Locale: "en-US"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `She said “hello”, then left.`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestQuotePunctuationNoOpWhereNone(t *testing.T) {
	for _, id := range allLocales(t) {
		if id == "en-US" {
			continue
		}
		in := `She said "hello", then left.`
		on, err := polytypo.Transform(in, qpOn(id))
		if err != nil {
			t.Fatal(err)
		}
		off, err := polytypo.Transform(in, polytypo.Options{Locale: id})
		if err != nil {
			t.Fatal(err)
		}
		if on != off {
			t.Errorf("%s: enabling the rule changed %q -> %q (off: %q)", id, in, on, off)
		}
	}
}

func TestQuotePunctuationModes(t *testing.T) {
	cases := []struct {
		name, mode, dialect, in, want string
		keys                          []string
	}{
		{"html one node", "html", "", `<p>She said "hello", then left.</p>`, `<p>She said “hello,” then left.</p>`, nil},
		{"html boundary at i+1", "html", "", `<p>She said <em>"hello"</em>, then left.</p>`, `<p>She said <em>“hello”</em>, then left.</p>`, nil},
		{"html boundary at i-1", "html", "", `<p>She said "<em>hello</em>", then left.</p>`, `<p>She said “<em>hello</em>”, then left.</p>`, nil},
		{"html boundary at i+2", "html", "", `<p>She said "hello".</p><p>Next.</p>`, `<p>She said “hello”.</p><p>Next.</p>`, nil},
		{"html end of unit", "html", "", `<p>Call it "ready".</p>`, `<p>Call it “ready.”</p>`, nil},
		{"markdown one span", "markdown", "commonmark", "She said \"hello\", then left.\n", "She said “hello,” then left.\n", nil},
		{"markdown closing delimiter", "markdown", "commonmark", `*She said "hello"*, then left.`, `*She said “hello”*, then left.`, nil},
		{"markdown delimiter after stop", "markdown", "commonmark", `*He said "hi".* Next`, `*He said “hi”.* Next`, nil},
		{"yaml scalar", "yaml", "", `title: She said "hello", then left.`, `title: She said “hello,” then left.`, []string{"title"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := qpOn("en-US")
			opts.Mode, opts.Dialect, opts.Keys = c.mode, c.dialect, c.keys
			got, err := polytypo.Transform(c.in, opts)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("Transform(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestQuotePunctuationProtect(t *testing.T) {
	cases := []struct {
		name, in, want string
		protect        []polytypo.Range
	}{
		{"protected after stop", `Set "file".${ext} now`, `Set “file”.${ext} now`, rng(12, 18)},
		{"protected before mark", `Use "${name}", then stop.`, `Use “${name}”, then stop.`, rng(5, 12)},
		{"protected elsewhere", `Say "hi", then ${x}.`, `Say “hi,” then ${x}.`, rng(15, 19)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := qpOn("en-US")
			opts.Protect = c.protect
			got, err := polytypo.Transform(c.in, opts)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("Transform(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestQuotePunctuationAnalyze(t *testing.T) {
	changes := mustAnalyze(t, `The "first", the "second".`, qpOn("en-US"))
	n := 0
	for _, ch := range changes {
		if ch.RuleID == "quote-punctuation" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("want 2 quote-punctuation changes, got %d: %+v", n, changes)
	}
	off := mustAnalyze(t, `The "first", the "second".`, polytypo.Options{Locale: "en-US"})
	if slicesContains(ruleIDs(off), "quote-punctuation") {
		t.Fatalf("reported quote-punctuation without it being turned on: %v", ruleIDs(off))
	}
}

// qpAlphabet is what the rule's conditions and its neighbours compete over (quote-punctuation.md
// 5): closing marks, stops, letters, digits, spaces, brackets, dashes and the apostrophe.
var qpAlphabet = []rune{
	'"', '\'', '“', '”', '‘', '’', '«', '»', 'a', 'B', '1', ',', '.', ';', '!', '…',
	' ', ' ', '\n', '(', ')', '-', '—',
}

func assertQPIdempotent(t interface {
	Fatalf(string, ...any)
}, text, locale string) {
	once, err := polytypo.Transform(text, qpOn(locale))
	if err != nil {
		t.Fatalf("transform(%q): %v", text, err)
	}
	twice, err := polytypo.Transform(once, qpOn(locale))
	if err != nil {
		t.Fatalf("transform(transform(%q)): %v", text, err)
	}
	if twice != once {
		t.Fatalf("not idempotent for locale %s: input %q -> %q -> %q", locale, text, once, twice)
	}
}

func TestQuotePunctuationIdempotentOverAlphabet(t *testing.T) {
	locales := allLocales(t)
	rapid.Check(t, func(t *rapid.T) {
		locale := rapid.SampledFrom(locales).Draw(t, "locale")
		n := rapid.IntRange(0, 24).Draw(t, "n")
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteRune(rapid.SampledFrom(qpAlphabet).Draw(t, "ch"))
		}
		assertQPIdempotent(t, b.String(), locale)
	})
}

// The documents are enumerated, not sampled, because a defect at this size cannot hide from a
// sweep. Sized by document count (3,906 over a five-character alphabet in en-US, where the rule
// acts) so it stays small under -race; the other locales are the random test's job.
func TestQuotePunctuationIdempotentBoundedSweep(t *testing.T) {
	alphabet := []rune{'”', 'a', ',', '.', ' '}
	docs := 0
	bestBoundedStrings(alphabet, 5, func(s string) {
		docs++
		assertQPIdempotent(t, s, "en-US")
	})
	t.Logf("%d documents", docs)
}
