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

// spec/rules/modes.md 3.7.3a -- where the block begins and ends (spec 1.8.0). The fixtures pin one
// trailing space on each fence and one trailing tab on the opener; these are the neighbouring
// shapes an author's editor produces, which a fixture would only restate. Every positive row
// carries a body that does change, so a row cannot pass by nothing running at all.
func TestFrontmatterFenceAcceptsTrailingSpacesAndTabs(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "several trailing spaces on the opener",
			source: "---   \ntitle: \"Une note\"\n---\n\nBody has \"quotes\" here.\n",
			want:   "---   \ntitle: \"Une note\"\n---\n\nBody has “quotes” here.\n",
		},
		{
			name:   "tabs on the opener",
			source: "---\t\t\ntitle: \"Une note\"\n---\n\nBody has \"quotes\" here.\n",
			want:   "---\t\t\ntitle: \"Une note\"\n---\n\nBody has “quotes” here.\n",
		},
		{
			name:   "spaces and tabs mixed on the opener",
			source: "--- \t \ntitle: \"Une note\"\n---\n\nBody has \"quotes\" here.\n",
			want:   "--- \t \ntitle: \"Une note\"\n---\n\nBody has “quotes” here.\n",
		},
		{
			name:   "trailing whitespace on the closer only",
			source: "---\ntitle: \"Une note\"\n--- \t\n\nBody has \"quotes\" here.\n",
			want:   "---\ntitle: \"Une note\"\n--- \t\n\nBody has “quotes” here.\n",
		},
		{
			name:   "a CRLF document whose opening fence carries a trailing space",
			source: "--- \r\ntitle: \"Une note\"\r\n---\r\n\r\nBody has \"quotes\" here.\r\n",
			want:   "--- \r\ntitle: \"Une note\"\r\n---\r\n\r\nBody has “quotes” here.\r\n",
		},
		{
			name:   "a TOML fence carrying a trailing space",
			source: "+++ \ntitle = \"Une note\"\n+++\n\nBody has \"quotes\" here.\n",
			want:   "+++ \ntitle = \"Une note\"\n+++\n\nBody has “quotes” here.\n",
		},
		{
			// 3.7.3a step 2: the wider reading widens the whitespace, not the syntax.
			name:   "a word after the opening delimiter is not a block",
			source: "--- yaml\ntitle: \"Une note\"\n---\n\nBody has \"quotes\" here.\n",
			want:   "--- yaml\ntitle: “Une note”\n---\n\nBody has “quotes” here.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mdOf(t, tc.source, "en-US", nil); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The widened fence and FrontmatterKeys read the same scan (modes.md 3.7.3a, 3.7.4), so the option
// reaches a block a trailing space would have hidden -- and the fence's own whitespace, including
// the U+000D of a CRLF document, stays outside the content range and comes back byte for byte.
func TestFrontmatterKeysReachesABlockWithAWidenedFence(t *testing.T) {
	source := "--- \r\ntitle: a \"b\"\r\n--- \t\r\n\r\nBody \"c\"\r\n"
	spans := modes.FrontmatterSpans(source, map[string]struct{}{"title": {}})
	cp := []rune(source)
	var got []string
	for _, span := range spans {
		got = append(got, string(cp[span.Start:span.End]))
	}
	if len(got) != 1 || got[0] != "a \"b\"" {
		t.Fatalf("spans = %q, want [`a \"b\"`]", got)
	}
	want := "--- \r\ntitle: a “b”\r\n--- \t\r\n\r\nBody “c”\r\n"
	if out := mdOf(t, source, "en-US", []string{"title"}); out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// spec/rules/modes.md 3.7.3a: the source handed to the parser is the document with the block
// masked to U+0020, so nothing inside it can form or close a construct in the body. The fixtures
// pin the fenced-code case in YAML; these are the other shapes a metadata value can carry into the
// parser, and the conversion of the body is what proves the mask rather than a range suppression.
func TestFrontmatterIsMaskedBeforeTheParserSeesIt(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// The discriminating shape: with the block unmasked the parser paired this fence with
			// nothing and swallowed the rest of the document into a code block, so the body was
			// missed entirely. Measured on the shipped 1.7.0 locator.
			name:   "an unterminated fence in a metadata value does not swallow the body",
			source: "---\nx: |\n  ```\n---\n\nBody \"q\" here.\n",
			want:   "---\nx: |\n  ```\n---\n\nBody \u201cq\u201d here.\n",
		},
		{
			name:   "a fence inside a TOML value does not pair with the body's",
			source: "+++ \nx = \"\"\"\n```\n\"\"\"\n+++\n\n```\ncode \"q\"\n```\n\nBody \"q\".\n",
			want:   "+++ \nx = \"\"\"\n```\n\"\"\"\n+++\n\n```\ncode \"q\"\n```\n\nBody \u201cq\u201d.\n",
		},
		{
			// Not a shape the old locator got wrong -- a guard on the mask itself. maskFrontmatter
			// is byte-wise where 3.7.3a is code-point-wise, so a multi-byte code point becomes that
			// many U+0020 and the body's offsets are unmoved; a code-point-wise mask would shift
			// every span after the block.
			name:   "multi-byte code points in the block leave the body's offsets where they were",
			source: "---\ntitle: \u00dcn\u00ef \u2014 \u00e7\u00e0 \"x\"\n---\n\nBody \"q\" here.\n",
			want:   "---\ntitle: \u00dcn\u00ef \u2014 \u00e7\u00e0 \"x\"\n---\n\nBody \u201cq\u201d here.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mdOf(t, tc.source, "en-US", nil); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// 3.7.3a step 1: a single leading U+FEFF is stepped over and is not part of the document for the
// scan. The fixtures pin the two documents that gain a block by it; these are the ones that must
// not.
func TestFrontmatterStepsOverASingleByteOrderMarkOnly(t *testing.T) {
	bom := string(rune(0xFEFF))
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a mark does not widen step 2",
			source: bom + "--- yaml\ntitle: a \"b\"\n---\n\nBody \"c\"\n",
			want:   bom + "--- yaml\ntitle: a “b”\n---\n\nBody “c”\n",
		},
		{
			name:   "a second mark is content, so there is no block",
			source: bom + bom + "---\ntitle: a \"b\"\n---\n\nBody \"c\"\n",
			want:   bom + bom + "---\ntitle: a “b”\n---\n\nBody “c”\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mdOf(t, tc.source, "en-US", nil); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The mark is stepped over by the locator, so the content range -- and every span FrontmatterKeys
// takes from it -- is still measured in the real document. A block located three bytes early would
// shift every span by three.
func TestFrontmatterKeysLocatesTheContentPastAByteOrderMark(t *testing.T) {
	source := string(rune(0xFEFF)) + "--- \ntitle: a \"b\"\n---\n\nBody \"c\"\n"
	spans := modes.FrontmatterSpans(source, map[string]struct{}{"title": {}})
	cp := []rune(source)
	var got []string
	for _, span := range spans {
		got = append(got, string(cp[span.Start:span.End]))
	}
	if len(got) != 1 || got[0] != "a \"b\"" {
		t.Fatalf("spans = %q, want [`a \"b\"`]", got)
	}
	want := string(rune(0xFEFF)) + "--- \ntitle: a “b”\n---\n\nBody “c”\n"
	if out := mdOf(t, source, "en-US", []string{"title"}); out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// 3.7.3a step 5: the locator ends a line the way CommonMark does. The fixtures pin a lone-CR
// document and a final U+000D with no newline; this is the mixed document, where one line ends
// CRLF and the next LF, and the CR of a CRLF closer is still not part of the delimiter line.
func TestFrontmatterLineModelAcceptsMixedTerminators(t *testing.T) {
	source := "---\r\ntitle: a \"b\"\n--- \r\n\r\nBody \"c\"\n"
	want := "---\r\ntitle: a \"b\"\n--- \r\n\r\nBody “c”\n"
	if got := mdOf(t, source, "en-US", nil); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// spec/rules/modes.md 3.7.4 (spec 1.8.0): a content LINE carrying a U+000D not followed by U+000A
// yields no spans, the sibling of 3.8.4 step 1's tab rule. The fixtures pin the two documents
// written wholly with lone U+000D and the one whose `title` carries a stray one; these are the
// window question and the controls that stop the rule from reading as "any U+000D declines" or "a
// U+000D anywhere declines the block".
func TestFrontmatterKeysDeclinesTheLinesCarryingALoneCarriageReturn(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			// The window row. `slug`'s lone U+000D sits where 3.8.4's splitter would read a
			// terminator, so a test that stops at the end of the line proper sees a clean line and
			// converts `d`. The clean `title` line above it still converts, which is the per-line
			// grain: one stray return costs one value.
			name:   "a line whose terminator is a lone U+000D is the only one declined",
			source: "---\ntitle: a \"b\"\nslug: c \"d\"\r---\n\nBody \"e\"\n",
			want:   "---\ntitle: a \u201cb\u201d\nslug: c \"d\"\r---\n\nBody \u201ce\u201d\n",
		},
		{
			// A U+000D mid-content folds two mapping lines into one 3.8.4 line, so the whole block
			// is that one declined line -- the same reason a wholly lone-U+000D document loses
			// everything. 3.8.4 would also refuse this shape on its own; the rule is what
			// guarantees it rather than the scan happening to refuse that particular value.
			name:   "a U+000D between two mapping lines declines both",
			source: "---\ntitle: a \"b\"\rslug: c \"d\"\n---\n\nBody \"e\"\n",
			want:   "---\ntitle: a \"b\"\rslug: c \"d\"\n---\n\nBody \u201ce\u201d\n",
		},
		{
			// Control: every U+000D here is followed by U+000A, so nothing is declined. Without
			// this row, declining CRLF too would satisfy the rows above.
			name:   "CRLF content is not declined",
			source: "---\r\ntitle: a \"b\"\r\nslug: c \"d\"\r\n---\r\n\r\nBody \"e\"\r\n",
			want:   "---\r\ntitle: a \u201cb\u201d\r\nslug: c \u201cd\u201d\r\n---\r\n\r\nBody \u201ce\u201d\r\n",
		},
		{
			// Control: the test is on the content range, not the document.
			name:   "a lone U+000D in the body leaves the block alone",
			source: "---\ntitle: a \"b\"\n---\n\nBody \"c\"\rmore \"d\"\n",
			want:   "---\ntitle: a \u201cb\u201d\n---\n\nBody \u201cc\u201d\rmore \u201cd\u201d\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mdOf(t, tc.source, "en-US", []string{"title", "slug"}); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// The parser's input gets the line terminators goldmark understands (see
// normalizeParserLineTerminators). goldmark ends a line at U+000A and nowhere else, so in a
// document written with lone U+000D endings it sees ONE line and 3.7.3a's mask is not blank lines
// but leading whitespace on the body's line — four U+0020 of which make the whole document an
// indented code block and the body unreachable.
//
// The conformance fixture pins the U+FEFF composition. These are the rest of the class, which is
// what shows the mark was never the cause: it is the third and fourth U+0020 of the masked fence,
// and a fence written "--- " reaches four with no mark at all. Every row but the last was red
// before the parser's input was normalized.
func TestLoneCarriageReturnBodyIsReachableWhateverTheFenceMasksTo(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "a trailing space on the fence reaches four U+0020 with no mark",
			source: "--- \rtitle: a \"b\"\r---\r\rBody \"q\".\r",
			want:   "--- \rtitle: a \"b\"\r---\r\rBody “q”.\r",
		},
		{
			name:   "a trailing tab masks to U+0020 and does the same",
			source: "---\t\rtitle: a \"b\"\r---\r\rBody \"q\".\r",
			want:   "---\t\rtitle: a \"b\"\r---\r\rBody “q”.\r",
		},
		{
			name:   "a mark and a trailing space together",
			source: "\uFEFF--- \rtitle: a \"b\"\r---\r\rBody \"q\".\r",
			want:   "\uFEFF--- \rtitle: a \"b\"\r---\r\rBody “q”.\r",
		},
		{
			// Multi-byte body content: the normalization is one byte for one byte, so the offsets
			// goldmark reports still index the original source and the span lands on the quotes
			// rather than beside them.
			name:   "multi-byte content in the body keeps its offsets",
			source: "\uFEFF---\rtitle: a \"b\"\r---\r\rCafé au lait \"q\".\r",
			want:   "\uFEFF---\rtitle: a \"b\"\r---\r\rCafé au lait “q”.\r",
		},
		{
			// Control: three U+0020 is under the indented-code threshold, so this document was
			// correct before the fix too. It is what made the defect look like a U+FEFF problem.
			name:   "a bare fence stays under the threshold",
			source: "---\rtitle: a \"b\"\r---\r\rBody \"q\".\r",
			want:   "---\rtitle: a \"b\"\r---\r\rBody “q”.\r",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mdOf(t, tc.source, "en-US", nil); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
