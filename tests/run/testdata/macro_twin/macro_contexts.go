// The twin of macro_contexts.gp: the built-ins are the constants the compiler puts in their place; #func() is a
// call that asks the Go runtime for the name of its caller. The embedded files are literals.

// The built-in macros in many places (Phase 10 step 10.8): constants and array lengths,
// switch cases, generic functions, method values, struct literals, goroutines, defer, labels,
// package-level initializers, and #embed as an untyped string constant.

package main

import (
	"fmt"
	"runtime"
)

const banner = "gocpp banner\nline two\n"
const bannerLen = len(banner)
const here = 13

var raw = "\x00\x01\xfe\xff"
var none = ""

var table = [...]string{fn(), "macro_contexts.gp", "x + y"}
var sizes [19 % 7]int

type Rec struct {
	Where string
	Line  int
	Name  string
}

func Id[T any](x T) (T, string, int) { return x, fn(), 27 }

func Log[T any](v T) string { return fmt.Sprint(v, " at ", 29, " in ", fn(), " ", "v") }

type Box struct{ n int }

func (b *Box) Where() Rec { return Rec{"macro_contexts.gp", 33, fn()} }

func classify(n int) string {
	switch n {
	case 37:
		return "this line"
	case here:
		return "the const line"
	case bannerLen:
		return "banner length"
	}
	return "other"
}

func main() {
	fmt.Printf("%q %d %d\n", banner, bannerLen, here)
	fmt.Printf("%q %d %q %d\n", raw, len(raw), none, len(none))
	fmt.Println(table, len(sizes))
	fmt.Println(Id(1))
	fmt.Println(Id("s"))
	fmt.Println(Log(42))
	fmt.Println(Log("x"))
	fmt.Println((&Box{}).Where())
	w := (&Box{}).Where
	fmt.Println(w())
	fmt.Println(classify(here), classify(bannerLen), classify(1), classify(37))

	done := make(chan bool)
	go func() {
		defer close(done)
		fmt.Println("goroutine", fn(), 63)
	}()
	<-done

	defer fmt.Println("deferred", fn(), 67)
	r := Rec{Where: "macro_contexts.gp", Line: 68, Name: fn()}
	fmt.Println(r)
outer:
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			if j == 1 {
				fmt.Println("label", 74)
				continue outer
			}
		}
	}
	fmt.Println("a b" + "|" + banner[:5])
}

// fn is #func(): the name of the function that called it, as the runtime knows it.
func fn() string {
	var pc [1]uintptr
	runtime.Callers(2, pc[:])
	f, _ := runtime.CallersFrames(pc[:]).Next()
	return f.Function
}
