package b

import "example.com/gtdesconly/a"

// W holds instances of a's generic type that only W's descriptor names: nothing in b calls a
// method of them or makes one.
type W struct {
	Ptr *a.Box[string]
	Val a.Box[int]
}

type Mine interface{ Do() W }
