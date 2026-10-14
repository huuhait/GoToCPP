// User names that start with gp_ get a _ suffix, so they can't clash with the
// generated names (the temporary gp_t0 here).
package main

import "fmt"

var gp_count = 1

func gp_t0() (int, int) {
	return 1, 2
}

func main() {
	gp_t0_ := 10
	gp_x, gp_y := gp_t0()
	a, b := gp_t0()
	fmt.Println(gp_count, gp_t0_, gp_x, gp_y, a, b)
	{
		gp_count := gp_count + 1
		gp_x := gp_x
		fmt.Println(gp_count, gp_x)
	}
}
