package modes

import (
	"strings"
	"testing"
)

// spec/rules/modes.md 3.8 -- span selection only. The scan is specified rather than delegated
// (3.8.1), so a test here is one of the few things standing between five hand-written scanners
// and five different answers.

func keySet(keys ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		set[k] = struct{}{}
	}
	return set
}

func proseKeys() map[string]struct{} {
	return keySet("description", "summary", "title", "a", "b", "c", "k", "n", "inner", "use")
}

func spanText(source string, keys map[string]struct{}) []string {
	cp := []rune(source)
	out := []string{}
	for _, s := range YAMLSpans(cp, keys) {
		out = append(out, string(cp[s.Start:s.End]))
	}
	return out
}

func eq(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestYAMLSpansThreeScalarForms(t *testing.T) {
	source := "a: one two\nb: \"three four\"\nc: |\n  five six\n"
	want := []string{"one two", "three four", "five six"}
	if got := spanText(source, proseKeys()); !eq(got, want) {
		t.Fatalf("spans = %q, want %q", got, want)
	}
}

func TestYAMLKeysAreMatchedExactly(t *testing.T) {
	cases := []struct {
		name   string
		source string
		keys   map[string]struct{}
		want   []string
	}{
		{"no case folding", "Description: one two\n", keySet("description"), nil},
		{"exact match", "Description: one two\n", keySet("Description"), []string{"one two"}},
		{"trailing spaces before the colon", "description  : one two\n", keySet("description"), []string{"one two"}},
		{"a quoted key never matches", "\"description\": one two\n", keySet("description"), nil},
		{"a key containing ! never matches", "a!b: one two\n", keySet("a!b"), nil},
		{"a colon not followed by a space is a key character", "a:b: one two\n", keySet("a:b"), []string{"one two"}},
		{"and that key is not `a`", "a:b: one two\n", keySet("a"), nil},
		{"an unlisted key yields nothing", "run: one two\n", keySet("description"), nil},
		{"the same name at any depth", "description: one\nn:\n  description: two\n", keySet("description"), []string{"one", "two"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := spanText(c.source, c.keys)
			if len(c.want) == 0 && len(got) == 0 {
				return
			}
			if !eq(got, c.want) {
				t.Fatalf("spans = %q, want %q", got, c.want)
			}
		})
	}
}

func TestYAMLSkipsByDefault(t *testing.T) {
	// 3.8.3: a construct the scan does not recognise yields no spans.
	cases := map[string]string{
		"a bare sequence item":                  "- Some prose here...\n",
		"a flow sequence":                       "a: [one two..., three]\n",
		"a flow mapping":                        "a: {b: one two...}\n",
		"an anchor":                             "a: &anchor one two...\n",
		"an alias":                              "a: *anchor\n",
		"a tag":                                 "a: !!str one two...\n",
		"a double-quoted scalar with an escape": "a: \"one \\\"two\\\"... three\"\n",
		"a single-quoted scalar with an escape": "a: 'it''s one two...'\n",
		"a tab anywhere on the line":            "a:\tone two...\n",
		"a compact nested sequence":             "a: - one two...\n",
		"a compact nested mapping":              "a: one two .:\n",
		"an unterminated quoted scalar":         "a: \"one two...\n",
		"a value that is only a comment":        "a: # one two...\n",
		"a multi-line quoted scalar":            "a: \"hello\n  b: some prose \"word\" here\"\n",
		"a multi-line flow mapping":             "a: {\n  b: hello world,\n  c: x\n}\n",
		"a multi-line plain scalar":             "a: one two...\n  b: three four...\n",
		"an inline value's continuation line":   "a: inline value\n  b: one two\n",
		"a document marker introducing a node":  "--- a: one two\n",
		"a block with an ambiguous indicator":   "a: |4\n  one two\n",
		"a block with a tab on a content line":  "a: |\n  one two\n  three\tfour\n",
		"a block dedented inside itself":        "a: |\n    deep one two\n  shallow three\n",
		"a block with an unrecognised header":   "a: |x\n  one two\n",
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			if got := spanText(source, proseKeys()); len(got) != 0 {
				t.Fatalf("spans = %q, want none", got)
			}
		})
	}
}

func TestYAMLBlockScalarShape(t *testing.T) {
	for _, header := range []string{"|", "|-", "|+", ">", ">-", ">+"} {
		source := "a: " + header + "\n  one two...\n\n\nz: 1\n"
		want := []string{"one two..."}
		if got := spanText(source, proseKeys()); !eq(got, want) {
			t.Fatalf("%s: spans = %q, want %q", header, got, want)
		}
	}
	if got := spanText("a: |\n  one two\n    three four\n", proseKeys()); !eq(got, []string{"one two", "  three four"}) {
		t.Fatalf("extra indentation belongs inside the span, got %q", got)
	}
	if got := spanText("a: |2\n   one two\n", proseKeys()); !eq(got, []string{" one two"}) {
		t.Fatalf("explicit indicator: got %q", got)
	}
	if got := spanText("a: | # note\n  one two\n", proseKeys()); !eq(got, []string{"one two"}) {
		t.Fatalf("header comment: got %q", got)
	}
}

func TestYAMLCRLFYieldsTheSameSpansAsLF(t *testing.T) {
	// 3.8.4: a U+000D immediately before the U+000A is a terminator, not part of the line.
	crlf := spanText("a: |\r\n  one two...\r\n", proseKeys())
	lf := spanText("a: |\n  one two...\n", proseKeys())
	if !eq(crlf, lf) {
		t.Fatalf("CRLF spans %q != LF spans %q", crlf, lf)
	}
	if len(crlf) != 1 || strings.ContainsRune(crlf[0], '\r') {
		t.Fatalf("a carriage return leaked into a span: %q", crlf)
	}
}

func TestYAMLPlainScalarSplitsAtColonAndHash(t *testing.T) {
	// 3.8.6: both are opaque one-code-point units inside a plain scalar.
	if got := spanText("a: one:two three\n", proseKeys()); !eq(got, []string{"one", "two three"}) {
		t.Fatalf("colon split: got %q", got)
	}
	if got := spanText("a: one#two three\n", proseKeys()); !eq(got, []string{"one", "two three"}) {
		t.Fatalf("hash split: got %q", got)
	}
	if got := spanText("a: some prose # note: here\n", proseKeys()); !eq(got, []string{"some prose"}) {
		t.Fatalf("the comment is stripped before the colon test: got %q", got)
	}
	if got := spanText("a: \"Chapter 1: the beginning\"\n", proseKeys()); !eq(got, []string{"Chapter 1: the beginning"}) {
		t.Fatalf("no colon test applies to a quoted scalar: got %q", got)
	}
}
