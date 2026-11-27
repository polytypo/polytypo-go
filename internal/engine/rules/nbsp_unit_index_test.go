package rules

import (
	"strings"
	"testing"

	"github.com/polytypo/polytypo-go/internal/spec"
)

func TestNbspUnitIndexMatchesLinearScan(t *testing.T) {
	reg, err := spec.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range reg.Locales {
		data, err := spec.LoadLocale(code)
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		units := nbPrepareUnits(data.NBSP)
		index := nbIndexUnits(units)
		var probes [][]rune
		for _, u := range units {
			probes = append(probes, u.w)
			probes = append(probes, append([]rune("9 "), u.w...))
			probes = append(probes, append(append([]rune{}, u.w...), 'x'))
			if len(u.w) > 1 {
				probes = append(probes, u.w[:len(u.w)-1])
			}
		}
		// Every unit entry's first letter in its other case must not match: the units are exact.
		for _, u := range units {
			probes = append(probes, []rune(strings.ToUpper(string(u.w))), []rune(strings.ToLower(string(u.w))))
		}
		for _, cp := range probes {
			for a := range cp {
				var want *nbUnitEntry
				for i := range units {
					if nbMatchExact(cp, a, units[i].w) {
						want = &units[i]
						break
					}
				}
				if got := nbUnitAt(cp, a, units, index); got != want {
					t.Fatalf("%s: unit index disagrees with the linear scan at %d in %q", code, a, string(cp))
				}
			}
		}
	}
}
