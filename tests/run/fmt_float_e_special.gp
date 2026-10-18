// phase: 05

// Go's %e of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	fmt.Printf("[%e]\n", inf)
	fmt.Printf("[%+e]\n", inf)
	fmt.Printf("[%-e]\n", inf)
	fmt.Printf("[% e]\n", inf)
	fmt.Printf("[%0e]\n", inf)
	fmt.Printf("[%#e]\n", inf)
	fmt.Printf("[%10e]\n", inf)
	fmt.Printf("[%-10e]\n", inf)
	fmt.Printf("[%010e]\n", inf)
	fmt.Printf("[%+010e]\n", inf)
	fmt.Printf("[%10.3e]\n", inf)
	fmt.Printf("[%.0e]\n", inf)
	fmt.Printf("[%.1e]\n", inf)
	fmt.Printf("[%.3e]\n", inf)
	fmt.Printf("[%.10e]\n", inf)
	fmt.Printf("[%#.0e]\n", inf)
	fmt.Printf("[%#10.3e]\n", inf)
	fmt.Printf("[%+.2e]\n", inf)
	fmt.Printf("[% 10.2e]\n", inf)
	fmt.Printf("[%-#10.2e]\n", inf)
	fmt.Printf("[%20.15e]\n", inf)
	fmt.Printf("[%.20e]\n", inf)
	fmt.Printf("[%0-10e]\n", inf)
	fmt.Printf("[%e]\n", ninf)
	fmt.Printf("[%+e]\n", ninf)
	fmt.Printf("[%-e]\n", ninf)
	fmt.Printf("[% e]\n", ninf)
	fmt.Printf("[%0e]\n", ninf)
	fmt.Printf("[%#e]\n", ninf)
	fmt.Printf("[%10e]\n", ninf)
	fmt.Printf("[%-10e]\n", ninf)
	fmt.Printf("[%010e]\n", ninf)
	fmt.Printf("[%+010e]\n", ninf)
	fmt.Printf("[%10.3e]\n", ninf)
	fmt.Printf("[%.0e]\n", ninf)
	fmt.Printf("[%.1e]\n", ninf)
	fmt.Printf("[%.3e]\n", ninf)
	fmt.Printf("[%.10e]\n", ninf)
	fmt.Printf("[%#.0e]\n", ninf)
	fmt.Printf("[%#10.3e]\n", ninf)
	fmt.Printf("[%+.2e]\n", ninf)
	fmt.Printf("[% 10.2e]\n", ninf)
	fmt.Printf("[%-#10.2e]\n", ninf)
	fmt.Printf("[%20.15e]\n", ninf)
	fmt.Printf("[%.20e]\n", ninf)
	fmt.Printf("[%0-10e]\n", ninf)
	fmt.Printf("[%e]\n", nan)
	fmt.Printf("[%+e]\n", nan)
	fmt.Printf("[%-e]\n", nan)
	fmt.Printf("[% e]\n", nan)
	fmt.Printf("[%0e]\n", nan)
	fmt.Printf("[%#e]\n", nan)
	fmt.Printf("[%10e]\n", nan)
	fmt.Printf("[%-10e]\n", nan)
	fmt.Printf("[%010e]\n", nan)
	fmt.Printf("[%+010e]\n", nan)
	fmt.Printf("[%10.3e]\n", nan)
	fmt.Printf("[%.0e]\n", nan)
	fmt.Printf("[%.1e]\n", nan)
	fmt.Printf("[%.3e]\n", nan)
	fmt.Printf("[%.10e]\n", nan)
	fmt.Printf("[%#.0e]\n", nan)
	fmt.Printf("[%#10.3e]\n", nan)
	fmt.Printf("[%+.2e]\n", nan)
	fmt.Printf("[% 10.2e]\n", nan)
	fmt.Printf("[%-#10.2e]\n", nan)
	fmt.Printf("[%20.15e]\n", nan)
	fmt.Printf("[%.20e]\n", nan)
	fmt.Printf("[%0-10e]\n", nan)
	fmt.Printf("[%e]\n", nzero)
	fmt.Printf("[%+e]\n", nzero)
	fmt.Printf("[%-e]\n", nzero)
	fmt.Printf("[% e]\n", nzero)
	fmt.Printf("[%0e]\n", nzero)
	fmt.Printf("[%#e]\n", nzero)
	fmt.Printf("[%10e]\n", nzero)
	fmt.Printf("[%-10e]\n", nzero)
	fmt.Printf("[%010e]\n", nzero)
	fmt.Printf("[%+010e]\n", nzero)
	fmt.Printf("[%10.3e]\n", nzero)
	fmt.Printf("[%.0e]\n", nzero)
	fmt.Printf("[%.1e]\n", nzero)
	fmt.Printf("[%.3e]\n", nzero)
	fmt.Printf("[%.10e]\n", nzero)
	fmt.Printf("[%#.0e]\n", nzero)
	fmt.Printf("[%#10.3e]\n", nzero)
	fmt.Printf("[%+.2e]\n", nzero)
	fmt.Printf("[% 10.2e]\n", nzero)
	fmt.Printf("[%-#10.2e]\n", nzero)
	fmt.Printf("[%20.15e]\n", nzero)
	fmt.Printf("[%.20e]\n", nzero)
	fmt.Printf("[%0-10e]\n", nzero)
	fmt.Printf("[%e]\n", 0.0)
	fmt.Printf("[%+e]\n", 0.0)
	fmt.Printf("[%-e]\n", 0.0)
	fmt.Printf("[% e]\n", 0.0)
	fmt.Printf("[%0e]\n", 0.0)
	fmt.Printf("[%#e]\n", 0.0)
	fmt.Printf("[%10e]\n", 0.0)
	fmt.Printf("[%-10e]\n", 0.0)
	fmt.Printf("[%010e]\n", 0.0)
	fmt.Printf("[%+010e]\n", 0.0)
	fmt.Printf("[%10.3e]\n", 0.0)
	fmt.Printf("[%.0e]\n", 0.0)
	fmt.Printf("[%.1e]\n", 0.0)
	fmt.Printf("[%.3e]\n", 0.0)
	fmt.Printf("[%.10e]\n", 0.0)
	fmt.Printf("[%#.0e]\n", 0.0)
	fmt.Printf("[%#10.3e]\n", 0.0)
	fmt.Printf("[%+.2e]\n", 0.0)
	fmt.Printf("[% 10.2e]\n", 0.0)
	fmt.Printf("[%-#10.2e]\n", 0.0)
	fmt.Printf("[%20.15e]\n", 0.0)
	fmt.Printf("[%.20e]\n", 0.0)
	fmt.Printf("[%0-10e]\n", 0.0)
}

// Output:
// [+Inf]
// [+Inf]
// [+Inf]
// [ Inf]
// [+Inf]
// [+Inf]
// [      +Inf]
// [+Inf      ]
// [      +Inf]
// [      +Inf]
// [      +Inf]
// [+Inf]
// [+Inf]
// [+Inf]
// [+Inf]
// [+Inf]
// [      +Inf]
// [+Inf]
// [       Inf]
// [+Inf      ]
// [                +Inf]
// [+Inf]
// [+Inf      ]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [      -Inf]
// [-Inf      ]
// [      -Inf]
// [      -Inf]
// [      -Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [      -Inf]
// [-Inf]
// [      -Inf]
// [-Inf      ]
// [                -Inf]
// [-Inf]
// [-Inf      ]
// [NaN]
// [+NaN]
// [NaN]
// [ NaN]
// [NaN]
// [NaN]
// [       NaN]
// [NaN       ]
// [       NaN]
// [      +NaN]
// [       NaN]
// [NaN]
// [NaN]
// [NaN]
// [NaN]
// [NaN]
// [       NaN]
// [+NaN]
// [       NaN]
// [NaN       ]
// [                 NaN]
// [NaN]
// [NaN       ]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000000e+00]
// [-0.000e+00]
// [-0e+00]
// [-0.0e+00]
// [-0.000e+00]
// [-0.0000000000e+00]
// [-0.e+00]
// [-0.000e+00]
// [-0.00e+00]
// [ -0.00e+00]
// [-0.00e+00 ]
// [-0.000000000000000e+00]
// [-0.00000000000000000000e+00]
// [-0.000000e+00]
// [0.000000e+00]
// [+0.000000e+00]
// [0.000000e+00]
// [ 0.000000e+00]
// [0.000000e+00]
// [0.000000e+00]
// [0.000000e+00]
// [0.000000e+00]
// [0.000000e+00]
// [+0.000000e+00]
// [ 0.000e+00]
// [0e+00]
// [0.0e+00]
// [0.000e+00]
// [0.0000000000e+00]
// [0.e+00]
// [ 0.000e+00]
// [+0.00e+00]
// [  0.00e+00]
// [0.00e+00  ]
// [0.000000000000000e+00]
// [0.00000000000000000000e+00]
// [0.000000e+00]
