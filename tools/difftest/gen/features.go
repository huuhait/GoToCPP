package gen

import (
	"fmt"
	"sort"
	"strings"
)

// featurePhase is the phase of the plan in which gocpp accepts each language feature. The
// generator only uses a feature when the feature set has it: the set is built from the phase
// the compiler under test has reached ("-phase N"), and individual features can be switched
// on or off ("-features +generics,-select").
var featurePhase = map[string]int{
	"ints": 2, "strings": 2, "floats": 2, "bools": 2, "consts": 2, "labels": 2, "shadow": 2,
	"slices": 3, "arrays": 3, "maps": 3, "structs": 3, "pointers": 3, "closures": 3,
	"methods": 3, "variadic": 3, "multiret": 3, "minmax": 3,
	"ifaces": 5, "embed": 5, "typeswitch": 5, "defer": 5, "errors": 5, "methodvals": 5,
	"fmtverbs": 5,
	"generics": 7,
	"chan":     8, "goroutines": 8, "select": 8,
	"osexit": 9, "sync": 9,
	// standard packages that gocpp has ported (stdlib/PORTED.txt) and the language corners that
	// the reviewer of 15.9 asked for
	"pkgstrings": 9, "pkgstrconv": 9, "pkgunicode": 9, "pkgbytes": 9, "pkgmath": 9, "pkgerrors": 9,
	"pkgsync": 9, "pkgtime": 9,
	"println": 3, "complex": 3, "fmtmethods": 5, "mapkeys": 5, "floatconv": 3, "utf8bad": 3,
	// floatoor: float to integer conversions of values that do not fit (implementation-defined
	// in Go, amd64's instructions in gc, which gocpp emulates)
	"floatoor": 3,
	// capgrowth: cap() after append growth (gc's size classes: gocpp differs for pointer-
	// containing and zero-size elements, a known gap). Off unless asked for.
	"capgrowth": 99,
}

// Features is a set of language features the generator may use.
type Features map[string]bool

// AllFeatures lists every feature name, sorted.
func AllFeatures() []string {
	var s []string
	for k := range featurePhase {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}

// FeaturesForPhase returns the features that gocpp accepts at the given phase.
func FeaturesForPhase(phase int) Features {
	f := Features{}
	for k, p := range featurePhase {
		if p <= phase {
			f[k] = true
		}
	}
	return f
}

// Parse applies a comma list of "+name", "-name" and "name" (= +name) to f. "all" and "none"
// stand for every feature and for the empty set.
func (f Features) Parse(spec string) error {
	for _, w := range strings.Split(spec, ",") {
		w = strings.TrimSpace(w)
		if w == "" {
			continue
		}
		on := true
		switch w[0] {
		case '+':
			w = w[1:]
		case '-':
			on = false
			w = w[1:]
		}
		switch w {
		case "all":
			for k := range featurePhase {
				f[k] = on
			}
			continue
		case "none":
			for k := range featurePhase {
				f[k] = !on
			}
			continue
		}
		if _, ok := featurePhase[w]; !ok {
			return fmt.Errorf("unknown feature %q (have: %s)", w, strings.Join(AllFeatures(), ", "))
		}
		f[w] = on
	}
	return nil
}

// GatePhase is the phase gocpp must be told to accept ("GOCPP_GATE_PHASE") so that every
// feature in f builds: the highest phase of a feature that is switched on.
func (f Features) GatePhase() int {
	p := 0
	for k, on := range f {
		if on && featurePhase[k] > p && featurePhase[k] <= 15 {
			p = featurePhase[k]
		}
	}
	return p
}

// Names returns the switched-on features, sorted.
func (f Features) Names() []string {
	var s []string
	for k, on := range f {
		if on {
			s = append(s, k)
		}
	}
	sort.Strings(s)
	return s
}

// Clone returns a copy.
func (f Features) Clone() Features {
	g := Features{}
	for k, v := range f {
		g[k] = v
	}
	return g
}
