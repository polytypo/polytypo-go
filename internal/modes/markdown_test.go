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
