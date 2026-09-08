package modes

import "sort"

// ByteOffsets converts a byte offset into the original UTF-8 source (as reported by a
// byte-oriented parser — golang.org/x/net/html's Tokenizer, goldmark's text.Segment) into a
// code-point index (ARCHITECTURE.md section 4.2's array-of-code-points discipline, applied here
// for the exact reason it exists elsewhere: an index that means one thing in one representation
// and another in a second is exactly the class of bug that discipline exists to rule out).
// Mirrors the Python port's _ByteOffsets, built for the same reason against tree-sitter's byte
// offsets.
type ByteOffsets struct {
	// byteStarts[i] is the byte offset at which code point i begins; byteStarts[len(cp)] is the
	// total byte length of the source.
	byteStarts []int
}

// NewByteOffsets builds the prefix-sum table from the source's code-point array.
func NewByteOffsets(cp []rune) *ByteOffsets {
	starts := make([]int, len(cp)+1)
	for i, r := range cp {
		starts[i+1] = starts[i] + runeLen(r)
	}
	return &ByteOffsets{byteStarts: starts}
}

func runeLen(r rune) int {
	switch {
	case r < 0x80:
		return 1
	case r < 0x800:
		return 2
	case r < 0x10000:
		return 3
	default:
		return 4
	}
}

// CodePointOf converts a byte offset to the code-point index it falls within (or immediately
// after, for a byte offset exactly at a boundary — the same semantics as Python's
// bisect_right(...) - 1).
func (b *ByteOffsets) CodePointOf(byteOffset int) int {
	// sort.Search finds the first index i such that byteStarts[i] > byteOffset; the code point
	// containing byteOffset is the one just before that (bisect_right - 1).
	i := sort.Search(len(b.byteStarts), func(i int) bool { return b.byteStarts[i] > byteOffset })
	return i - 1
}
