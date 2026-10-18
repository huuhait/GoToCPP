// phase: 05

// Go's %f of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%f]\n", inf)
	fmt.Printf("[%+f]\n", inf)
	fmt.Printf("[%-f]\n", inf)
	fmt.Printf("[% f]\n", inf)
	fmt.Printf("[%0f]\n", inf)
	fmt.Printf("[%#f]\n", inf)
	fmt.Printf("[%10f]\n", inf)
	fmt.Printf("[%-10f]\n", inf)
	fmt.Printf("[%010f]\n", inf)
	fmt.Printf("[%+010f]\n", inf)
	fmt.Printf("[%10.3f]\n", inf)
	fmt.Printf("[%.0f]\n", inf)
	fmt.Printf("[%.1f]\n", inf)
	fmt.Printf("[%.3f]\n", inf)
	fmt.Printf("[%.10f]\n", inf)
	fmt.Printf("[%#.0f]\n", inf)
	fmt.Printf("[%#10.3f]\n", inf)
	fmt.Printf("[%+.2f]\n", inf)
	fmt.Printf("[% 10.2f]\n", inf)
	fmt.Printf("[%-#10.2f]\n", inf)
	fmt.Printf("[%20.15f]\n", inf)
	fmt.Printf("[%.20f]\n", inf)
	fmt.Printf("[%0-10f]\n", inf)
	fmt.Printf("[%f]\n", ninf)
	fmt.Printf("[%+f]\n", ninf)
	fmt.Printf("[%-f]\n", ninf)
	fmt.Printf("[% f]\n", ninf)
	fmt.Printf("[%0f]\n", ninf)
	fmt.Printf("[%#f]\n", ninf)
	fmt.Printf("[%10f]\n", ninf)
	fmt.Printf("[%-10f]\n", ninf)
	fmt.Printf("[%010f]\n", ninf)
	fmt.Printf("[%+010f]\n", ninf)
	fmt.Printf("[%10.3f]\n", ninf)
	fmt.Printf("[%.0f]\n", ninf)
	fmt.Printf("[%.1f]\n", ninf)
	fmt.Printf("[%.3f]\n", ninf)
	fmt.Printf("[%.10f]\n", ninf)
	fmt.Printf("[%#.0f]\n", ninf)
	fmt.Printf("[%#10.3f]\n", ninf)
	fmt.Printf("[%+.2f]\n", ninf)
	fmt.Printf("[% 10.2f]\n", ninf)
	fmt.Printf("[%-#10.2f]\n", ninf)
	fmt.Printf("[%20.15f]\n", ninf)
	fmt.Printf("[%.20f]\n", ninf)
	fmt.Printf("[%0-10f]\n", ninf)
	fmt.Printf("[%f]\n", nan)
	fmt.Printf("[%+f]\n", nan)
	fmt.Printf("[%-f]\n", nan)
	fmt.Printf("[% f]\n", nan)
	fmt.Printf("[%0f]\n", nan)
	fmt.Printf("[%#f]\n", nan)
	fmt.Printf("[%10f]\n", nan)
	fmt.Printf("[%-10f]\n", nan)
	fmt.Printf("[%010f]\n", nan)
	fmt.Printf("[%+010f]\n", nan)
	fmt.Printf("[%10.3f]\n", nan)
	fmt.Printf("[%.0f]\n", nan)
	fmt.Printf("[%.1f]\n", nan)
	fmt.Printf("[%.3f]\n", nan)
	fmt.Printf("[%.10f]\n", nan)
	fmt.Printf("[%#.0f]\n", nan)
	fmt.Printf("[%#10.3f]\n", nan)
	fmt.Printf("[%+.2f]\n", nan)
	fmt.Printf("[% 10.2f]\n", nan)
	fmt.Printf("[%-#10.2f]\n", nan)
	fmt.Printf("[%20.15f]\n", nan)
	fmt.Printf("[%.20f]\n", nan)
	fmt.Printf("[%0-10f]\n", nan)
	fmt.Printf("[%f]\n", nzero)
	fmt.Printf("[%+f]\n", nzero)
	fmt.Printf("[%-f]\n", nzero)
	fmt.Printf("[% f]\n", nzero)
	fmt.Printf("[%0f]\n", nzero)
	fmt.Printf("[%#f]\n", nzero)
	fmt.Printf("[%10f]\n", nzero)
	fmt.Printf("[%-10f]\n", nzero)
	fmt.Printf("[%010f]\n", nzero)
	fmt.Printf("[%+010f]\n", nzero)
	fmt.Printf("[%10.3f]\n", nzero)
	fmt.Printf("[%.0f]\n", nzero)
	fmt.Printf("[%.1f]\n", nzero)
	fmt.Printf("[%.3f]\n", nzero)
	fmt.Printf("[%.10f]\n", nzero)
	fmt.Printf("[%#.0f]\n", nzero)
	fmt.Printf("[%#10.3f]\n", nzero)
	fmt.Printf("[%+.2f]\n", nzero)
	fmt.Printf("[% 10.2f]\n", nzero)
	fmt.Printf("[%-#10.2f]\n", nzero)
	fmt.Printf("[%20.15f]\n", nzero)
	fmt.Printf("[%.20f]\n", nzero)
	fmt.Printf("[%0-10f]\n", nzero)
	fmt.Printf("[%f]\n", 0.0)
	fmt.Printf("[%+f]\n", 0.0)
	fmt.Printf("[%-f]\n", 0.0)
	fmt.Printf("[% f]\n", 0.0)
	fmt.Printf("[%0f]\n", 0.0)
	fmt.Printf("[%#f]\n", 0.0)
	fmt.Printf("[%10f]\n", 0.0)
	fmt.Printf("[%-10f]\n", 0.0)
	fmt.Printf("[%010f]\n", 0.0)
	fmt.Printf("[%+010f]\n", 0.0)
	fmt.Printf("[%10.3f]\n", 0.0)
	fmt.Printf("[%.0f]\n", 0.0)
	fmt.Printf("[%.1f]\n", 0.0)
	fmt.Printf("[%.3f]\n", 0.0)
	fmt.Printf("[%.10f]\n", 0.0)
	fmt.Printf("[%#.0f]\n", 0.0)
	fmt.Printf("[%#10.3f]\n", 0.0)
	fmt.Printf("[%+.2f]\n", 0.0)
	fmt.Printf("[% 10.2f]\n", 0.0)
	fmt.Printf("[%-#10.2f]\n", 0.0)
	fmt.Printf("[%20.15f]\n", 0.0)
	fmt.Printf("[%.20f]\n", 0.0)
	fmt.Printf("[%0-10f]\n", 0.0)
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
// [-0.000000]
// [-0.000000]
// [-0.000000]
// [-0.000000]
// [-0.000000]
// [-0.000000]
// [ -0.000000]
// [-0.000000 ]
// [-00.000000]
// [-00.000000]
// [    -0.000]
// [-0]
// [-0.0]
// [-0.000]
// [-0.0000000000]
// [-0.]
// [    -0.000]
// [-0.00]
// [     -0.00]
// [-0.00     ]
// [  -0.000000000000000]
// [-0.00000000000000000000]
// [-0.000000 ]
// [0.000000]
// [+0.000000]
// [0.000000]
// [ 0.000000]
// [0.000000]
// [0.000000]
// [  0.000000]
// [0.000000  ]
// [000.000000]
// [+00.000000]
// [     0.000]
// [0]
// [0.0]
// [0.000]
// [0.0000000000]
// [0.]
// [     0.000]
// [+0.00]
// [      0.00]
// [0.00      ]
// [   0.000000000000000]
// [0.00000000000000000000]
// [0.000000  ]
