// phase: 05

// Package check has macros that use macros of util: the place stays that of the outermost use.
package check

import "example.com/macros/util"

macro Positive(v int) {
	#util.Assert(v > 0, "v must be positive")
}

macro Both(v int, w lazy int) int {
	#Positive(v)
	return #util.Dbg(v + w())
}
