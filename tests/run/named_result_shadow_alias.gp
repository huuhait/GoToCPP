// phase: 05

// The user's own names next to the hidden alias that gocpp writes for a result that an inner
// variable hides (gp_res_i): variables named like it, and like the result, in several scopes.
package main

import "fmt"

func alias(xs []int) (i int, gp_res_i int) {
	defer func() { gp_res_i++ }()
	for i, gp_res_i := range xs {
		if gp_res_i > 1 {
			return gp_res_i * 10, i
		}
	}
	return -1, -1
}

func more(xs []int) (i int, gp_res_i_ int, gp_r0 int) {
	defer func() { gp_r0++ }()
	for gp_res_i, i := range xs {
		var gp_res_i_ = gp_res_i + i
		return i, gp_res_i_, gp_res_i
	}
	return
}

func temps(i int) (gp_t0 int, gp_ret0 int) {
	defer func() { gp_ret0 *= 2 }()
	for gp_t0 := 0; gp_t0 < 3; gp_t0++ {
		gp_ret0 := gp_t0 + i
		if gp_ret0 > 3 {
			return gp_ret0, gp_t0
		}
	}
	return
}

func main() {
	fmt.Println(alias([]int{1, 2, 3}))
	fmt.Println(alias(nil))
	fmt.Println(more([]int{4, 5}))
	fmt.Println(more(nil))
	fmt.Println(temps(2))
	fmt.Println(temps(-9))
}

// Output:
// 20 2
// -1 0
// 4 4 1
// 0 0 1
// 4 4
// 0 0
