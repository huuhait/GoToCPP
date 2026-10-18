package mk

import "example.com/anonemb/base"

// Tagged returns an unnamed struct type with an embedded field of another package; main and
// the package other write the same type.
func Tagged(n int) struct {
	base.Base
	Tag string
} {
	return struct {
		base.Base
		Tag string
	}{base.New(n), "mk"}
}

func AsGetter(g base.Getter) int { return g.Get() }
