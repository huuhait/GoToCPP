// What the user writes (gocpp syntax):
//
//	#util.Trace()
//	#util.VM()
//	func secret(key string) (int, error) {
//		if key == "" {
//			return 0, errors.New("empty key")
//		}
//		defer fmt.Println("  secret: body defer")
//		return len(key) * 42, nil
//	}
//
//	#util.Recover()
//	func risky(n int) int { return 10 / n }
//
//	#util.InThread()
//	func work(id int) string { return fmt.Sprint("job ", id, " done") }
//
//	#util.Once(&setupOnce)
//	func setup() int { calls++; return calls }
//
// What the compiler generates (plain Go, checked by stock go/types). On-top
// macros apply top to bottom; a wrap macro moves the body into a closure, and
// `return x` inside it becomes "set the results, then return".
package main

import (
	"errors"
	"fmt"
	"sync"

	"spike/macro"
	"spike/util"
)

var (
	gp_site_1 = macro.Site{File: "main.gp", Line: 3, Func: "main.secret"}
	gp_site_2 = macro.Site{File: "main.gp", Line: 4, Func: "main.secret"}
	gp_site_3 = macro.Site{File: "main.gp", Line: 13, Func: "main.risky"}
	gp_site_4 = macro.Site{File: "main.gp", Line: 16, Func: "main.work"}
	gp_site_5 = macro.Site{File: "main.gp", Line: 19, Func: "main.setup"}
)

func secret(key string) (gp_r0 int, gp_r1 error) {
	defer util.Trace(&gp_site_1)() // start style: inserted first
	util.VM(&gp_site_2, func() {   // wrap style: the body runs where VM calls #body()
		if key == "" {
			gp_r0, gp_r1 = 0, errors.New("empty key")
			return
		}
		defer fmt.Println("  secret: body defer")
		gp_r0, gp_r1 = len(key)*42, nil
		return
	})
	return
}

func risky(n int) (gp_r0 int) {
	util.Recover(&gp_site_3, func() {
		gp_r0 = 10 / n
		return
	})
	return
}

func work(id int) (gp_r0 string) {
	util.InThread(&gp_site_4, func() {
		gp_r0 = fmt.Sprint("job ", id, " done")
		return
	})
	return
}

var (
	setupOnce sync.Once
	calls     int
)

func setup() (gp_r0 int) {
	util.Once(&gp_site_5, &setupOnce, func() {
		calls++
		gp_r0 = calls
		return
	})
	return
}

func main() {
	fmt.Println(secret("key"))
	fmt.Println(secret(""))
	fmt.Println("risky(2) =", risky(2))
	fmt.Println("risky(0) =", risky(0))

	var wg sync.WaitGroup
	results := make([]string, 4)
	for i := range 4 {
		wg.Add(1)
		go func() { defer wg.Done(); results[i] = work(i) }()
	}
	wg.Wait()
	fmt.Println(results)

	fmt.Println("setup:", setup(), setup(), "calls:", calls)
}
