// phase: 07

// A yield function that outlives its loop and is called after the stack frames of the function
// with the loop were reused: Go panics ("range function continued iteration after whole loop exit")
// before the body touches anything. The body captures a big array, a string, a slice and a counter
// of the dead frame; under ASan any read of the dead frame would be reported.
package main

import "fmt"

var saved func(int) bool

func iter(yield func(int) bool) {
	saved = yield
	yield(1)
}

func loop() (get func() int) {
	var big [64]int
	for i := range big {
		big[i] = i * i
	}
	name := fmt.Sprint("frame-", len(big))
	items := []string{"a", "b", "c"}
	count := 0
	for x := range iter {
		count += x + big[x] + len(name) + len(items)
		items = append(items, name)
	}
	return func() int { return count + len(items) }
}

func smash(n int) int {
	var junk [256]int
	for i := range junk {
		junk[i] = -n
	}
	if n > 0 {
		return smash(n-1) + junk[n]
	}
	return junk[0]
}

func try(name string) {
	defer func() { fmt.Println(name, "->", recover()) }()
	saved(2)
	fmt.Println("not reached")
}

func main() {
	get := loop()
	fmt.Println("count", get())
	smash(20)
	try("after smash")
	fmt.Println("count", get())
	for i := 0; i < 3; i++ {
		smash(i)
		try(fmt.Sprint("again ", i))
	}
	fmt.Println("count", get())
}

// Output:
// count 17
// after smash -> runtime error: range function continued iteration after whole loop exit
// count 17
// again 0 -> runtime error: range function continued iteration after loop body panic
// again 1 -> runtime error: range function continued iteration after loop body panic
// again 2 -> runtime error: range function continued iteration after loop body panic
// count 17
