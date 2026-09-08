// transform(transform(x)) == transform(x) is a release blocker, not a bug report
// (docs/ARCHITECTURE.md section 6.3, PLAN.md 3.4). Property-based over a biased alphabet (uniform
// random Unicode almost never produces the adjacent-quote-mark shapes that actually break a
// pipeline), plus bounded exhaustive sweeps, which a defect at this size cannot hide from.
// Mirrors the JS reference implementation's tests/engine/idempotency.test.ts and the Python
// port's tests/engine/test_idempotency.py.
package polytypo_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"pgregory.net/rapid"

	polytypo "github.com/polytypo/polytypo-go"
	"github.com/polytypo/polytypo-go/internal/engine"
	"github.com/polytypo/polytypo-go/internal/spec"
)

func allLocales(t interface{ Fatal(...any) }) []string {
	reg, err := spec.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	locales := make([]string, len(reg.Locales))
	copy(locales, reg.Locales)
	return locales
}

// hotChars are the characters every rule reads: quote marks, strokes, spacing, digits, brackets,
// stops. Uniform random Unicode almost never produces the adjacent-quote-mark shapes that
// actually break a pipeline (ARCHITECTURE.md 6.3: "a uniform random generator ran green over this
// pipeline while an enumeration found the first counterexample in milliseconds").
var hotChars = []rune{
	'"', '\'', '“', '”', '‘', '’', '„', '‚', '«', '»', '‹', '›',
	'-', '‐', '‑', '–', '—', ' ', ' ', ' ', '\n',
	'.', ',', ':', ';', '!', '?', '…', '(', ')', '[', ']',
	'0', '1', '9', 'a', 'B', 'x', 'é', 'и', 'k', 'm', '%', '§', '№', 'σ', '·',
}

func TestIdempotentOverHotAlphabet(t *testing.T) {
	locales := allLocales(t)
	rapid.Check(t, func(t *rapid.T) {
		locale := rapid.SampledFrom(locales).Draw(t, "locale")
		n := rapid.IntRange(0, 24).Draw(t, "n")
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteRune(rapid.SampledFrom(hotChars).Draw(t, "ch"))
		}
		text := b.String()

		once, err := polytypo.Transform(text, polytypo.Options{Locale: locale})
		if err != nil {
			t.Fatalf("transform(%q): %v", text, err)
		}
		twice, err := polytypo.Transform(once, polytypo.Options{Locale: locale})
		if err != nil {
			t.Fatalf("transform(transform(%q)): %v", text, err)
		}
		if twice != once {
			t.Fatalf("not idempotent for locale %s: input %q -> %q -> %q", locale, text, once, twice)
		}
	})
}

func TestIdempotentOverArbitraryUnicode(t *testing.T) {
	locales := allLocales(t)
	rapid.Check(t, func(t *rapid.T) {
		locale := rapid.SampledFrom(locales).Draw(t, "locale")
		text := rapid.StringN(0, 100, -1).Draw(t, "text")

		once, err := polytypo.Transform(text, polytypo.Options{Locale: locale})
		if err != nil {
			t.Fatalf("transform(%q): %v", text, err)
		}
		twice, err := polytypo.Transform(once, polytypo.Options{Locale: locale})
		if err != nil {
			t.Fatalf("transform(transform(%q)): %v", text, err)
		}
		if twice != once {
			t.Fatalf("not idempotent for locale %s: input %q -> %q -> %q", locale, text, once, twice)
		}
	})
}

func TestAllRulesDisabledIsANoOp(t *testing.T) {
	locales := allLocales(t)
	order, err := engine.RuleOrder()
	if err != nil {
		t.Fatal(err)
	}
	allOff := make(map[string]bool, len(order))
	for _, id := range order {
		allOff[id] = false
	}

	rapid.Check(t, func(t *rapid.T) {
		locale := rapid.SampledFrom(locales).Draw(t, "locale")
		text := rapid.StringN(0, 100, -1).Draw(t, "text")

		out, err := polytypo.Transform(text, polytypo.Options{Locale: locale, Rules: allOff})
		if err != nil {
			t.Fatalf("transform: %v", err)
		}
		if out != text {
			t.Fatalf("all rules disabled changed input: %q -> %q", text, out)
		}
	})
}

// bestBoundedStrings yields every string, including the empty one, over alphabet up to maxLength
// code points long -- both the input characters and the ones the rules produce, since a pass over
// its own output is what idempotency actually asserts.
func bestBoundedStrings(alphabet []rune, maxLength int, yield func(string)) {
	frontier := []string{""}
	yield("")
	for i := 0; i < maxLength; i++ {
		var next []string
		for _, prefix := range frontier {
			for _, ch := range alphabet {
				candidate := prefix + string(ch)
				next = append(next, candidate)
				yield(candidate)
			}
		}
		frontier = next
	}
}

func TestBoundedExhaustiveSweepEveryLocale(t *testing.T) {
	locales := allLocales(t)
	alphabet := []rune{'"', '\'', '-', ' ', '.', '1', 'a', '«', '–', '”'}
	var broken []string
	for _, locale := range locales {
		bestBoundedStrings(alphabet, 4, func(text string) {
			if len(broken) >= 10 {
				return
			}
			once, err := polytypo.Transform(text, polytypo.Options{Locale: locale})
			if err != nil {
				broken = append(broken, fmt.Sprintf("%s: %q errored: %v", locale, text, err))
				return
			}
			twice, err := polytypo.Transform(once, polytypo.Options{Locale: locale})
			if err != nil || twice != once {
				broken = append(broken, fmt.Sprintf("%s: %q -> %q -> %q (err=%v)", locale, text, once, twice, err))
			}
		})
	}
	if len(broken) > 0 {
		t.Fatalf("non-idempotent cases found:\n%s", strings.Join(broken, "\n"))
	}
}

func TestMixedKindStraightMarksAreIdempotent(t *testing.T) {
	// A straight mark of one kind stranded inside a span quoted with the other kind
	// ("a 'b" c') is the shape that broke quotes' first repair attempt; this discriminates.
	locales := allLocales(t)
	alphabet := []rune{'"', '\'', 'a', ' ', '.'}
	var broken []string
	for _, locale := range locales {
		bestBoundedStrings(alphabet, 6, func(text string) {
			if len(broken) >= 10 {
				return
			}
			once, err := polytypo.Transform(text, polytypo.Options{Locale: locale})
			if err != nil {
				broken = append(broken, fmt.Sprintf("%s: %q errored: %v", locale, text, err))
				return
			}
			twice, err := polytypo.Transform(once, polytypo.Options{Locale: locale})
			if err != nil || twice != once {
				broken = append(broken, fmt.Sprintf("%s: %q -> %q -> %q (err=%v)", locale, text, once, twice, err))
			}
		})
	}
	if len(broken) > 0 {
		t.Fatalf("non-idempotent cases found:\n%s", strings.Join(broken, "\n"))
	}
}

func TestIdempotentAroundHTMLLineBoundaryMarker(t *testing.T) {
	// modes.md 3.2's LineMarker must be a BREAK member for every rule; this is exactly the class
	// of regression the marker/BREAK handling in this port guards against.
	locales := allLocales(t)
	alphabet := []rune{' ', '"', '-', '.', 'a', '1'}
	var broken []string
	for _, locale := range locales {
		bestBoundedStrings(alphabet, 2, func(left string) {
			bestBoundedStrings(alphabet, 2, func(right string) {
				if len(broken) >= 10 {
					return
				}
				text := left + "<!--\n-->" + right
				once, err := polytypo.Transform(text, polytypo.Options{Locale: locale, Mode: "html"})
				if err != nil {
					broken = append(broken, fmt.Sprintf("%s: %q errored: %v", locale, text, err))
					return
				}
				twice, err := polytypo.Transform(once, polytypo.Options{Locale: locale, Mode: "html"})
				if err != nil || twice != once {
					broken = append(broken, fmt.Sprintf("%s: %q -> %q -> %q (err=%v)", locale, text, once, twice, err))
				}
			})
		})
	}
	if len(broken) > 0 {
		t.Fatalf("non-idempotent cases found:\n%s", strings.Join(broken, "\n"))
	}
}

// TestConcurrentTransform exercises ARCHITECTURE.md section 7's goroutine-safety requirement --
// the one property this runtime can actually test that the others structurally can't ("Go and
// Ruby ports will be called concurrently; JS will not care").
func TestConcurrentTransform(t *testing.T) {
	locales := allLocales(t)
	inputs := []string{
		`She said, "it's fine" -- see 3-5 km.`,
		`Elle a dit "bonjour" et "au revoir".`,
		`Она сказала: "привет" - и ушла...`,
		``,
		`plain text with no typography to fix`,
	}

	var wg sync.WaitGroup
	errs := make(chan error, 256)
	for g := 0; g < 64; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			locale := locales[g%len(locales)]
			input := inputs[g%len(inputs)]
			for i := 0; i < 20; i++ {
				if _, err := polytypo.Transform(input, polytypo.Options{Locale: locale}); err != nil {
					errs <- fmt.Errorf("goroutine %d: %w", g, err)
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
