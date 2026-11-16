package polytypo_test

import (
	"fmt"
	"reflect"
	"testing"

	polytypo "github.com/polytypo/polytypo-go"
)

func TestSkippedElementsAreTheTenNamesInSpecOrder(t *testing.T) {
	want := []string{"code", "pre", "kbd", "samp", "var", "script", "style", "textarea", "svg", "math"}
	if got := polytypo.SkippedElements(); !reflect.DeepEqual(got, want) {
		t.Fatalf("SkippedElements() = %v, want %v", got, want)
	}
}

func TestSkippedElementsReturnsACopy(t *testing.T) {
	polytypo.SkippedElements()[0] = "p"
	if got := polytypo.SkippedElements()[0]; got != "code" {
		t.Fatalf("a caller changed the list the mode uses: first name is now %q", got)
	}
}

func TestEveryPublishedNameIsLeftAloneWithItsSubtree(t *testing.T) {
	for _, tag := range polytypo.SkippedElements() {
		in := fmt.Sprintf("<p>a...b</p><%[1]s>c...d</%[1]s>", tag)
		want := fmt.Sprintf("<p>a…b</p><%[1]s>c...d</%[1]s>", tag)
		got, err := polytypo.Transform(in, polytypo.Options{Locale: "en-US", Mode: "html"})
		if err != nil || got != want {
			t.Errorf("%s: got %q, err %v, want %q", tag, got, err, want)
		}
	}
}
