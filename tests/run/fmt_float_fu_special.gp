// phase: 05

// Go's %F of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%F]\n", inf)
	fmt.Printf("[%+F]\n", inf)
	fmt.Printf("[%-F]\n", inf)
	fmt.Printf("[% F]\n", inf)
	fmt.Printf("[%0F]\n", inf)
	fmt.Printf("[%#F]\n", inf)
	fmt.Printf("[%10F]\n", inf)
	fmt.Printf("[%-10F]\n", inf)
	fmt.Printf("[%010F]\n", inf)
	fmt.Printf("[%+010F]\n", inf)
	fmt.Printf("[%10.3F]\n", inf)
	fmt.Printf("[%.0F]\n", inf)
	fmt.Printf("[%.1F]\n", inf)
	fmt.Printf("[%.3F]\n", inf)
	fmt.Printf("[%.10F]\n", inf)
	fmt.Printf("[%#.0F]\n", inf)
	fmt.Printf("[%#10.3F]\n", inf)
	fmt.Printf("[%+.2F]\n", inf)
	fmt.Printf("[% 10.2F]\n", inf)
	fmt.Printf("[%-#10.2F]\n", inf)
	fmt.Printf("[%20.15F]\n", inf)
	fmt.Printf("[%.20F]\n", inf)
	fmt.Printf("[%0-10F]\n", inf)
	fmt.Printf("[%F]\n", ninf)
	fmt.Printf("[%+F]\n", ninf)
	fmt.Printf("[%-F]\n", ninf)
	fmt.Printf("[% F]\n", ninf)
	fmt.Printf("[%0F]\n", ninf)
	fmt.Printf("[%#F]\n", ninf)
	fmt.Printf("[%10F]\n", ninf)
	fmt.Printf("[%-10F]\n", ninf)
	fmt.Printf("[%010F]\n", ninf)
	fmt.Printf("[%+010F]\n", ninf)
	fmt.Printf("[%10.3F]\n", ninf)
	fmt.Printf("[%.0F]\n", ninf)
	fmt.Printf("[%.1F]\n", ninf)
	fmt.Printf("[%.3F]\n", ninf)
	fmt.Printf("[%.10F]\n", ninf)
	fmt.Printf("[%#.0F]\n", ninf)
	fmt.Printf("[%#10.3F]\n", ninf)
	fmt.Printf("[%+.2F]\n", ninf)
	fmt.Printf("[% 10.2F]\n", ninf)
	fmt.Printf("[%-#10.2F]\n", ninf)
	fmt.Printf("[%20.15F]\n", ninf)
	fmt.Printf("[%.20F]\n", ninf)
	fmt.Printf("[%0-10F]\n", ninf)
	fmt.Printf("[%F]\n", nan)
	fmt.Printf("[%+F]\n", nan)
	fmt.Printf("[%-F]\n", nan)
	fmt.Printf("[% F]\n", nan)
	fmt.Printf("[%0F]\n", nan)
	fmt.Printf("[%#F]\n", nan)
	fmt.Printf("[%10F]\n", nan)
	fmt.Printf("[%-10F]\n", nan)
	fmt.Printf("[%010F]\n", nan)
	fmt.Printf("[%+010F]\n", nan)
	fmt.Printf("[%10.3F]\n", nan)
	fmt.Printf("[%.0F]\n", nan)
	fmt.Printf("[%.1F]\n", nan)
	fmt.Printf("[%.3F]\n", nan)
	fmt.Printf("[%.10F]\n", nan)
	fmt.Printf("[%#.0F]\n", nan)
	fmt.Printf("[%#10.3F]\n", nan)
	fmt.Printf("[%+.2F]\n", nan)
	fmt.Printf("[% 10.2F]\n", nan)
	fmt.Printf("[%-#10.2F]\n", nan)
	fmt.Printf("[%20.15F]\n", nan)
	fmt.Printf("[%.20F]\n", nan)
	fmt.Printf("[%0-10F]\n", nan)
	fmt.Printf("[%F]\n", nzero)
	fmt.Printf("[%+F]\n", nzero)
	fmt.Printf("[%-F]\n", nzero)
	fmt.Printf("[% F]\n", nzero)
	fmt.Printf("[%0F]\n", nzero)
	fmt.Printf("[%#F]\n", nzero)
	fmt.Printf("[%10F]\n", nzero)
	fmt.Printf("[%-10F]\n", nzero)
	fmt.Printf("[%010F]\n", nzero)
	fmt.Printf("[%+010F]\n", nzero)
	fmt.Printf("[%10.3F]\n", nzero)
	fmt.Printf("[%.0F]\n", nzero)
	fmt.Printf("[%.1F]\n", nzero)
	fmt.Printf("[%.3F]\n", nzero)
	fmt.Printf("[%.10F]\n", nzero)
	fmt.Printf("[%#.0F]\n", nzero)
	fmt.Printf("[%#10.3F]\n", nzero)
	fmt.Printf("[%+.2F]\n", nzero)
	fmt.Printf("[% 10.2F]\n", nzero)
	fmt.Printf("[%-#10.2F]\n", nzero)
	fmt.Printf("[%20.15F]\n", nzero)
	fmt.Printf("[%.20F]\n", nzero)
	fmt.Printf("[%0-10F]\n", nzero)
	fmt.Printf("[%F]\n", 0.0)
	fmt.Printf("[%+F]\n", 0.0)
	fmt.Printf("[%-F]\n", 0.0)
	fmt.Printf("[% F]\n", 0.0)
	fmt.Printf("[%0F]\n", 0.0)
	fmt.Printf("[%#F]\n", 0.0)
	fmt.Printf("[%10F]\n", 0.0)
	fmt.Printf("[%-10F]\n", 0.0)
	fmt.Printf("[%010F]\n", 0.0)
	fmt.Printf("[%+010F]\n", 0.0)
	fmt.Printf("[%10.3F]\n", 0.0)
	fmt.Printf("[%.0F]\n", 0.0)
	fmt.Printf("[%.1F]\n", 0.0)
	fmt.Printf("[%.3F]\n", 0.0)
	fmt.Printf("[%.10F]\n", 0.0)
	fmt.Printf("[%#.0F]\n", 0.0)
	fmt.Printf("[%#10.3F]\n", 0.0)
	fmt.Printf("[%+.2F]\n", 0.0)
	fmt.Printf("[% 10.2F]\n", 0.0)
	fmt.Printf("[%-#10.2F]\n", 0.0)
	fmt.Printf("[%20.15F]\n", 0.0)
	fmt.Printf("[%.20F]\n", 0.0)
	fmt.Printf("[%0-10F]\n", 0.0)
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
