package modes

import "testing"

func spansToText(source string, spans []Span, cp []rune) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = string(cp[s.Start:s.End])
	}
	return out
}

func TestHTMLSmoke(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"plain text", "hello world", []string{"hello world"}},
		{"skips code", "a <code>x-y</code> b", []string{"a ", " b"}},
		{"case insensitive skip", "a <CODE>x</CoDe> b", []string{"a ", " b"}},
		{"nested skip whole", "a <pre><em>x</em></pre> b", []string{"a ", " b"}},
		{"entity opaque", "R&amp;D and 3-5", []string{"R", "D and 3-5"}},
		{"bare amp stays", `Tom & Jerry's "book"`, []string{`Tom & Jerry's "book"`}},
		{"numeric entity", "a&#8217;b", []string{"a", "b"}},
		{"hex entity", "a&#x2019;b", []string{"a", "b"}},
		{"void element no stack effect", "a<br>b", []string{"a", "b"}},
		{"self closing skip element", "a<script/>b", []string{"a", "b"}},
		{"unicode text", "héllo — wörld", []string{"héllo — wörld"}},
		{"unicode inside skip", "a <code>héllo</code> b", []string{"a ", " b"}},
		{"unknown element processable", "<my-callout>hi</my-callout>", []string{"hi"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cp := []rune(c.in)
			spans, err := HTMLSpans(c.in)
			if err != nil {
				t.Fatal(err)
			}
			got := spansToText(c.in, spans, cp)
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
