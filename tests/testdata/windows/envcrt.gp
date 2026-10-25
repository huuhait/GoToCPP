// phase: 14

// The C library's copy of the environment after os.Setenv and os.Unsetenv, with a block that has
// several variables whose names differ only in case (tests/godebug_env_test.go starts it with
// exact environments): C's getenv must give the block's first value of the name, or NULL when
// there is none or it is empty. Each line: the step, Go's view, the block's variables of the name
// in order, C's view.
package main

import (
	"fmt"
	"os"
	"strings"

	"<stdlib.h>"
)

func show(what string) {
	var es []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(strings.ToUpper(e), "ZV=") {
			es = append(es, e)
		}
	}
	v, ok := os.LookupEnv("ZV")
	c := "<NULL>"
	if p := stdlib.getenv(C.cstr("ZV")); p != nil {
		c = C.GoString(p)
	}
	fmt.Printf("%s|%q,%v|%s|%s\n", what, v, ok, strings.Join(es, " "), c)
}

func main() {
	show("start")
	os.Unsetenv("ZV")
	show("unset ZV")
	os.Unsetenv("ZV")
	show("unset ZV again")
	os.Setenv("zv", "s1")
	show("set zv=s1")
	os.Setenv("ZV", "")
	show("set ZV empty")
	os.Setenv("ZV", "s2")
	show("set ZV=s2")
	os.Unsetenv("zv")
	show("unset zv")
}
