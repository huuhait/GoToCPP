// phase: 05

// Go's %e of complex128: both parts with the verb, the imaginary part always signed.
package main

import "fmt"

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	fmt.Printf("[%e]\n", complex(1, 2))
	fmt.Printf("[%+e]\n", complex(1, 2))
	fmt.Printf("[%-e]\n", complex(1, 2))
	fmt.Printf("[% e]\n", complex(1, 2))
	fmt.Printf("[%10e]\n", complex(1, 2))
	fmt.Printf("[%-10e]\n", complex(1, 2))
	fmt.Printf("[%010e]\n", complex(1, 2))
	fmt.Printf("[%.2e]\n", complex(1, 2))
	fmt.Printf("[%10.3e]\n", complex(1, 2))
	fmt.Printf("[%#e]\n", complex(1, 2))
	fmt.Printf("[%+.1e]\n", complex(1, 2))
	fmt.Printf("[%e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-e]\n", complex(-1.5, 0.25))
	fmt.Printf("[% e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-10e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%010e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%.2e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10.3e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%#e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+.1e]\n", complex(-1.5, 0.25))
	fmt.Printf("[%e]\n", complex(0, 0))
	fmt.Printf("[%+e]\n", complex(0, 0))
	fmt.Printf("[%-e]\n", complex(0, 0))
	fmt.Printf("[% e]\n", complex(0, 0))
	fmt.Printf("[%10e]\n", complex(0, 0))
	fmt.Printf("[%-10e]\n", complex(0, 0))
	fmt.Printf("[%010e]\n", complex(0, 0))
	fmt.Printf("[%.2e]\n", complex(0, 0))
	fmt.Printf("[%10.3e]\n", complex(0, 0))
	fmt.Printf("[%#e]\n", complex(0, 0))
	fmt.Printf("[%+.1e]\n", complex(0, 0))
	fmt.Printf("[%e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-e]\n", complex(1e21, 1e-7))
	fmt.Printf("[% e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-10e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%010e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%.2e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10.3e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%#e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+.1e]\n", complex(1e21, 1e-7))
	fmt.Printf("[%e]\n", complex(inf, nan))
	fmt.Printf("[%+e]\n", complex(inf, nan))
	fmt.Printf("[%-e]\n", complex(inf, nan))
	fmt.Printf("[% e]\n", complex(inf, nan))
	fmt.Printf("[%10e]\n", complex(inf, nan))
	fmt.Printf("[%-10e]\n", complex(inf, nan))
	fmt.Printf("[%010e]\n", complex(inf, nan))
	fmt.Printf("[%.2e]\n", complex(inf, nan))
	fmt.Printf("[%10.3e]\n", complex(inf, nan))
	fmt.Printf("[%#e]\n", complex(inf, nan))
	fmt.Printf("[%+.1e]\n", complex(inf, nan))
	fmt.Printf("[%e]\n", complex(2.675, -2.675))
	fmt.Printf("[%+e]\n", complex(2.675, -2.675))
	fmt.Printf("[%-e]\n", complex(2.675, -2.675))
	fmt.Printf("[% e]\n", complex(2.675, -2.675))
	fmt.Printf("[%10e]\n", complex(2.675, -2.675))
	fmt.Printf("[%-10e]\n", complex(2.675, -2.675))
	fmt.Printf("[%010e]\n", complex(2.675, -2.675))
	fmt.Printf("[%.2e]\n", complex(2.675, -2.675))
	fmt.Printf("[%10.3e]\n", complex(2.675, -2.675))
	fmt.Printf("[%#e]\n", complex(2.675, -2.675))
	fmt.Printf("[%+.1e]\n", complex(2.675, -2.675))
}

// Output:
// [(1.000000e+00+2.000000e+00i)]
// [(+1.000000e+00+2.000000e+00i)]
// [(1.000000e+00+2.000000e+00i)]
// [( 1.000000e+00+2.000000e+00i)]
// [(1.000000e+00+2.000000e+00i)]
// [(1.000000e+00+2.000000e+00i)]
// [(1.000000e+00+2.000000e+00i)]
// [(1.00e+00+2.00e+00i)]
// [( 1.000e+00+2.000e+00i)]
// [(1.000000e+00+2.000000e+00i)]
// [(+1.0e+00+2.0e+00i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.50e+00+2.50e-01i)]
// [(-1.500e+00+2.500e-01i)]
// [(-1.500000e+00+2.500000e-01i)]
// [(-1.5e+00+2.5e-01i)]
// [(0.000000e+00+0.000000e+00i)]
// [(+0.000000e+00+0.000000e+00i)]
// [(0.000000e+00+0.000000e+00i)]
// [( 0.000000e+00+0.000000e+00i)]
// [(0.000000e+00+0.000000e+00i)]
// [(0.000000e+00+0.000000e+00i)]
// [(0.000000e+00+0.000000e+00i)]
// [(0.00e+00+0.00e+00i)]
// [( 0.000e+00+0.000e+00i)]
// [(0.000000e+00+0.000000e+00i)]
// [(+0.0e+00+0.0e+00i)]
// [(1.000000e+21+1.000000e-07i)]
// [(+1.000000e+21+1.000000e-07i)]
// [(1.000000e+21+1.000000e-07i)]
// [( 1.000000e+21+1.000000e-07i)]
// [(1.000000e+21+1.000000e-07i)]
// [(1.000000e+21+1.000000e-07i)]
// [(1.000000e+21+1.000000e-07i)]
// [(1.00e+21+1.00e-07i)]
// [( 1.000e+21+1.000e-07i)]
// [(1.000000e+21+1.000000e-07i)]
// [(+1.0e+21+1.0e-07i)]
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
// [(2.675000e+00-2.675000e+00i)]
// [(+2.675000e+00-2.675000e+00i)]
// [(2.675000e+00-2.675000e+00i)]
// [( 2.675000e+00-2.675000e+00i)]
// [(2.675000e+00-2.675000e+00i)]
// [(2.675000e+00-2.675000e+00i)]
// [(2.675000e+00-2.675000e+00i)]
// [(2.67e+00-2.67e+00i)]
// [( 2.675e+00-2.675e+00i)]
// [(2.675000e+00-2.675000e+00i)]
// [(+2.7e+00-2.7e+00i)]
