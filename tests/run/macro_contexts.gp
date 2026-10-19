// phase: 08

// The built-in macros in many places (Phase 10 step 10.8): constants and array lengths,
// switch cases, generic functions, method values, struct literals, goroutines, defer, labels,
// package-level initializers, and #embed as an untyped string constant.

package main

import "fmt"

const banner = #embed("embeddata/banner.txt")
const bannerLen = len(#embed("embeddata/banner.txt"))
const here = #line()

var raw = #embed("embeddata/raw.bin")
var none = #embed("embeddata/none.txt")

var table = [...]string{#func(), #file(), #text(x + y)}
var sizes [#line() % 7]int

type Rec struct {
	Where string
	Line  int
	Name  string
}

func Id[T any](x T) (T, string, int) { return x, #func(), #line() }

func Log[T any](v T) string { return fmt.Sprint(v, " at ", #line(), " in ", #func(), " ", #text(v)) }

type Box struct{ n int }

func (b *Box) Where() Rec { return Rec{#file(), #line(), #func()} }

func classify(n int) string {
	switch n {
	case #line():
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
		fmt.Println("goroutine", #func(), #line())
	}()
	<-done

	defer fmt.Println("deferred", #func(), #line())
	r := Rec{Where: #file(), Line: #line(), Name: #func()}
	fmt.Println(r)
outer:
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			if j == 1 {
				fmt.Println("label", #line())
				continue outer
			}
		}
	}
	fmt.Println(#embed("embeddata/with space.txt") + "|" + #embed("embeddata/banner.txt")[:5])
}

// Output:
// "gocpp banner\nline two\n" 22 13
// "\x00\x01\xfe\xff" 4 "" 0
// [main.init macro_contexts.gp x + y] 5
// 1 main.Id[...] 27
// s main.Id[...] 27
// 42 at 29 in main.Log[...] v
// x at 29 in main.Log[...] v
// {macro_contexts.gp 33 main.(*Box).Where}
// {macro_contexts.gp 33 main.(*Box).Where}
// the const line banner length other this line
// goroutine main.main.func1 63
// {macro_contexts.gp 68 main.main}
// label 74
// label 74
// a b|gocpp
// deferred main.main 67
