// The hand-written Go twin of umacro_contexts.gp.

package main

import (
	"fmt"
	"sync"
)

func tag[T any](gp_file string, gp_line int, gp_func string, v T) string {
	return fmt.Sprintf("%v@%d", v, gp_line)
}

func show(gp_file string, gp_line int, gp_func string, v int) int {
	fmt.Println("show", v, gp_func)
	return v
}

const file = "umacro_contexts.gp"

var first = show(file, 27, "main.init", 1)

var table = map[string]int{
	"a": show(file, 30, "main.init", 10),
	"b": show(file, 31, "main.init", 20),
}

type Box struct {
	Label string
	N     int
}

func (b Box) Describe() string { return tag(file, 39, "main.Box.Describe", b.N) }

func noUse() int {
	show := 5
	return show
}

func label(n int) string {
	switch n {
	case show(file, 48, "main.label", 1):
		return "one"
	case show(file, 50, "main.label", 2) + 1:
		return "three"
	}
	return "other"
}

func collect() (out []string) {
	for i := 0; i < show(file, 57, "main.collect", 2); i++ {
		out = append(out, tag(file, 58, "main.collect", i))
	}
	return
}

func main() {
	macro, lazy := 1, 2
	fmt.Println(macro + lazy)

	fmt.Println(first, table["a"]+table["b"], noUse())

	b := Box{Label: "box", N: show(file, 71, "main.main", 5)}
	fmt.Println(b, b.Describe())
	desc := b.Describe
	fmt.Println(desc())

	fmt.Println(label(1), label(3), label(7))
	fmt.Println(collect())

	f := func() int { return show(file, 80, "main.main.func1", 6) }
	fmt.Println(f())
	var wg sync.WaitGroup
	res := make([]int, 3)
	for i := range 3 {
		wg.Go(func() { res[i] = show(file, 85, "main.main.func2", 0) + i })
	}
	wg.Wait()
	fmt.Println(res)

	ch := make(chan int, 1)
	ch <- show(file, 92, "main.main", 8)
	fmt.Println(<-ch, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}[show(file, 93, "main.main", 3)], show(file, 93, "main.main", 1)+show(file, 93, "main.main", 2)*show(file, 93, "main.main", 3))
	if v := show(file, 94, "main.main", 4); v > 3 {
		fmt.Println("v =", v)
	}
	goto end
end:
	fmt.Println(tag(file, 99, "main.main", "end"))
}
