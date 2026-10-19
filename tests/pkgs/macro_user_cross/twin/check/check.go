package check

import "example.com/macros/util"

func Positive(file string, line int, fn string, v int) {
	util.Assert(file, line, fn, "v > 0", v > 0, func() string { return "v must be positive" })
}

func Both(file string, line int, fn string, v int, w func() int) int {
	Positive(file, line, fn, v)
	return util.Dbg("v + w()", v+w())
}
