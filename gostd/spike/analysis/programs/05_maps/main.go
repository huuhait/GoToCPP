// Maps: literals, insert, read with and without comma-ok, delete, clear, len, range (the sum is
// deterministic; the iteration order is not printed), nil maps and make with a size hint.
package main

func main() {
	m := map[string]int{"a": 1, "b": 2}
	m["c"] = 3
	println(len(m), m["a"], m["zz"])
	v, ok := m["b"]
	println(v, ok)
	_, ok = m["nope"]
	println(ok)
	delete(m, "a")
	println(len(m))
	clear(m)
	println(len(m))
	mm := map[int]int{1: 10, 2: 20, 3: 30}
	total := 0
	for k, v := range mm {
		total += k * v
	}
	println(total)
	var nm map[string]int
	println(nm == nil, len(nm))
	nm2 := make(map[int]bool, 8)
	nm2[1] = true
	println(len(nm2))
	type key struct{ a, b int }
	km := map[key]string{{1, 2}: "x"}
	println(km[key{1, 2}])
}
