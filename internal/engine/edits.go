package engine

import "fmt"

// Edit is one replacement a rule proposes: replace cp[Start:End] with Replacement. Indices
// address the code-point array, never a native string (docs/ARCHITECTURE.md section 4.2).
type Edit struct {
	Start       int
	End         int
	Replacement []rune
	RuleID      string
}

// ValidateEdits enforces the rule contract rather than trusting it (rules are
// community-contributed): edits must be in bounds, ascending, non-overlapping, and made of real
// code points. ruleID is empty when validating a caller-agnostic batch (mirrors the JS/Python
// reference implementations' optional ruleId parameter).
func ValidateEdits(edits []Edit, length int, ruleID string) error {
	previousEnd := 0
	for i, edit := range edits {
		where := fmt.Sprintf("edit %d", i)
		if ruleID != "" {
			where = fmt.Sprintf("rule %q edit %d", ruleID, i)
		}

		if edit.Start < 0 || edit.End > length {
			return newError(CodeRuleContract, fmt.Sprintf(
				"%s is out of bounds (%d, %d) for length %d", where, edit.Start, edit.End, length))
		}
		if edit.End < edit.Start {
			return newError(CodeRuleContract, fmt.Sprintf(
				"%s has end %d before start %d", where, edit.End, edit.Start))
		}
		if edit.Start < previousEnd {
			return newError(CodeRuleContract, fmt.Sprintf(
				"%s starts at %d, which overlaps or precedes the previous edit", where, edit.Start))
		}
		if ruleID != "" && edit.RuleID != ruleID {
			return newError(CodeRuleContract, fmt.Sprintf(
				"%s is tagged %q but was produced by rule %q", where, edit.RuleID, ruleID))
		}
		for _, value := range edit.Replacement {
			if !IsValidCodePoint(value) {
				return newError(CodeRuleContract, fmt.Sprintf(
					"%s contains an invalid code point (%d)", where, value))
			}
		}
		previousEnd = edit.End
	}
	return nil
}

// ApplyEdits applies validated edits left to right, producing a new code-point array.
func ApplyEdits(cp []rune, edits []Edit, ruleID string) ([]rune, error) {
	if err := ValidateEdits(edits, len(cp), ruleID); err != nil {
		return nil, err
	}
	if len(edits) == 0 {
		out := make([]rune, len(cp))
		copy(out, cp)
		return out, nil
	}

	out := make([]rune, 0, len(cp))
	cursor := 0
	for _, edit := range edits {
		out = append(out, cp[cursor:edit.Start]...)
		out = append(out, edit.Replacement...)
		cursor = edit.End
	}
	out = append(out, cp[cursor:]...)
	return out, nil
}
