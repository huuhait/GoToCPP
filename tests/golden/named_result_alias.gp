// phase: 05

// The C++ of a return in a function with a named result and a defer when an inner variable has the
// result's name: the return stores through the alias gp_res_i (defer.go: resultAliases), the
// boxed result through its box, and a result that nothing hides keeps its name.
package main

func plain(xs []int) (i int, n int) {
	defer func() {}()
	for _, x := range xs {
		n += x
	}
	return n, 0
}

func hidden(xs []int, v int) (i int, p any) {
	defer func() { p = recover() }()
	for i, x := range xs {
		if x == v {
			return i, nil
		}
	}
	return -1, nil
}

func boxed() (i int) {
	defer func() { i++ }()
	{
		i := 7
		return i
	}
}

func main() {
	plain(nil)
	hidden(nil, 1)
	boxed()
}
