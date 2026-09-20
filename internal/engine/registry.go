package engine

import (
	"sort"
	"sync"

	"github.com/polytypo/polytypo-go/internal/spec"
)

// RuleContext is the per-call context every rule reads, alongside the code-point array and the
// locale data (mirrors the JS reference implementation's RuleContext / the Python port's
// RuleContext TypedDict).
type RuleContext struct {
	Mode    string
	Dialect string // "" when not markdown
	Locale  string // resolved locale id
	// NarrowTarget is nbsp.md 3.1a's NARROW-TARGET, already resolved to a code point: U+202F by
	// default, U+00A0 when the caller passed NarrowNbsp "nbsp". A rule reads a code point and
	// never the option, so the string never reaches the pipeline.
	NarrowTarget rune
}

// RuleFunc is one rule's scan function: given the current code-point array, the resolved locale's
// data, and the call context, return non-overlapping edits in ascending order of Start.
type RuleFunc func(cp []rune, locale spec.LocaleData, ctx RuleContext) []Edit

var (
	orderOnce   sync.Once
	orderData   spec.Order
	orderErr    error
	ruleOrder   []string
	ruleDefault map[string]bool
)

func loadOrder() {
	orderData, orderErr = spec.LoadOrder()
	if orderErr != nil {
		return
	}
	rules := make([]spec.Rule, len(orderData.Rules))
	copy(rules, orderData.Rules)
	sort.Slice(rules, func(i, j int) bool { return rules[i].Order < rules[j].Order })

	ruleOrder = make([]string, len(rules))
	ruleDefault = make(map[string]bool, len(rules))
	for i, r := range rules {
		ruleOrder[i] = r.ID
		// order.json's "default" field is the string "on"/"off", not a JSON boolean.
		ruleDefault[r.ID] = r.Default == "on"
	}
}

// RuleOrder returns rule ids in ascending pipeline order (spec/rules/order.json), never
// registration order and never map-iteration order (ARCHITECTURE.md section 4.5).
func RuleOrder() ([]string, error) {
	orderOnce.Do(loadOrder)
	if orderErr != nil {
		return nil, orderErr
	}
	return ruleOrder, nil
}

// RuleDefaults returns rule id -> default enabled/disabled (only "ranges" defaults to false).
func RuleDefaults() (map[string]bool, error) {
	orderOnce.Do(loadOrder)
	if orderErr != nil {
		return nil, orderErr
	}
	return ruleDefault, nil
}

var (
	rulesMu sync.RWMutex
	rules   = map[string]RuleFunc{}
)

// RegisterRule adds a rule implementation to the registry. Called from each rule package's
// init(), mirroring the JS/Python reference implementations' static-import-registers-every-rule
// pattern — importing the engine's rules package is enough, with no separate caller-side step.
func RegisterRule(id string, fn RuleFunc) {
	rulesMu.Lock()
	defer rulesMu.Unlock()
	rules[id] = fn
}

// GetRule returns the registered implementation for a rule id (exported for internal/modes'
// span-runner, which interposes the boundary filters of modes.md 3.4 between rule and apply —
// the same sequence RunRules uses for text mode).
func GetRule(id string) RuleFunc {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	return rules[id]
}

// KnownRuleIDs reports whether id is a real rule id from order.json.
func KnownRuleIDs() (map[string]bool, error) {
	return RuleDefaults()
}
