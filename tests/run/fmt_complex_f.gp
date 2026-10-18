// phase: 05

// Go's %f of complex128: both parts with the verb, the imaginary part always signed.
package main

import "fmt"

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	fmt.Printf("[%f]\n", complex(1, 2))
	fmt.Printf("[%+f]\n", complex(1, 2))
	fmt.Printf("[%-f]\n", complex(1, 2))
	fmt.Printf("[% f]\n", complex(1, 2))
	fmt.Printf("[%10f]\n", complex(1, 2))
	fmt.Printf("[%-10f]\n", complex(1, 2))
	fmt.Printf("[%010f]\n", complex(1, 2))
	fmt.Printf("[%.2f]\n", complex(1, 2))
	fmt.Printf("[%10.3f]\n", complex(1, 2))
	fmt.Printf("[%#f]\n", complex(1, 2))
	fmt.Printf("[%+.1f]\n", complex(1, 2))
	fmt.Printf("[%f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-f]\n", complex(-1.5, 0.25))
	fmt.Printf("[% f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-10f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%010f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%.2f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10.3f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%#f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+.1f]\n", complex(-1.5, 0.25))
	fmt.Printf("[%f]\n", complex(0, 0))
	fmt.Printf("[%+f]\n", complex(0, 0))
	fmt.Printf("[%-f]\n", complex(0, 0))
	fmt.Printf("[% f]\n", complex(0, 0))
	fmt.Printf("[%10f]\n", complex(0, 0))
	fmt.Printf("[%-10f]\n", complex(0, 0))
	fmt.Printf("[%010f]\n", complex(0, 0))
	fmt.Printf("[%.2f]\n", complex(0, 0))
	fmt.Printf("[%10.3f]\n", complex(0, 0))
	fmt.Printf("[%#f]\n", complex(0, 0))
	fmt.Printf("[%+.1f]\n", complex(0, 0))
	fmt.Printf("[%f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-f]\n", complex(1e21, 1e-7))
	fmt.Printf("[% f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-10f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%010f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%.2f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10.3f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%#f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+.1f]\n", complex(1e21, 1e-7))
	fmt.Printf("[%f]\n", complex(inf, nan))
	fmt.Printf("[%+f]\n", complex(inf, nan))
	fmt.Printf("[%-f]\n", complex(inf, nan))
	fmt.Printf("[% f]\n", complex(inf, nan))
	fmt.Printf("[%10f]\n", complex(inf, nan))
	fmt.Printf("[%-10f]\n", complex(inf, nan))
	fmt.Printf("[%010f]\n", complex(inf, nan))
	fmt.Printf("[%.2f]\n", complex(inf, nan))
	fmt.Printf("[%10.3f]\n", complex(inf, nan))
	fmt.Printf("[%#f]\n", complex(inf, nan))
	fmt.Printf("[%+.1f]\n", complex(inf, nan))
	fmt.Printf("[%f]\n", complex(2.675, -2.675))
	fmt.Printf("[%+f]\n", complex(2.675, -2.675))
	fmt.Printf("[%-f]\n", complex(2.675, -2.675))
	fmt.Printf("[% f]\n", complex(2.675, -2.675))
	fmt.Printf("[%10f]\n", complex(2.675, -2.675))
	fmt.Printf("[%-10f]\n", complex(2.675, -2.675))
	fmt.Printf("[%010f]\n", complex(2.675, -2.675))
	fmt.Printf("[%.2f]\n", complex(2.675, -2.675))
	fmt.Printf("[%10.3f]\n", complex(2.675, -2.675))
	fmt.Printf("[%#f]\n", complex(2.675, -2.675))
	fmt.Printf("[%+.1f]\n", complex(2.675, -2.675))
}

// Output:
// [(1.000000+2.000000i)]
// [(+1.000000+2.000000i)]
// [(1.000000+2.000000i)]
// [( 1.000000+2.000000i)]
// [(  1.000000 +2.000000i)]
// [(1.000000  +2.000000 i)]
// [(001.000000+02.000000i)]
// [(1.00+2.00i)]
// [(     1.000    +2.000i)]
// [(1.000000+2.000000i)]
// [(+1.0+2.0i)]
// [(-1.500000+0.250000i)]
// [(-1.500000+0.250000i)]
// [(-1.500000+0.250000i)]
// [(-1.500000+0.250000i)]
// [( -1.500000 +0.250000i)]
// [(-1.500000 +0.250000 i)]
// [(-01.500000+00.250000i)]
// [(-1.50+0.25i)]
// [(    -1.500    +0.250i)]
// [(-1.500000+0.250000i)]
// [(-1.5+0.2i)]
// [(0.000000+0.000000i)]
// [(+0.000000+0.000000i)]
// [(0.000000+0.000000i)]
// [( 0.000000+0.000000i)]
// [(  0.000000 +0.000000i)]
// [(0.000000  +0.000000 i)]
// [(000.000000+00.000000i)]
// [(0.00+0.00i)]
// [(     0.000    +0.000i)]
// [(0.000000+0.000000i)]
// [(+0.0+0.0i)]
// [(1000000000000000000000.000000+0.000000i)]
// [(+1000000000000000000000.000000+0.000000i)]
// [(1000000000000000000000.000000+0.000000i)]
// [( 1000000000000000000000.000000+0.000000i)]
// [(1000000000000000000000.000000 +0.000000i)]
// [(1000000000000000000000.000000+0.000000 i)]
// [(1000000000000000000000.000000+00.000000i)]
// [(1000000000000000000000.00+0.00i)]
// [(1000000000000000000000.000    +0.000i)]
// [(1000000000000000000000.000000+0.000000i)]
// [(+1000000000000000000000.0+0.0i)]
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
// [(2.675000-2.675000i)]
// [(+2.675000-2.675000i)]
// [(2.675000-2.675000i)]
// [( 2.675000-2.675000i)]
// [(  2.675000 -2.675000i)]
// [(2.675000  -2.675000 i)]
// [(002.675000-02.675000i)]
// [(2.67-2.67i)]
// [(     2.675    -2.675i)]
// [(2.675000-2.675000i)]
// [(+2.7-2.7i)]
