package modes

import "testing"

func TestMarkdownSmoke(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"prose", "Is this \"polytypo\"?\n", []string{"Is this \"polytypo\"?"}},
		{"inline code excluded", "Use `\"raw\"` here but not \"this\" here.\n", []string{"Use ", " here but not \"this\" here."}},
		{"fenced code excluded", "```js\nconst s = \"raw\";\n```\n\nBut \"this\" converts.\n", []string{"But \"this\" converts."}},
		{"link text and destination", "[te...xt](http://a...b \"ti...tle\")\n", []string{"te...xt"}},
		{"autolink excluded", "See <https://example.com/a...b> and also a...b here.\n", []string{"See ", " and also a...b here."}},
		{"frontmatter excluded", "---\ntitle: \"Une note\"\n---\n\nBody has \"quotes\" here.\n", []string{"Body has \"quotes\" here."}},
		// <b> is not in the HTML skip list (only code/pre/kbd/samp/var/script/style/textarea/
		// svg/math are), so its content is processable prose like any other inline raw HTML —
		// this only proves the html-tag-stack correctly treats <b>/</b> as ordinary boundaries,
		// not that <b> content is skipped.
		{"raw html", "<div class=\"x\">\n  Body \"text\" inside.\n</div>\n\nAlso <b>\"kept\"</b> outside \"here\".\n",
			[]string{"\n  Body \"text\" inside.\n", "\n", "Also ", "\"kept\"", " outside \"here\"."}},
		{"indented code excluded", "paragraph\n\n    indented code\n", []string{"paragraph"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cp := []rune(c.in)
			spans, err := MarkdownSpans(c.in)
			if err != nil {
				t.Fatal(err)
			}
			// The real pipeline always normalizes (coalesces adjacent, sorts) before use; a
			// mode extractor is allowed to hand back multiple immediately-adjacent spans, e.g.
			// when goldmark's Linkify inline-parser trigger splits a Text run at a position
			// where no link ultimately matched.
			spans, err = NormalizeSpans(spans)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(spans))
			for i, s := range spans {
				got[i] = string(cp[s.Start:s.End])
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %d spans %q, want %d spans %q", len(got), got, len(c.want), c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("span %d: got %q want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// spec/rules/modes.md 3.7.3a (spec 1.8.0) states an outcome as well as a mechanism: no span may lie
// inside the block, whatever the parser did with the masked text. goldmark needs no clip on top of
// the mask to satisfy it, which is why MarkdownSpans carries none — but that is a measurement about
// goldmark, not something the spec guarantees, so it is asserted here rather than assumed. The
// shapes are the ones where a parser is most likely to make something of an all-U+0020 region: a
// block that is the whole document, one abutting the body, an empty one, and one whose masked text
// would have opened a construct.
func TestNoSpanLiesInsideAMaskedFrontmatterBlock(t *testing.T) {
	sources := []string{
		"---\ntitle: \"x\"\n---\n\nBody \"q\".\n",
		"---\ntitle: \"x\"\n---\nAbutting body \"q\".\n",
		"---\ntitle: \"x\"\n---\n",
		"---\n---\n\nBody \"q\".\n",
		"---\nx: |\n  ```\n---\n\nBody \"q\".\n",
		"--- \t\nx: <div>\n--- \n\nBody \"q\".\n",
		"\uFEFF---\ntitle: \"x\"\n---\n\nBody \"q\".\n",
		"---\r\ntitle: \"x\"\r\n---\r\n\r\nBody \"q\".\r\n",
		"---\rtitle: \"x\"\r---\r\rBody \"q\".\r",
		"+++ \ntitle = \"x\"\n+++\n\nBody \"q\".\n",
	}
	for _, source := range sources {
		block, ok := detectFrontmatter(source)
		if !ok {
			t.Fatalf("%q: no block located, so the case does not test what it claims", source)
		}
		spans, err := MarkdownSpans(source)
		if err != nil {
			t.Fatalf("%q: %v", source, err)
		}
		limit := NewByteOffsets([]rune(source)).CodePointOf(block.end)
		for _, span := range spans {
			if span.Start < limit {
				t.Errorf("%q: span %d..%d starts inside the masked block (ends at code point %d)",
					source, span.Start, span.End, limit)
			}
		}
	}
}
