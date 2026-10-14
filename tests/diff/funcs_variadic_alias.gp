// phase: 03

package main

import "fmt"

func set0(xs ...int) {
	if len(xs) > 0 {
		xs[0] = -1
	}
}

func grow(xs ...int) []int {
	return append(xs, 99)
}

func pairs(kv ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

func main() {
	s := []int{1, 2, 3}
	set0(s...)
	set0(4, 5)
	fmt.Println(s[0], len(s))

	t := make([]int, 2, 10)
	u := grow(t...)
	u[0] = 7
	fmt.Println(t[0], len(u), u[2], cap(u) >= 3)

	full := []int{1, 2}
	v := grow(full[:2:2]...)
	v[0] = 8
	fmt.Println(full[0], v[0], v[2])

	m := pairs("a", "1", "b", "2", "odd")
	fmt.Println(len(m), m["a"], m["b"])
	none := pairs()
	fmt.Println(len(none))
}
