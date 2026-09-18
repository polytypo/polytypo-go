package engine

import (
	"fmt"

	"github.com/polytypo/polytypo-go/internal/spec"
)

// ResolveRulePlan applies defaults plus opt-out overrides. rulesOption is opt-out only
// (ARCHITECTURE.md section 7): it may only disable a default-on rule or enable the one
// default-off rule ("ranges"). An unknown key raises CodeUnknownRule.
func ResolveRulePlan(rulesOption map[string]bool) ([]string, error) {
	defaults, err := RuleDefaults()
	if err != nil {
		return nil, err
	}
	enabled := make(map[string]bool, len(defaults))
	for id, on := range defaults {
		enabled[id] = on
	}
	for id, flag := range rulesOption {
		if _, known := defaults[id]; !known {
			return nil, newError(CodeUnknownRule, fmt.Sprintf("unknown rule id: %q", id))
		}
		enabled[id] = flag
	}
	order, err := RuleOrder()
	if err != nil {
		return nil, err
	}
	plan := make([]string, 0, len(order))
	for _, id := range order {
		if enabled[id] {
			plan = append(plan, id)
		}
	}
	return plan, nil
}

// Prepare builds the rule plan, then resolves the locale and loads its data — the setup step
// shared by the text pipeline and the span-runner (html/markdown modes). Order matters and is
// public, tested behaviour: an unknown-rule error must win over an unknown-locale error when
// both are present (mirrors the JS/Python reference implementations exactly).
func Prepare(locale string, rulesOption map[string]bool) (resolvedLocale string, localeData spec.LocaleData, plan []string, err error) {
	plan, err = ResolveRulePlan(rulesOption)
	if err != nil {
		return "", spec.LocaleData{}, nil, err
	}
	resolvedLocale, err = ResolveLocale(locale)
	if err != nil {
		return "", spec.LocaleData{}, nil, err
	}
	localeData, err = spec.LoadLocale(resolvedLocale)
	if err != nil {
		return "", spec.LocaleData{}, nil, newError(CodeMalformedLocaleData, err.Error())
	}
	return resolvedLocale, localeData, plan, nil
}

// RunRules runs each enabled rule in order.json order over cp, applying its edits before the next
// rule sees the array. Shared by text mode and the span-runner (modes.md 3.5), which interposes
// the two boundary filters of modes.md 3.4 between rule and apply.
func RunRules(cp []rune, plan []string, localeData spec.LocaleData, ctx RuleContext) ([]rune, error) {
	current := cp
	for _, ruleID := range plan {
		fn := GetRule(ruleID)
		edits := fn(current, localeData, ctx)
		if len(edits) == 0 {
			continue
		}
		next, err := ApplyEdits(current, edits, ruleID)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return current, nil
}

// RunRulesRecording is RunRules, keeping the edits instead of discarding them (analyze.md
// section 1: same pipeline, same order, reporting rather than applying). The origin map travels
// alongside the array so every change comes back in input coordinates, and filterEdits is the
// hook the span-runner needs for modes.md 3.4's boundary filters — text mode passes nil and gets
// the identity.
func RunRulesRecording(
	cp []rune,
	plan []string,
	localeData spec.LocaleData,
	ctx RuleContext,
	origin []int,
	inputLength int,
	filterEdits func([]rune, []Edit) []Edit,
) ([]Change, error) {
	current := cp
	currentOrigin := origin
	changes := make([]Change, 0)
	for _, ruleID := range plan {
		fn := GetRule(ruleID)
		produced := fn(current, localeData, ctx)
		edits := produced
		if filterEdits != nil {
			edits = filterEdits(current, produced)
		}
		if len(edits) == 0 {
			continue
		}
		changes = append(changes, RecordChanges(current, edits, currentOrigin, inputLength, ruleID)...)
		currentOrigin = ApplyEditsToOrigin(currentOrigin, edits)
		next, err := ApplyEdits(current, edits, ruleID)
		if err != nil {
			return nil, err
		}
		current = next
	}
	return changes, nil
}
