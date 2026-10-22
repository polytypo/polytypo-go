package engine

import "fmt"

// Range is one element of the Protect option (modes.md 3.9.2): the code points at
// Start ... End-1 of the input, as code-point offsets and not bytes.
type Range struct {
	Start int
	End   int
}

// ResolveProtect validates the Protect option against the input's length in code points and
// merges touching ranges into maximal runs (modes.md 3.9.2). Absent (nil) and an empty slice are
// the same call. Go's type system already rules out a value that is not a list of integer pairs,
// so what remains is the range arithmetic.
//
// Checked after every option that precedes it in the mode's chain (ARCHITECTURE.md 7).
func ResolveProtect(protect []Range, length int) ([]Range, error) {
	if len(protect) == 0 {
		return nil, nil
	}
	runs := make([]Range, 0, len(protect))
	previousEnd := 0
	for _, r := range protect {
		if r.Start < 0 || r.End <= r.Start {
			return nil, NewError(CodeInvalidOption, fmt.Sprintf(
				"protect range [%d, %d) is empty or negative", r.Start, r.End))
		}
		if r.Start < previousEnd {
			return nil, NewError(CodeInvalidOption, fmt.Sprintf(
				"protect range [%d, %d) overlaps or precedes the previous one", r.Start, r.End))
		}
		if r.End > length {
			return nil, NewError(CodeInvalidOption, fmt.Sprintf(
				"protect range [%d, %d) runs past the end of the input (%d code points)", r.Start, r.End, length))
		}
		if n := len(runs); n > 0 && runs[n-1].End == r.Start {
			runs[n-1].End = r.End
		} else {
			runs = append(runs, r)
		}
		previousEnd = r.End
	}
	return runs, nil
}

// AssertNoProtect is modes.md 3.9.2: outside "text" mode a non-empty Protect is refused, not
// ignored, because ignoring it would silently typeset text the caller marked as not theirs to
// touch. An empty list is accepted everywhere, being absent.
func AssertNoProtect(protect []Range, mode string) error {
	if len(protect) == 0 {
		return nil
	}
	return NewError(CodeInvalidOption, fmt.Sprintf(`"protect" is only available when mode is "text", not %q`, mode))
}
