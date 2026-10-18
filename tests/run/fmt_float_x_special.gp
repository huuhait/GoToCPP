// phase: 05

// Go's %x of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%x]\n", inf)
	fmt.Printf("[%+x]\n", inf)
	fmt.Printf("[%-x]\n", inf)
	fmt.Printf("[% x]\n", inf)
	fmt.Printf("[%0x]\n", inf)
	fmt.Printf("[%#x]\n", inf)
	fmt.Printf("[%30x]\n", inf)
	fmt.Printf("[%-30x]\n", inf)
	fmt.Printf("[%030x]\n", inf)
	fmt.Printf("[%+030x]\n", inf)
	fmt.Printf("[%.0x]\n", inf)
	fmt.Printf("[%.1x]\n", inf)
	fmt.Printf("[%.3x]\n", inf)
	fmt.Printf("[%.10x]\n", inf)
	fmt.Printf("[%#.3x]\n", inf)
	fmt.Printf("[%#30x]\n", inf)
	fmt.Printf("[%x]\n", ninf)
	fmt.Printf("[%+x]\n", ninf)
	fmt.Printf("[%-x]\n", ninf)
	fmt.Printf("[% x]\n", ninf)
	fmt.Printf("[%0x]\n", ninf)
	fmt.Printf("[%#x]\n", ninf)
	fmt.Printf("[%30x]\n", ninf)
	fmt.Printf("[%-30x]\n", ninf)
	fmt.Printf("[%030x]\n", ninf)
	fmt.Printf("[%+030x]\n", ninf)
	fmt.Printf("[%.0x]\n", ninf)
	fmt.Printf("[%.1x]\n", ninf)
	fmt.Printf("[%.3x]\n", ninf)
	fmt.Printf("[%.10x]\n", ninf)
	fmt.Printf("[%#.3x]\n", ninf)
	fmt.Printf("[%#30x]\n", ninf)
	fmt.Printf("[%x]\n", nan)
	fmt.Printf("[%+x]\n", nan)
	fmt.Printf("[%-x]\n", nan)
	fmt.Printf("[% x]\n", nan)
	fmt.Printf("[%0x]\n", nan)
	fmt.Printf("[%#x]\n", nan)
	fmt.Printf("[%30x]\n", nan)
	fmt.Printf("[%-30x]\n", nan)
	fmt.Printf("[%030x]\n", nan)
	fmt.Printf("[%+030x]\n", nan)
	fmt.Printf("[%.0x]\n", nan)
	fmt.Printf("[%.1x]\n", nan)
	fmt.Printf("[%.3x]\n", nan)
	fmt.Printf("[%.10x]\n", nan)
	fmt.Printf("[%#.3x]\n", nan)
	fmt.Printf("[%#30x]\n", nan)
	fmt.Printf("[%x]\n", nzero)
	fmt.Printf("[%+x]\n", nzero)
	fmt.Printf("[%-x]\n", nzero)
	fmt.Printf("[% x]\n", nzero)
	fmt.Printf("[%0x]\n", nzero)
	fmt.Printf("[%#x]\n", nzero)
	fmt.Printf("[%30x]\n", nzero)
	fmt.Printf("[%-30x]\n", nzero)
	fmt.Printf("[%030x]\n", nzero)
	fmt.Printf("[%+030x]\n", nzero)
	fmt.Printf("[%.0x]\n", nzero)
	fmt.Printf("[%.1x]\n", nzero)
	fmt.Printf("[%.3x]\n", nzero)
	fmt.Printf("[%.10x]\n", nzero)
	fmt.Printf("[%#.3x]\n", nzero)
	fmt.Printf("[%#30x]\n", nzero)
	fmt.Printf("[%x]\n", 0.0)
	fmt.Printf("[%+x]\n", 0.0)
	fmt.Printf("[%-x]\n", 0.0)
	fmt.Printf("[% x]\n", 0.0)
	fmt.Printf("[%0x]\n", 0.0)
	fmt.Printf("[%#x]\n", 0.0)
	fmt.Printf("[%30x]\n", 0.0)
	fmt.Printf("[%-30x]\n", 0.0)
	fmt.Printf("[%030x]\n", 0.0)
	fmt.Printf("[%+030x]\n", 0.0)
	fmt.Printf("[%.0x]\n", 0.0)
	fmt.Printf("[%.1x]\n", 0.0)
	fmt.Printf("[%.3x]\n", 0.0)
	fmt.Printf("[%.10x]\n", 0.0)
	fmt.Printf("[%#.3x]\n", 0.0)
	fmt.Printf("[%#30x]\n", 0.0)
}

// Output:
// [+Inf]
// [+Inf]
// [+Inf]
// [ Inf]
// [+Inf]
// [+Inf]
// [                          +Inf]
// [+Inf                          ]
// [                          +Inf]
// [                          +Inf]
// [+Inf]
// [+Inf]
// [+Inf]
// [+Inf]
// [+Inf]
// [                          +Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [                          -Inf]
// [-Inf                          ]
// [                          -Inf]
// [                          -Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [-Inf]
// [                          -Inf]
// [NaN]
// [+NaN]
// [NaN]
// [ NaN]
// [NaN]
// [NaN]
// [                           NaN]
// [NaN                           ]
// [                           NaN]
// [                          +NaN]
// [NaN]
// [NaN]
// [NaN]
// [NaN]
// [NaN]
// [                           NaN]
// [-0x0p+00]
// [-0x0p+00]
// [-0x0p+00]
// [-0x0p+00]
// [-0x0p+00]
// [-0x0.0000p+00]
// [                      -0x0p+00]
// [-0x0p+00                      ]
// [-00000000000000000000000x0p+00]
// [-00000000000000000000000x0p+00]
// [-0x0p+00]
// [-0x0.0p+00]
// [-0x0.000p+00]
// [-0x0.0000000000p+00]
// [-0x0.000p+00]
// [                 -0x0.0000p+00]
// [0x0p+00]
// [+0x0p+00]
// [0x0p+00]
// [ 0x0p+00]
// [0x0p+00]
// [0x0.0000p+00]
// [                       0x0p+00]
// [0x0p+00                       ]
// [000000000000000000000000x0p+00]
// [+00000000000000000000000x0p+00]
// [0x0p+00]
// [0x0.0p+00]
// [0x0.000p+00]
// [0x0.0000000000p+00]
// [0x0.000p+00]
// [                  0x0.0000p+00]
