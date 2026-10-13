package util

import (
	"fmt"
	"os"

	"spike/gocpp/macro"
)

// What the user writes (definitions are plain, type-checked functions):
//
//	//gocpp:macro
//	func Assert(site *macro.Site, cond bool, msg macro.Lazy[string]) { ... }
//
// and calls it as  #util.Assert(len(xs) > 0, "need items: "+expensive())

//gocpp:macro
func Assert(site *macro.Site, cond bool, msg macro.Lazy[string]) {
	if !cond {
		panic(fmt.Sprintf("%s: assertion failed: %s: %s", site.Pos(), site.Text(0), msg.Get()))
	}
}

//gocpp:macro
func Dbg[T any](site *macro.Site, v T) T {
	fmt.Fprintf(os.Stderr, "[%s] %s = %v\n", site.Pos(), site.Text(0), v)
	return v
}
