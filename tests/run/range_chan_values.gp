// phase: 08

// Range over channels of structs, arrays, strings, interfaces, funcs and pointers; the elements are
// copies.
package main

import "fmt"

type item struct {
	name string
	tags []string
	n    [2]int
}

type namer interface{ Name() string }

func (i item) Name() string { return i.name }

func main() {
	items := make(chan item, 3)
	orig := item{"a", []string{"t"}, [2]int{1, 2}}
	items <- orig
	orig.name = "changed"
	orig.n[0] = 99
	items <- orig
	items <- item{name: "c"}
	close(items)
	for it := range items {
		fmt.Printf("%+v\n", it)
	}

	nm := make(chan namer, 2)
	nm <- item{name: "x"}
	nm <- nil
	close(nm)
	for n := range nm {
		if n == nil {
			fmt.Println("nil namer")
			continue
		}
		fmt.Println(n.Name())
	}

	fn := make(chan func() string, 2)
	for _, s := range []string{"p", "q"} {
		fn <- func() string { return s + s }
	}
	close(fn)
	for f := range fn {
		fmt.Println(f())
	}

	ptrs := make(chan *item, 2)
	x := &item{name: "shared"}
	ptrs <- x
	ptrs <- x
	close(ptrs)
	for p := range ptrs {
		p.n[0]++
	}
	fmt.Println(x.n)

	chans := make(chan chan int, 2)
	for i := 0; i < 2; i++ {
		c := make(chan int, 1)
		c <- i + 100
		chans <- c
	}
	close(chans)
	for c := range chans {
		fmt.Println(<-c)
	}

	anys := make(chan any, 4)
	anys <- 1
	anys <- "s"
	anys <- 2.5
	anys <- []int{1}
	close(anys)
	for a := range anys {
		fmt.Printf("%v %T\n", a, a)
	}
}

// Output:
// {name:a tags:[t] n:[1 2]}
// {name:changed tags:[t] n:[99 2]}
// {name:c tags:[] n:[0 0]}
// x
// nil namer
// pp
// qq
// [2 0]
// 100
// 101
// 1 int
// s string
// 2.5 float64
// [1] []int
