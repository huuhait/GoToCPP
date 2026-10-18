// phase: 05

// Go's %E of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%E]\n", inf)
	fmt.Printf("[%+E]\n", inf)
	fmt.Printf("[%-E]\n", inf)
	fmt.Printf("[% E]\n", inf)
	fmt.Printf("[%0E]\n", inf)
	fmt.Printf("[%#E]\n", inf)
	fmt.Printf("[%10E]\n", inf)
	fmt.Printf("[%-10E]\n", inf)
	fmt.Printf("[%010E]\n", inf)
	fmt.Printf("[%+010E]\n", inf)
	fmt.Printf("[%10.3E]\n", inf)
	fmt.Printf("[%.0E]\n", inf)
	fmt.Printf("[%.1E]\n", inf)
	fmt.Printf("[%.3E]\n", inf)
	fmt.Printf("[%.10E]\n", inf)
	fmt.Printf("[%#.0E]\n", inf)
	fmt.Printf("[%#10.3E]\n", inf)
	fmt.Printf("[%+.2E]\n", inf)
	fmt.Printf("[% 10.2E]\n", inf)
	fmt.Printf("[%-#10.2E]\n", inf)
	fmt.Printf("[%20.15E]\n", inf)
	fmt.Printf("[%.20E]\n", inf)
	fmt.Printf("[%0-10E]\n", inf)
	fmt.Printf("[%E]\n", ninf)
	fmt.Printf("[%+E]\n", ninf)
	fmt.Printf("[%-E]\n", ninf)
	fmt.Printf("[% E]\n", ninf)
	fmt.Printf("[%0E]\n", ninf)
	fmt.Printf("[%#E]\n", ninf)
	fmt.Printf("[%10E]\n", ninf)
	fmt.Printf("[%-10E]\n", ninf)
	fmt.Printf("[%010E]\n", ninf)
	fmt.Printf("[%+010E]\n", ninf)
	fmt.Printf("[%10.3E]\n", ninf)
	fmt.Printf("[%.0E]\n", ninf)
	fmt.Printf("[%.1E]\n", ninf)
	fmt.Printf("[%.3E]\n", ninf)
	fmt.Printf("[%.10E]\n", ninf)
	fmt.Printf("[%#.0E]\n", ninf)
	fmt.Printf("[%#10.3E]\n", ninf)
	fmt.Printf("[%+.2E]\n", ninf)
	fmt.Printf("[% 10.2E]\n", ninf)
	fmt.Printf("[%-#10.2E]\n", ninf)
	fmt.Printf("[%20.15E]\n", ninf)
	fmt.Printf("[%.20E]\n", ninf)
	fmt.Printf("[%0-10E]\n", ninf)
	fmt.Printf("[%E]\n", nan)
	fmt.Printf("[%+E]\n", nan)
	fmt.Printf("[%-E]\n", nan)
	fmt.Printf("[% E]\n", nan)
	fmt.Printf("[%0E]\n", nan)
	fmt.Printf("[%#E]\n", nan)
	fmt.Printf("[%10E]\n", nan)
	fmt.Printf("[%-10E]\n", nan)
	fmt.Printf("[%010E]\n", nan)
	fmt.Printf("[%+010E]\n", nan)
	fmt.Printf("[%10.3E]\n", nan)
	fmt.Printf("[%.0E]\n", nan)
	fmt.Printf("[%.1E]\n", nan)
	fmt.Printf("[%.3E]\n", nan)
	fmt.Printf("[%.10E]\n", nan)
	fmt.Printf("[%#.0E]\n", nan)
	fmt.Printf("[%#10.3E]\n", nan)
	fmt.Printf("[%+.2E]\n", nan)
	fmt.Printf("[% 10.2E]\n", nan)
	fmt.Printf("[%-#10.2E]\n", nan)
	fmt.Printf("[%20.15E]\n", nan)
	fmt.Printf("[%.20E]\n", nan)
	fmt.Printf("[%0-10E]\n", nan)
	fmt.Printf("[%E]\n", nzero)
	fmt.Printf("[%+E]\n", nzero)
	fmt.Printf("[%-E]\n", nzero)
	fmt.Printf("[% E]\n", nzero)
	fmt.Printf("[%0E]\n", nzero)
	fmt.Printf("[%#E]\n", nzero)
	fmt.Printf("[%10E]\n", nzero)
	fmt.Printf("[%-10E]\n", nzero)
	fmt.Printf("[%010E]\n", nzero)
	fmt.Printf("[%+010E]\n", nzero)
	fmt.Printf("[%10.3E]\n", nzero)
	fmt.Printf("[%.0E]\n", nzero)
	fmt.Printf("[%.1E]\n", nzero)
	fmt.Printf("[%.3E]\n", nzero)
	fmt.Printf("[%.10E]\n", nzero)
	fmt.Printf("[%#.0E]\n", nzero)
	fmt.Printf("[%#10.3E]\n", nzero)
	fmt.Printf("[%+.2E]\n", nzero)
	fmt.Printf("[% 10.2E]\n", nzero)
	fmt.Printf("[%-#10.2E]\n", nzero)
	fmt.Printf("[%20.15E]\n", nzero)
	fmt.Printf("[%.20E]\n", nzero)
	fmt.Printf("[%0-10E]\n", nzero)
	fmt.Printf("[%E]\n", 0.0)
	fmt.Printf("[%+E]\n", 0.0)
	fmt.Printf("[%-E]\n", 0.0)
	fmt.Printf("[% E]\n", 0.0)
	fmt.Printf("[%0E]\n", 0.0)
	fmt.Printf("[%#E]\n", 0.0)
	fmt.Printf("[%10E]\n", 0.0)
	fmt.Printf("[%-10E]\n", 0.0)
	fmt.Printf("[%010E]\n", 0.0)
	fmt.Printf("[%+010E]\n", 0.0)
	fmt.Printf("[%10.3E]\n", 0.0)
	fmt.Printf("[%.0E]\n", 0.0)
	fmt.Printf("[%.1E]\n", 0.0)
	fmt.Printf("[%.3E]\n", 0.0)
	fmt.Printf("[%.10E]\n", 0.0)
	fmt.Printf("[%#.0E]\n", 0.0)
	fmt.Printf("[%#10.3E]\n", 0.0)
	fmt.Printf("[%+.2E]\n", 0.0)
	fmt.Printf("[% 10.2E]\n", 0.0)
	fmt.Printf("[%-#10.2E]\n", 0.0)
	fmt.Printf("[%20.15E]\n", 0.0)
	fmt.Printf("[%.20E]\n", 0.0)
	fmt.Printf("[%0-10E]\n", 0.0)
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
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000000E+00]
// [-0.000E+00]
// [-0E+00]
// [-0.0E+00]
// [-0.000E+00]
// [-0.0000000000E+00]
// [-0.E+00]
// [-0.000E+00]
// [-0.00E+00]
// [ -0.00E+00]
// [-0.00E+00 ]
// [-0.000000000000000E+00]
// [-0.00000000000000000000E+00]
// [-0.000000E+00]
// [0.000000E+00]
// [+0.000000E+00]
// [0.000000E+00]
// [ 0.000000E+00]
// [0.000000E+00]
// [0.000000E+00]
// [0.000000E+00]
// [0.000000E+00]
// [0.000000E+00]
// [+0.000000E+00]
// [ 0.000E+00]
// [0E+00]
// [0.0E+00]
// [0.000E+00]
// [0.0000000000E+00]
// [0.E+00]
// [ 0.000E+00]
// [+0.00E+00]
// [  0.00E+00]
// [0.00E+00  ]
// [0.000000000000000E+00]
// [0.00000000000000000000E+00]
// [0.000000E+00]
