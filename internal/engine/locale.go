package engine

import (
	"fmt"
	"strings"
	"sync"

	"github.com/polytypo/polytypo-go/internal/spec"
)

const (
	hyphenCP     rune = 0x2d
	underscoreCP rune = 0x5f
	upperACP     rune = 0x41
	upperZCP     rune = 0x5a
	lowerACP     rune = 0x61
	lowerZCP     rune = 0x7a
	caseGap      rune = 0x20
)

var (
	registryOnce sync.Once
	registryData spec.Registry
	registryErr  error
)

func registry() (spec.Registry, error) {
	registryOnce.Do(func() {
		registryData, registryErr = spec.LoadRegistry()
	})
	return registryData, registryErr
}

func isLowerASCII(cp rune) bool { return cp >= lowerACP && cp <= lowerZCP }
func isUpperASCII(cp rune) bool { return cp >= upperACP && cp <= upperZCP }

// canonicalize is spec/rules/locale-resolution.md 3.2. ASCII arithmetic only: no ToUpper/ToLower,
// no ICU, no host locale, so a Turkish process resolves "EN-us" exactly as a Finnish one does
// (ARCHITECTURE.md 4.4).
func canonicalize(tag string) []rune {
	c := ToCodePoints(tag)
	for i, cp := range c {
		if cp == underscoreCP {
			c[i] = hyphenCP
		}
	}
	m := len(c)
	if m >= 2 {
		for _, j := range [2]int{0, 1} {
			if isUpperASCII(c[j]) {
				c[j] += caseGap
			}
		}
	}
	if m == 5 && c[2] == hyphenCP {
		for _, j := range [2]int{3, 4} {
			if isLowerASCII(c[j]) {
				c[j] -= caseGap
			}
		}
	}
	return c
}

// hasAcceptedShape is spec/rules/locale-resolution.md 3.3. Exactly two accepted shapes, tested by
// index rather than by pattern (ARCHITECTURE.md 4.1). Everything else is malformed and throws
// like any unknown tag.
func hasAcceptedShape(c []rune) bool {
	switch len(c) {
	case 2:
		return isLowerASCII(c[0]) && isLowerASCII(c[1])
	case 5:
		return isLowerASCII(c[0]) && isLowerASCII(c[1]) && c[2] == hyphenCP &&
			isUpperASCII(c[3]) && isUpperASCII(c[4])
	default:
		return false
	}
}

// aliasTarget resolves tag through the registry's alias table. Alias values are concrete locales
// by registry invariant (locale-resolution.md 2); a violation is a spec-data bug, reported as
// CodeMalformedLocaleData rather than trusted.
func aliasTarget(tag string) (string, bool, error) {
	reg, err := registry()
	if err != nil {
		return "", false, err
	}
	target, ok := reg.Aliases[tag]
	if !ok {
		return "", false, nil
	}
	for _, known := range reg.Locales {
		if known == target {
			return target, true, nil
		}
	}
	return "", false, newError(CodeMalformedLocaleData, fmt.Sprintf(
		"registry alias %q points at %q, which is not a declared locale", tag, target))
}

// lookup checks the exact locale list before the alias table, so a registry that wrongly lists a
// tag in both cannot make lookup order observable (locale-resolution.md 3.4).
func lookup(tag string) (string, bool, error) {
	reg, err := registry()
	if err != nil {
		return "", false, err
	}
	for _, known := range reg.Locales {
		if known == tag {
			return tag, true, nil
		}
	}
	return aliasTarget(tag)
}

// ResolveLocale implements spec/rules/locale-resolution.md: exact match, then the registry alias
// table, then the language subtag alone once with no chain. Never a platform locale negotiator —
// those disagree across runtimes (ARCHITECTURE.md 4.7). An empty tag has no accepted shape and
// falls through to the same unknown-locale error as any other unresolvable tag; Go's static
// typing makes JS/Python's separate "not a string at all" branch structurally unreachable here.
func ResolveLocale(tag string) (string, error) {
	c := canonicalize(tag)
	if hasAcceptedShape(c) {
		canonical := FromCodePoints(c)
		if direct, ok, err := lookup(canonical); err != nil {
			return "", err
		} else if ok {
			return direct, nil
		}
		if len(c) == 5 {
			base := FromCodePoints(c[0:2])
			if resolved, ok, err := lookup(base); err != nil {
				return "", err
			} else if ok {
				return resolved, nil
			}
		}
	}
	reg, err := registry()
	if err != nil {
		return "", err
	}
	return "", newError(CodeUnknownLocale, fmt.Sprintf(
		"unknown locale %q. Known locales: %s.", tag, strings.Join(reg.Locales, ", ")))
}

// GetLocaleData resolves tag and returns its declarative data.
func GetLocaleData(tag string) (spec.LocaleData, error) {
	resolved, err := ResolveLocale(tag)
	if err != nil {
		return spec.LocaleData{}, err
	}
	data, err := spec.LoadLocale(resolved)
	if err != nil {
		return spec.LocaleData{}, newError(CodeMalformedLocaleData, err.Error())
	}
	return data, nil
}
