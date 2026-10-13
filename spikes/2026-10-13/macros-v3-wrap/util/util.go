// What the user writes (gocpp syntax), and below it what the compiler generates (plain Go).
// A macro that calls #body() is a wrap macro: it gets the function's own code and decides
// how, when and how often it runs. The compiler passes that code as a func() value
// (in C++ a lambda passed to an inline template, so nothing is allocated).
//
//	macro Trace() func() {              // start style (no #body): runs first, its func() at the end
//		fmt.Println("enter", #func())
//		return func() { fmt.Println("exit ", #func()) }
//	}
//
//	macro VM() {                        // wrap style: run the body "inside a VM"
//		vmEnter(#func())
//		defer vmExit(#func())
//		#body()
//	}
//
//	macro Recover() {                   // a panic in the body is logged, the function returns zero values
//		defer func() {
//			if r := recover(); r != nil {
//				fmt.Println("recovered in", #func()+":", r)
//			}
//		}()
//		#body()
//	}
//
//	macro InThread() {                  // run the body on its own thread and wait for it
//		done := make(chan struct{})
//		go func() {
//			defer close(done)
//			#body()
//		}()
//		<-done
//	}
//
//	macro Once(o *sync.Once) {          // the body runs at most once: #body() may run 0 times
//		o.Do(#body)
//	}
package util

import (
	"fmt"
	"sync"

	"spike/macro"
)

func Trace(gp_site *macro.Site) func() {
	fmt.Println("enter", gp_site.Func)
	return func() { fmt.Println("exit ", gp_site.Func) }
}

func VM(gp_site *macro.Site, gp_body func()) {
	vmEnter(gp_site.Func)
	defer vmExit(gp_site.Func)
	gp_body()
}

func Recover(gp_site *macro.Site, gp_body func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("recovered in", gp_site.Func+":", r)
		}
	}()
	gp_body()
}

func InThread(gp_site *macro.Site, gp_body func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		gp_body()
	}()
	<-done
}

func Once(gp_site *macro.Site, o *sync.Once, gp_body func()) {
	o.Do(gp_body)
}

func vmEnter(f string) { fmt.Println("  vm enter:", f) }
func vmExit(f string)  { fmt.Println("  vm exit: ", f) }
