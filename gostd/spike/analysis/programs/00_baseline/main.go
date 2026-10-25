// Baseline: the constructs the 16.1c GC emitter supports today. It must compile with
// `gocpp run -gc-engine` and print the expected lines, so a failure of the gocpp harness shows
// up here before it is blamed on a newer construct.
package main

func add(a, b int) int {
	return a + b
}

func main() {
	answer := 42
	var total int
	for i := 0; i < 5; i++ {
		if i == 3 {
			continue
		}
		total += i
	}
	println("answer", answer)
	println("total", total)
	println("arith", 7/2, 7%2, 7<<3, 7>>1, 6&3, 6|3, 6^3)
	s := "go"
	println("len", len(s), "merged", s+"cpp")
	var b bool
	println("bool", b, !b)
	println("order", add(1, 2*3), 10-2-3)
	n := 0
	for n < 3 {
		n++
	}
	println("while", n)
}
