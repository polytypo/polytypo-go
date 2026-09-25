package polytypo_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
	"github.com/polytypo/polytypo-go/internal/modes"
)

// spec/rules/modes.md 3.7.4 -- "markdown" mode's optional FrontmatterKeys (spec 1.7.0).
//
// The conformance fixtures cover what the option converts. This file covers what a fixture cannot
// express and the claim the section asks a port to test directly: the option cannot change a byte
// outside the frontmatter block, because that block is a text unit of its own.

const fmDoc = "---\ntitle: He said \"hello\" once\nslug: \"a - b\"\n---\n\nBody \"quotes\" - here.\n"

func mdOf(t *testing.T, source, locale string, keys []string) string {
	t.Helper()
	out, err := polytypo.Transform(source, polytypo.Options{
		Locale:          locale,
		Mode:            "markdown",
		Dialect:         "commonmark",
		FrontmatterKeys: keys,
	})
	if err != nil {
		t.Fatalf("Transform(%q, keys=%q): %v", source, keys, err)
	}
	return out
}

func TestFrontmatterKeysProcessesListedScalarsOnly(t *testing.T) {
	want := "---\ntitle: He said “hello” once\nslug: \"a - b\"\n---\n\nBody “quotes”—here.\n"
	if got := mdOf(t, fmDoc, "en-US", []string{"title"}); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFrontmatterKeysAbsentIsThePre170Skip(t *testing.T) {
	want := "---\ntitle: He said \"hello\" once\nslug: \"a - b\"\n---\n\nBody “quotes”—here.\n"
	if got := mdOf(t, fmDoc, "en-US", nil); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFrontmatterKeysEmptyIsLegalAndYieldsNoSpans(t *testing.T) {
	if got, want := mdOf(t, fmDoc, "en-US", []string{}), mdOf(t, fmDoc, "en-US", nil); got != want {
		t.Fatalf("empty slice changed the document: %q", got)
	}
	if spans := modes.FrontmatterSpans(fmDoc, map[string]struct{}{}); len(spans) != 0 {
		t.Fatalf("empty key set produced %d span(s)", len(spans))
	}
}

func TestFrontmatterKeysMatchesBareNameAtAnyDepth(t *testing.T) {
	source := "---\nseo:\n  title: a \"b\"\ntitle: c \"d\"\nother:\n  slug: e \"f\"\n---\n\nx\n"
	want := "---\nseo:\n  title: a “b”\ntitle: c “d”\nother:\n  slug: e \"f\"\n---\n\nx\n"
	if got := mdOf(t, source, "en-US", []string{"title"}); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFrontmatterKeysLeavesTOMLAlone(t *testing.T) {
	source := "+++\ntitle = \"a - b\"\n+++\n\nBody - here.\n"
	want := "+++\ntitle = \"a - b\"\n+++\n\nBody—here.\n"
	if got := mdOf(t, source, "en-US", []string{"title"}); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if spans := modes.FrontmatterSpans(source, map[string]struct{}{"title": {}}); len(spans) != 0 {
		t.Fatalf("a TOML block produced %d span(s)", len(spans))
	}
}

func TestFrontmatterKeysAddsNoSpansWithoutABlock(t *testing.T) {
	for _, source := range []string{
		"---\ntitle: a \"b\"\n\nBody\n",        // unterminated
		"\n---\ntitle: a \"b\"\n---\n\nBody\n", // not at the start
	} {
		if got, want := mdOf(t, source, "en-US", []string{"title"}), mdOf(t, source, "en-US", nil); got != want {
			t.Fatalf("%q: option changed a document with no block: %q != %q", source, got, want)
		}
		if spans := modes.FrontmatterSpans(source, map[string]struct{}{"title": {}}); len(spans) != 0 {
			t.Fatalf("%q produced %d span(s)", source, len(spans))
		}
	}
}

func TestFrontmatterKeysKeepsDelimitersOutsideEverySpanIncludingCRLF(t *testing.T) {
	source := "---\r\ntitle: a \"b\"\r\n---\r\n\r\nBody\r\n"
	spans := modes.FrontmatterSpans(source, map[string]struct{}{"title": {}})
	cp := []rune(source)
	var got []string
	for _, span := range spans {
		got = append(got, string(cp[span.Start:span.End]))
	}
	if len(got) != 1 || got[0] != "a \"b\"" {
		t.Fatalf("spans = %q, want [`a \"b\"`]", got)
	}
	want := "---\r\ntitle: a “b”\r\n---\r\n\r\nBody\r\n"
	if out := mdOf(t, source, "en-US", []string{"title"}); out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// modes.md 3.7.4: the block is its own text unit, so an unbalanced mark in it cannot pair with one
// in the body. This is the discriminating case -- as one unit those two marks do pair.
func TestFrontmatterKeysBlockIsItsOwnTextUnit(t *testing.T) {
	source := "---\ntitle: He said \"hello\n---\n\nworld\" she said\n"
	if got := mdOf(t, source, "en-US", []string{"title"}); got != source {
		t.Fatalf("marks paired across the block: %q", got)
	}
	oneUnit, err := polytypo.Transform("title: He said \"hello\n\nworld\" she said\n", polytypo.Options{Locale: "en-US"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "title: He said “hello\n\nworld” she said\n"; oneUnit != want {
		t.Fatalf("the control case does not discriminate: %q != %q", oneUnit, want)
	}
}

func TestFrontmatterKeysCannotChangeAByteOutsideTheBlock(t *testing.T) {
	// The block's own length can change, so the body is located in each result rather than at one
	// offset taken from the input.
	body := func(source string) string {
		i := strings.Index(source[3:], "\n---")
		if i < 0 {
			return source
		}
		return source[3+i+len("\n---"):]
	}
	sources := []string{
		fmDoc,
		"---\ntitle: \"x\"\n---\n\nBody - one \"two\" three...\n",
		"---\nsummary: |\n  a - b\n  c - d\n---\n\nBody - e \"f\"...\n",
		"---\ntitle: a\n---\nAbutting body \"x\" - y\n",
	}
	keySets := [][]string{{}, {"title"}, {"title", "slug", "summary", "seo"}}
	for _, source := range sources {
		for _, keys := range keySets {
			if got, want := body(mdOf(t, source, "en-US", keys)), body(mdOf(t, source, "en-US", nil)); got != want {
				t.Fatalf("%q with keys %q: body changed to %q, want %q", source, keys, got, want)
			}
		}
	}
}

func TestFrontmatterKeysIsIdempotentUnderItsOwnOptions(t *testing.T) {
	sources := []string{fmDoc, "---\ntitle: a -- b \"c\"\nx: keep -- me\n---\n\nBody -- \"d\"\n"}
	for _, locale := range []string{"en-US", "de-DE", "fr", "ru"} {
		for _, source := range sources {
			once := mdOf(t, source, locale, []string{"title", "summary"})
			if twice := mdOf(t, once, locale, []string{"title", "summary"}); twice != once {
				t.Fatalf("%s %q: %q -> %q", locale, source, once, twice)
			}
		}
	}
}

func TestFrontmatterKeysIsIgnoredInTheOtherModes(t *testing.T) {
	source := "title: a \"b\"\n"
	for _, opts := range []polytypo.Options{
		{Locale: "en-US", FrontmatterKeys: []string{"title"}},
		{Locale: "en-US", Mode: "html", FrontmatterKeys: []string{"title"}},
		{Locale: "en-US", Mode: "yaml", Keys: []string{"title"}, FrontmatterKeys: []string{"title"}},
	} {
		withOption, err := polytypo.Transform(source, opts)
		if err != nil {
			t.Fatalf("%+v: %v", opts, err)
		}
		bare := opts
		bare.FrontmatterKeys = nil
		without, err := polytypo.Transform(source, bare)
		if err != nil {
			t.Fatalf("%+v: %v", bare, err)
		}
		if withOption != without {
			t.Fatalf("%+v: the option changed a non-markdown mode: %q != %q", opts, withOption, without)
		}
	}
}

func TestFrontmatterKeysIsCheckedAfterTheDialect(t *testing.T) {
	_, err := polytypo.Transform(fmDoc, polytypo.Options{
		Locale:          "en-US",
		Mode:            "markdown",
		Dialect:         "mdx",
		FrontmatterKeys: []string{"title"},
	})
	code, ok := codeOf(err)
	if !ok || code != "POLYTYPO_INVALID_DIALECT" {
		t.Fatalf("code = %q (ok=%v), want POLYTYPO_INVALID_DIALECT", code, ok)
	}
}

func TestFrontmatterKeysAnalyzeReportsBothUnitsInDocumentOrder(t *testing.T) {
	changes, err := polytypo.Analyze(fmDoc, polytypo.Options{
		Locale:          "en-US",
		Mode:            "markdown",
		Dialect:         "commonmark",
		FrontmatterKeys: []string{"title"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) < 2 {
		t.Fatalf("expected changes in both units, got %d", len(changes))
	}
	starts := make([]int, len(changes))
	for i, c := range changes {
		starts[i] = c.Start
	}
	if !sort.IntsAreSorted(starts) {
		t.Fatalf("changes are not in document order: %v", starts)
	}
	blockEnd := strings.Index(fmDoc[3:], "\n---") + 3
	if starts[0] >= blockEnd {
		t.Fatalf("first change at %d is not inside the block (ends at %d)", starts[0], blockEnd)
	}
	for _, c := range changes {
		if c.End > len([]rune(fmDoc)) {
			t.Fatalf("change %+v reaches past the document", c)
		}
	}
}

// modes.md 5, spec 1.7.0: the "markdown" sweep must also carry a template with a frontmatter block
// and FrontmatterKeys naming a key in it. Such a document has TWO text units (3.1), a composition
// no single-unit template reaches: the pipeline runs twice and the two edit sets are merged into
// one emission, so a mistake there shows up as a document that is not a fixed point even though
// each unit is.
func TestBoundedSweepMarkdownFrontmatterTwoUnits(t *testing.T) {
	locales := allLocales(t)
	alphabet := []string{"\"", "'", "-", ":", "#", " ", ".", "a", "---"}
	templates := []struct {
		label string
		of    func(a, b string) string
	}{
		{"plain scalar and body", func(a, b string) string { return "---\nk: " + a + "\n---\n\n" + b + "\n" }},
		{"two scalars", func(a, b string) string { return "---\nk: " + a + "\nj: " + b + "\n---\n\nbody\n" }},
		{"block scalar and body", func(a, b string) string { return "---\nk: |\n  " + a + "\n---\n\nbody " + b + " end\n" }},
	}
	var payloads []string
	var build func(prefix string, depth int)
	build = func(prefix string, depth int) {
		payloads = append(payloads, prefix)
		if depth == 0 {
			return
		}
		for _, token := range alphabet {
			build(prefix+token, depth-1)
		}
	}
	build("", 1)
	var deepPayloads []string
	saved := payloads
	payloads = nil
	build("", 2)
	deepPayloads = payloads
	payloads = saved

	// The swept payload is the BLOCK's content, which is what the second unit is made of; the body
	// only has to be something the rules touch, so it takes a fixed handful. And the two lengths
	// are split across two runs rather than multiplied: markdown parsing under -race costs ~350x
	// what it costs here, so the full cross product of the deeper payload was 13 minutes of CI per
	// test step, testing the same composition over and over. Every locale gets the length-1 sweep;
	// the length-2 payloads run in four locales, one of each dash and quote convention.
	bodies := []string{"", "a", "a - b", `he said "x"`, "a...b", "--- a"}
	deepLocales := []string{"en-US", "de-DE", "fr", "ru"}

	keys := []string{"k", "j"}
	var broken []string
	for _, locale := range locales {
		for _, tpl := range templates {
			for _, a := range payloads {
				for _, b := range bodies {
					source := tpl.of(a, b)
					opts := polytypo.Options{Locale: locale, Mode: "markdown", Dialect: "commonmark", FrontmatterKeys: keys}
					once, err := polytypo.Transform(source, opts)
					if err != nil {
						broken = append(broken, fmt.Sprintf("%s %s: %q errored: %v", locale, tpl.label, source, err))
						continue
					}
					twice, err := polytypo.Transform(once, opts)
					if err != nil || twice != once {
						broken = append(broken, fmt.Sprintf("%s %s: %q -> %q -> %q (err=%v)", locale, tpl.label, source, once, twice, err))
					}
					if len(broken) >= 10 {
						break
					}
				}
			}
		}
	}
	for _, locale := range deepLocales {
		for _, tpl := range templates {
			for _, a := range deepPayloads {
				source := tpl.of(a, `he said "x"`)
				opts := polytypo.Options{Locale: locale, Mode: "markdown", Dialect: "commonmark", FrontmatterKeys: keys}
				once, err := polytypo.Transform(source, opts)
				if err != nil {
					broken = append(broken, fmt.Sprintf("%s %s: %q errored: %v", locale, tpl.label, source, err))
					continue
				}
				twice, err := polytypo.Transform(once, opts)
				if err != nil || twice != once {
					broken = append(broken, fmt.Sprintf("%s %s: %q -> %q -> %q (err=%v)", locale, tpl.label, source, once, twice, err))
				}
				if len(broken) >= 10 {
					break
				}
			}
		}
	}
	if len(broken) > 0 {
		t.Fatalf("non-idempotent frontmatter documents:\n%s", strings.Join(broken, "\n"))
	}
}
