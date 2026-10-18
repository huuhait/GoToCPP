// phase: 05

// Go's %g of complex128: both parts with the verb, the imaginary part always signed.
package main

import "fmt"

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	fmt.Printf("[%g]\n", complex(1, 2))
	fmt.Printf("[%+g]\n", complex(1, 2))
	fmt.Printf("[%-g]\n", complex(1, 2))
	fmt.Printf("[% g]\n", complex(1, 2))
	fmt.Printf("[%10g]\n", complex(1, 2))
	fmt.Printf("[%-10g]\n", complex(1, 2))
	fmt.Printf("[%010g]\n", complex(1, 2))
	fmt.Printf("[%.2g]\n", complex(1, 2))
	fmt.Printf("[%10.3g]\n", complex(1, 2))
	fmt.Printf("[%#g]\n", complex(1, 2))
	fmt.Printf("[%+.1g]\n", complex(1, 2))
	fmt.Printf("[%g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-g]\n", complex(-1.5, 0.25))
	fmt.Printf("[% g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-10g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%010g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%.2g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10.3g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%#g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+.1g]\n", complex(-1.5, 0.25))
	fmt.Printf("[%g]\n", complex(0, 0))
	fmt.Printf("[%+g]\n", complex(0, 0))
	fmt.Printf("[%-g]\n", complex(0, 0))
	fmt.Printf("[% g]\n", complex(0, 0))
	fmt.Printf("[%10g]\n", complex(0, 0))
	fmt.Printf("[%-10g]\n", complex(0, 0))
	fmt.Printf("[%010g]\n", complex(0, 0))
	fmt.Printf("[%.2g]\n", complex(0, 0))
	fmt.Printf("[%10.3g]\n", complex(0, 0))
	fmt.Printf("[%#g]\n", complex(0, 0))
	fmt.Printf("[%+.1g]\n", complex(0, 0))
	fmt.Printf("[%g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-g]\n", complex(1e21, 1e-7))
	fmt.Printf("[% g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-10g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%010g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%.2g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10.3g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%#g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+.1g]\n", complex(1e21, 1e-7))
	fmt.Printf("[%g]\n", complex(inf, nan))
	fmt.Printf("[%+g]\n", complex(inf, nan))
	fmt.Printf("[%-g]\n", complex(inf, nan))
	fmt.Printf("[% g]\n", complex(inf, nan))
	fmt.Printf("[%10g]\n", complex(inf, nan))
	fmt.Printf("[%-10g]\n", complex(inf, nan))
	fmt.Printf("[%010g]\n", complex(inf, nan))
	fmt.Printf("[%.2g]\n", complex(inf, nan))
	fmt.Printf("[%10.3g]\n", complex(inf, nan))
	fmt.Printf("[%#g]\n", complex(inf, nan))
	fmt.Printf("[%+.1g]\n", complex(inf, nan))
	fmt.Printf("[%g]\n", complex(2.675, -2.675))
	fmt.Printf("[%+g]\n", complex(2.675, -2.675))
	fmt.Printf("[%-g]\n", complex(2.675, -2.675))
	fmt.Printf("[% g]\n", complex(2.675, -2.675))
	fmt.Printf("[%10g]\n", complex(2.675, -2.675))
	fmt.Printf("[%-10g]\n", complex(2.675, -2.675))
	fmt.Printf("[%010g]\n", complex(2.675, -2.675))
	fmt.Printf("[%.2g]\n", complex(2.675, -2.675))
	fmt.Printf("[%10.3g]\n", complex(2.675, -2.675))
	fmt.Printf("[%#g]\n", complex(2.675, -2.675))
	fmt.Printf("[%+.1g]\n", complex(2.675, -2.675))
}

// Output:
// [(1+2i)]
// [(+1+2i)]
// [(1+2i)]
// [( 1+2i)]
// [(         1        +2i)]
// [(1         +2        i)]
// [(0000000001+000000002i)]
// [(1+2i)]
// [(         1        +2i)]
// [(1.00000+2.00000i)]
// [(+1+2i)]
// [(-1.5+0.25i)]
// [(-1.5+0.25i)]
// [(-1.5+0.25i)]
// [(-1.5+0.25i)]
// [(      -1.5     +0.25i)]
// [(-1.5      +0.25     i)]
// [(-0000001.5+000000.25i)]
// [(-1.5+0.25i)]
// [(      -1.5     +0.25i)]
// [(-1.50000+0.250000i)]
// [(-2+0.2i)]
// [(0+0i)]
// [(+0+0i)]
// [(0+0i)]
// [( 0+0i)]
// [(         0        +0i)]
// [(0         +0        i)]
// [(0000000000+000000000i)]
// [(0+0i)]
// [(         0        +0i)]
// [(0.00000+0.00000i)]
// [(+0+0i)]
// [(1e+21+1e-07i)]
// [(+1e+21+1e-07i)]
// [(1e+21+1e-07i)]
// [( 1e+21+1e-07i)]
// [(     1e+21    +1e-07i)]
// [(1e+21     +1e-07    i)]
// [(000001e+21+00001e-07i)]
// [(1e+21+1e-07i)]
// [(     1e+21    +1e-07i)]
// [(1.00000e+21+1.00000e-07i)]
// [(+1e+21+1e-07i)]
// [(+Inf+NaNi)]
// [(+Inf+NaNi)]
// [(+Inf+NaNi)]
// [( Inf+NaNi)]
// [(      +Inf      +NaNi)]
// [(+Inf      +NaN      i)]
// [(      +Inf      +NaNi)]
// [(+Inf+NaNi)]
// [(      +Inf      +NaNi)]
// [(+Inf+NaNi)]
// [(+Inf+NaNi)]
// [(2.675-2.675i)]
// [(+2.675-2.675i)]
// [(2.675-2.675i)]
// [( 2.675-2.675i)]
// [(     2.675    -2.675i)]
// [(2.675     -2.675    i)]
// [(000002.675-00002.675i)]
// [(2.7-2.7i)]
// [(      2.67     -2.67i)]
// [(2.67500-2.67500i)]
// [(+3-3i)]
