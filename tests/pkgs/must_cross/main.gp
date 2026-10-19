// phase: 09

// Must functions and must calls across packages (step 10.13): a must function of another package
// (its extra error result is in the summary of the package), a must method through an interface, a
// generic must function, a deferred must call, errors.Wrapf and errors.Is across packages, and the
// order of package-level initializers and init! functions. The twin (twin/) is the module written
// with if err != nil, and real Go runs it.

package main

import (
	"errors"
	"fmt"

	"example.com/mustx/conf"
	"example.com/mustx/store"
)

func report(what string, v any, err error) {
	fmt.Println(what, v, err, errors.Is(err, store.ErrMissing))
}

// total is a normal function with must calls of another package's must functions.
func total(name string, keys ...string) (int, error) {
	db := store.Open!(name)
	pair1, pair2 := store.Pair!(db, keys[0], keys[1])
	return pair1 + pair2 + store.Sum!(db, keys...), nil
}

// run is a must function that calls other packages' must functions with and without !.
func run!() {
	n := total("one-store", "one", "two")
	fmt.Println("run: total", n)
	cfg, err := conf.Load(store.Default, "one", "word")
	report("run: load", cfg, err)
	cfg = conf.Load!(store.Default, "one", "two")
	fmt.Println("run: cfg", cfg)
	fmt.Println(conf.Check(store.Default))
	fmt.Println(conf.Fresh("fresh"))
	n64 := store.Parse![int64]("77")
	fmt.Println("run: parse", n64)
	store.Touch!(store.Default)
	fmt.Println("run: not reached by the failing call below")
	fmt.Println(conf.Fresh("bad"))
	_ = store.Parse![int]("x")
	fmt.Println("run: not reached")
}

func main() {
	fmt.Println("main: opened", store.Opened)
	n, err := total("a", "one", "nokey")
	report("main: total", n, err)
	err = run()
	fmt.Println("main: run gives", err)
	fmt.Println("main: opened", store.Opened)
}

// Output:
// store: init 1 default
// main: opened [default]
// store: opened a
// main: total 0 get a/nokey: missing true
// store: opened one-store
// run: total 6
// run: load {0 0} load config: strconv.Atoi: parsing "w": invalid syntax false
// run: cfg {1 2}
// conf: check 42
// 42 <nil>
// store: opened fresh
// store: close fresh
// 1 <nil>
// run: parse 77
// store: close default
// run: not reached by the failing call below
// store: opened bad
// store: close bad
// 1 close bad: missing
// main: run gives strconv.Atoi: parsing "x": invalid syntax
// main: opened [default a one-store fresh bad]
