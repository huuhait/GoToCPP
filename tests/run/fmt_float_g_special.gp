// phase: 05

// Go's %g of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%g]\n", inf)
	fmt.Printf("[%+g]\n", inf)
	fmt.Printf("[%-g]\n", inf)
	fmt.Printf("[% g]\n", inf)
	fmt.Printf("[%0g]\n", inf)
	fmt.Printf("[%#g]\n", inf)
	fmt.Printf("[%10g]\n", inf)
	fmt.Printf("[%-10g]\n", inf)
	fmt.Printf("[%010g]\n", inf)
	fmt.Printf("[%+010g]\n", inf)
	fmt.Printf("[%10.3g]\n", inf)
	fmt.Printf("[%.0g]\n", inf)
	fmt.Printf("[%.1g]\n", inf)
	fmt.Printf("[%.3g]\n", inf)
	fmt.Printf("[%.10g]\n", inf)
	fmt.Printf("[%#.0g]\n", inf)
	fmt.Printf("[%#10.3g]\n", inf)
	fmt.Printf("[%+.2g]\n", inf)
	fmt.Printf("[% 10.2g]\n", inf)
	fmt.Printf("[%-#10.2g]\n", inf)
	fmt.Printf("[%20.15g]\n", inf)
	fmt.Printf("[%.20g]\n", inf)
	fmt.Printf("[%0-10g]\n", inf)
	fmt.Printf("[%g]\n", ninf)
	fmt.Printf("[%+g]\n", ninf)
	fmt.Printf("[%-g]\n", ninf)
	fmt.Printf("[% g]\n", ninf)
	fmt.Printf("[%0g]\n", ninf)
	fmt.Printf("[%#g]\n", ninf)
	fmt.Printf("[%10g]\n", ninf)
	fmt.Printf("[%-10g]\n", ninf)
	fmt.Printf("[%010g]\n", ninf)
	fmt.Printf("[%+010g]\n", ninf)
	fmt.Printf("[%10.3g]\n", ninf)
	fmt.Printf("[%.0g]\n", ninf)
	fmt.Printf("[%.1g]\n", ninf)
	fmt.Printf("[%.3g]\n", ninf)
	fmt.Printf("[%.10g]\n", ninf)
	fmt.Printf("[%#.0g]\n", ninf)
	fmt.Printf("[%#10.3g]\n", ninf)
	fmt.Printf("[%+.2g]\n", ninf)
	fmt.Printf("[% 10.2g]\n", ninf)
	fmt.Printf("[%-#10.2g]\n", ninf)
	fmt.Printf("[%20.15g]\n", ninf)
	fmt.Printf("[%.20g]\n", ninf)
	fmt.Printf("[%0-10g]\n", ninf)
	fmt.Printf("[%g]\n", nan)
	fmt.Printf("[%+g]\n", nan)
	fmt.Printf("[%-g]\n", nan)
	fmt.Printf("[% g]\n", nan)
	fmt.Printf("[%0g]\n", nan)
	fmt.Printf("[%#g]\n", nan)
	fmt.Printf("[%10g]\n", nan)
	fmt.Printf("[%-10g]\n", nan)
	fmt.Printf("[%010g]\n", nan)
	fmt.Printf("[%+010g]\n", nan)
	fmt.Printf("[%10.3g]\n", nan)
	fmt.Printf("[%.0g]\n", nan)
	fmt.Printf("[%.1g]\n", nan)
	fmt.Printf("[%.3g]\n", nan)
	fmt.Printf("[%.10g]\n", nan)
	fmt.Printf("[%#.0g]\n", nan)
	fmt.Printf("[%#10.3g]\n", nan)
	fmt.Printf("[%+.2g]\n", nan)
	fmt.Printf("[% 10.2g]\n", nan)
	fmt.Printf("[%-#10.2g]\n", nan)
	fmt.Printf("[%20.15g]\n", nan)
	fmt.Printf("[%.20g]\n", nan)
	fmt.Printf("[%0-10g]\n", nan)
	fmt.Printf("[%g]\n", nzero)
	fmt.Printf("[%+g]\n", nzero)
	fmt.Printf("[%-g]\n", nzero)
	fmt.Printf("[% g]\n", nzero)
	fmt.Printf("[%0g]\n", nzero)
	fmt.Printf("[%#g]\n", nzero)
	fmt.Printf("[%10g]\n", nzero)
	fmt.Printf("[%-10g]\n", nzero)
	fmt.Printf("[%010g]\n", nzero)
	fmt.Printf("[%+010g]\n", nzero)
	fmt.Printf("[%10.3g]\n", nzero)
	fmt.Printf("[%.0g]\n", nzero)
	fmt.Printf("[%.1g]\n", nzero)
	fmt.Printf("[%.3g]\n", nzero)
	fmt.Printf("[%.10g]\n", nzero)
	fmt.Printf("[%#.0g]\n", nzero)
	fmt.Printf("[%#10.3g]\n", nzero)
	fmt.Printf("[%+.2g]\n", nzero)
	fmt.Printf("[% 10.2g]\n", nzero)
	fmt.Printf("[%-#10.2g]\n", nzero)
	fmt.Printf("[%20.15g]\n", nzero)
	fmt.Printf("[%.20g]\n", nzero)
	fmt.Printf("[%0-10g]\n", nzero)
	fmt.Printf("[%g]\n", 0.0)
	fmt.Printf("[%+g]\n", 0.0)
	fmt.Printf("[%-g]\n", 0.0)
	fmt.Printf("[% g]\n", 0.0)
	fmt.Printf("[%0g]\n", 0.0)
	fmt.Printf("[%#g]\n", 0.0)
	fmt.Printf("[%10g]\n", 0.0)
	fmt.Printf("[%-10g]\n", 0.0)
	fmt.Printf("[%010g]\n", 0.0)
	fmt.Printf("[%+010g]\n", 0.0)
	fmt.Printf("[%10.3g]\n", 0.0)
	fmt.Printf("[%.0g]\n", 0.0)
	fmt.Printf("[%.1g]\n", 0.0)
	fmt.Printf("[%.3g]\n", 0.0)
	fmt.Printf("[%.10g]\n", 0.0)
	fmt.Printf("[%#.0g]\n", 0.0)
	fmt.Printf("[%#10.3g]\n", 0.0)
	fmt.Printf("[%+.2g]\n", 0.0)
	fmt.Printf("[% 10.2g]\n", 0.0)
	fmt.Printf("[%-#10.2g]\n", 0.0)
	fmt.Printf("[%20.15g]\n", 0.0)
	fmt.Printf("[%.20g]\n", 0.0)
	fmt.Printf("[%0-10g]\n", 0.0)
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
// [-0]
// [-0]
// [-0]
// [-0]
// [-0]
// [-0.00000]
// [        -0]
// [-0        ]
// [-000000000]
// [-000000000]
// [        -0]
// [-0]
// [-0]
// [-0]
// [-0]
// [-0.]
// [     -0.00]
// [-0]
// [        -0]
// [-0.0      ]
// [                  -0]
// [-0]
// [-0        ]
// [0]
// [+0]
// [0]
// [ 0]
// [0]
// [0.00000]
// [         0]
// [0         ]
// [0000000000]
// [+000000000]
// [         0]
// [0]
// [0]
// [0]
// [0]
// [0.]
// [      0.00]
// [+0]
// [         0]
// [0.0       ]
// [                   0]
// [0]
// [0         ]
