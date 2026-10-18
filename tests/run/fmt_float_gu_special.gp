// phase: 05

// Go's %G of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%G]\n", inf)
	fmt.Printf("[%+G]\n", inf)
	fmt.Printf("[%-G]\n", inf)
	fmt.Printf("[% G]\n", inf)
	fmt.Printf("[%0G]\n", inf)
	fmt.Printf("[%#G]\n", inf)
	fmt.Printf("[%10G]\n", inf)
	fmt.Printf("[%-10G]\n", inf)
	fmt.Printf("[%010G]\n", inf)
	fmt.Printf("[%+010G]\n", inf)
	fmt.Printf("[%10.3G]\n", inf)
	fmt.Printf("[%.0G]\n", inf)
	fmt.Printf("[%.1G]\n", inf)
	fmt.Printf("[%.3G]\n", inf)
	fmt.Printf("[%.10G]\n", inf)
	fmt.Printf("[%#.0G]\n", inf)
	fmt.Printf("[%#10.3G]\n", inf)
	fmt.Printf("[%+.2G]\n", inf)
	fmt.Printf("[% 10.2G]\n", inf)
	fmt.Printf("[%-#10.2G]\n", inf)
	fmt.Printf("[%20.15G]\n", inf)
	fmt.Printf("[%.20G]\n", inf)
	fmt.Printf("[%0-10G]\n", inf)
	fmt.Printf("[%G]\n", ninf)
	fmt.Printf("[%+G]\n", ninf)
	fmt.Printf("[%-G]\n", ninf)
	fmt.Printf("[% G]\n", ninf)
	fmt.Printf("[%0G]\n", ninf)
	fmt.Printf("[%#G]\n", ninf)
	fmt.Printf("[%10G]\n", ninf)
	fmt.Printf("[%-10G]\n", ninf)
	fmt.Printf("[%010G]\n", ninf)
	fmt.Printf("[%+010G]\n", ninf)
	fmt.Printf("[%10.3G]\n", ninf)
	fmt.Printf("[%.0G]\n", ninf)
	fmt.Printf("[%.1G]\n", ninf)
	fmt.Printf("[%.3G]\n", ninf)
	fmt.Printf("[%.10G]\n", ninf)
	fmt.Printf("[%#.0G]\n", ninf)
	fmt.Printf("[%#10.3G]\n", ninf)
	fmt.Printf("[%+.2G]\n", ninf)
	fmt.Printf("[% 10.2G]\n", ninf)
	fmt.Printf("[%-#10.2G]\n", ninf)
	fmt.Printf("[%20.15G]\n", ninf)
	fmt.Printf("[%.20G]\n", ninf)
	fmt.Printf("[%0-10G]\n", ninf)
	fmt.Printf("[%G]\n", nan)
	fmt.Printf("[%+G]\n", nan)
	fmt.Printf("[%-G]\n", nan)
	fmt.Printf("[% G]\n", nan)
	fmt.Printf("[%0G]\n", nan)
	fmt.Printf("[%#G]\n", nan)
	fmt.Printf("[%10G]\n", nan)
	fmt.Printf("[%-10G]\n", nan)
	fmt.Printf("[%010G]\n", nan)
	fmt.Printf("[%+010G]\n", nan)
	fmt.Printf("[%10.3G]\n", nan)
	fmt.Printf("[%.0G]\n", nan)
	fmt.Printf("[%.1G]\n", nan)
	fmt.Printf("[%.3G]\n", nan)
	fmt.Printf("[%.10G]\n", nan)
	fmt.Printf("[%#.0G]\n", nan)
	fmt.Printf("[%#10.3G]\n", nan)
	fmt.Printf("[%+.2G]\n", nan)
	fmt.Printf("[% 10.2G]\n", nan)
	fmt.Printf("[%-#10.2G]\n", nan)
	fmt.Printf("[%20.15G]\n", nan)
	fmt.Printf("[%.20G]\n", nan)
	fmt.Printf("[%0-10G]\n", nan)
	fmt.Printf("[%G]\n", nzero)
	fmt.Printf("[%+G]\n", nzero)
	fmt.Printf("[%-G]\n", nzero)
	fmt.Printf("[% G]\n", nzero)
	fmt.Printf("[%0G]\n", nzero)
	fmt.Printf("[%#G]\n", nzero)
	fmt.Printf("[%10G]\n", nzero)
	fmt.Printf("[%-10G]\n", nzero)
	fmt.Printf("[%010G]\n", nzero)
	fmt.Printf("[%+010G]\n", nzero)
	fmt.Printf("[%10.3G]\n", nzero)
	fmt.Printf("[%.0G]\n", nzero)
	fmt.Printf("[%.1G]\n", nzero)
	fmt.Printf("[%.3G]\n", nzero)
	fmt.Printf("[%.10G]\n", nzero)
	fmt.Printf("[%#.0G]\n", nzero)
	fmt.Printf("[%#10.3G]\n", nzero)
	fmt.Printf("[%+.2G]\n", nzero)
	fmt.Printf("[% 10.2G]\n", nzero)
	fmt.Printf("[%-#10.2G]\n", nzero)
	fmt.Printf("[%20.15G]\n", nzero)
	fmt.Printf("[%.20G]\n", nzero)
	fmt.Printf("[%0-10G]\n", nzero)
	fmt.Printf("[%G]\n", 0.0)
	fmt.Printf("[%+G]\n", 0.0)
	fmt.Printf("[%-G]\n", 0.0)
	fmt.Printf("[% G]\n", 0.0)
	fmt.Printf("[%0G]\n", 0.0)
	fmt.Printf("[%#G]\n", 0.0)
	fmt.Printf("[%10G]\n", 0.0)
	fmt.Printf("[%-10G]\n", 0.0)
	fmt.Printf("[%010G]\n", 0.0)
	fmt.Printf("[%+010G]\n", 0.0)
	fmt.Printf("[%10.3G]\n", 0.0)
	fmt.Printf("[%.0G]\n", 0.0)
	fmt.Printf("[%.1G]\n", 0.0)
	fmt.Printf("[%.3G]\n", 0.0)
	fmt.Printf("[%.10G]\n", 0.0)
	fmt.Printf("[%#.0G]\n", 0.0)
	fmt.Printf("[%#10.3G]\n", 0.0)
	fmt.Printf("[%+.2G]\n", 0.0)
	fmt.Printf("[% 10.2G]\n", 0.0)
	fmt.Printf("[%-#10.2G]\n", 0.0)
	fmt.Printf("[%20.15G]\n", 0.0)
	fmt.Printf("[%.20G]\n", 0.0)
	fmt.Printf("[%0-10G]\n", 0.0)
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
