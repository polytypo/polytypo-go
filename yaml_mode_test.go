package polytypo_test

import (
	"strings"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
	"github.com/polytypo/polytypo-go/internal/spec"
)

// spec/rules/modes.md 3.8 -- the mode's behaviour through the public API, and 3.4's character
// clause, which the new mode is how we found but which is live in shipped html and markdown.

func yamlOf(t *testing.T, source, locale string, keys []string) string {
	t.Helper()
	out, err := polytypo.Transform(source, polytypo.Options{Locale: locale, Mode: "yaml", Keys: keys})
	if err != nil {
		t.Fatalf("Transform(%q, %s, keys=%q): %v", source, locale, keys, err)
	}
	return out
}

func TestYAMLKeysIsRequiredWithNoDefault(t *testing.T) {
	cases := []struct {
		name string
		opts polytypo.Options
	}{
		{"absent", polytypo.Options{Locale: "en-US", Mode: "yaml"}},
		{"explicitly nil", polytypo.Options{Locale: "en-US", Mode: "yaml", Keys: nil}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := polytypo.Transform("a: one two\n", c.opts)
			code, ok := codeOf(err)
			if !ok || code != string(polytypo.CodeInvalidOption) {
				t.Fatalf("err = %v, want CodeInvalidOption", err)
			}
		})
	}
}

func TestYAMLEmptyKeysIsLegalAndProcessesNothing(t *testing.T) {
	source := "description: one...two\n"
	if got := yamlOf(t, source, "en-US", []string{}); got != source {
		t.Fatalf("got %q, want it unchanged", got)
	}
}

func TestYAMLProcessesListedKeysOnly(t *testing.T) {
	source := "description: one...two\nrun: three...four\n"
	want := "description: one…two\nrun: three...four\n"
	if got := yamlOf(t, source, "en-US", []string{"description"}); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestYAMLNeverThrowsOnInputAndReturnsUnrecognisedBytes(t *testing.T) {
	source := "{{ not yaml at all ... }}\n\t\tmixed\tindentation\n"
	if got := yamlOf(t, source, "en-US", []string{"description"}); got != source {
		t.Fatalf("got %q, want it unchanged", got)
	}
}

func TestYAMLChompingIndicatorsAreIdentical(t *testing.T) {
	for _, header := range []string{"|", "|-", "|+", ">", ">-", ">+"} {
		source := "a: " + header + "\n  one two...\n\n\nz: 1\n"
		want := "a: " + header + "\n  one two…\n\n\nz: 1\n"
		if got := yamlOf(t, source, "en-US", []string{"a"}); got != want {
			t.Fatalf("%s: got %q, want %q", header, got, want)
		}
	}
}

func TestYAMLQuotesPairAcrossBlockScalarLines(t *testing.T) {
	source := "a: |\n  He said \"hi\"\n  and left\n"
	want := "a: |\n  He said “hi”\n  and left\n"
	if got := yamlOf(t, source, "en-US", []string{"a"}); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestYAMLColonAndHashSplitProtectTheDocument(t *testing.T) {
	// 3.8.6 with 3.4: a growing replacement is discarded at the extremity, a contraction is not.
	for _, source := range []string{"k: a:--b\n", "k: a--#b\n", "k: a:---b\n", "k: a---#b\n"} {
		if got := yamlOf(t, source, "de-DE", []string{"k"}); got != source {
			t.Fatalf("de-DE %q: got %q, want it unchanged", source, got)
		}
	}
	if got := yamlOf(t, "k: a--#b\n", "en-US", []string{"k"}); got != "k: a—#b\n" {
		t.Fatalf("en-US contraction: got %q", got)
	}
}

func TestYAMLSpanPartitionIsStable(t *testing.T) {
	// modes.md 5 item 2: the predicate that selects a span must not read anything a rule can
	// change. It reads the key, so this is a fixed point.
	source := "a: ${{ steps.pin.outputs.sha }}\n"
	once := yamlOf(t, source, "en-US", []string{"a"})
	if twice := yamlOf(t, once, "en-US", []string{"a"}); twice != once {
		t.Fatalf("not idempotent: %q -> %q -> %q", source, once, twice)
	}
}

// spacedLocales reads the set from the locale data rather than naming it here: a hardcoded list
// rots silently as locales are added, and it would also admit "el", whose dash.parenthetical is
// "none" and which therefore emits nothing at all.
func spacedLocales(t *testing.T) []string {
	t.Helper()
	reg, err := spec.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, id := range reg.Locales {
		data, err := spec.LoadLocale(id)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(data.Dash.Parenthetical, "-spaced") {
			out = append(out, id)
		}
	}
	return out
}

func TestEdgeGrowthCharacterClause(t *testing.T) {
	// modes.md 3.4, spec 1.3.0. `r > d` is not the rule its own normative sentence states: it
	// misses r == d. dashes P3 admits a run of three, so `---` -> ` – ` is 3 -> 3 and lands
	// U+0020 on both extremities while the length test sees nothing. In html, shipped at v1.0.0,
	// that produced an element beginning and ending with a space it never held.
	for _, locale := range spacedLocales(t) {
		out, err := polytypo.Transform("a<em>---</em>b", polytypo.Options{Locale: locale, Mode: "html"})
		if err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
		if out != "a<em>---</em>b" {
			t.Fatalf("%s: got %q, want it unchanged", locale, out)
		}
	}

	interior, _ := polytypo.Transform("<p>a---b</p>", polytypo.Options{Locale: "de-DE", Mode: "html"})
	if interior != "<p>a – b</p>" {
		t.Fatalf("interior to a span the edit still applies, got %q", interior)
	}

	tight, _ := polytypo.Transform("a<em>---</em>b", polytypo.Options{Locale: "en-US", Mode: "html"})
	if tight != "a<em>—</em>b" {
		t.Fatalf("an em-tight locale emits no U+0020 and still converts, got %q", tight)
	}

	spanning, _ := polytypo.Transform("a<em>x --- y</em>b", polytypo.Options{Locale: "de-DE", Mode: "html"})
	if spanning != "a<em>x – y</em>b" {
		t.Fatalf("a U+0020 replacing a U+0020 changes no edge, got %q", spanning)
	}
}
