package main

func count(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func find() (int, bool) {
	for x := range count(5) {
		if x == 3 {
			return x * 2, true
		}
	}
	return 0, false
}

func quiet() {
	defer println("bye")
	return
}

func main() {
	n := 3
	var k int8
	k = int8(n)
	_ = int64(n)
	m := map[string]int{"a": 7}
	d := 2
	m["a"] %= d
	m["a"] <<= d
	m["a"] /= d
	quiet()
	println(find())
	println(k, m["a"])
}
