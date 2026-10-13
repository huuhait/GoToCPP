// What the user writes (gocpp syntax), and below it what the compiler generates (plain Go):
//
//	macro Assert(cond bool, msg lazy string) {
//		if !cond {
//			panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", #file(), #line(), #text(cond), msg()))
//		}
//	}
//
//	macro Trace() func() {
//		fmt.Println("enter", #func())
//		return func() { fmt.Println("exit ", #func()) }
//	}
//
//	macro Locked(mu *sync.Mutex) func() {
//		mu.Lock()
//		return mu.Unlock
//	}
package util

import (
	"fmt"
	"sync"

	"spike/macro"
)

func Assert(gp_site *macro.Site, cond bool, msg func() string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: assertion failed: %s: %s", gp_site.File, gp_site.Line, gp_site.Args[0], msg()))
	}
}

func Trace(gp_site *macro.Site) func() {
	fmt.Println("enter", gp_site.Func)
	return func() { fmt.Println("exit ", gp_site.Func) }
}

func Locked(gp_site *macro.Site, mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}
