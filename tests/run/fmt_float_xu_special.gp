// phase: 05

// Go's %X of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%X]\n", inf)
	fmt.Printf("[%+X]\n", inf)
	fmt.Printf("[%-X]\n", inf)
	fmt.Printf("[% X]\n", inf)
	fmt.Printf("[%0X]\n", inf)
	fmt.Printf("[%#X]\n", inf)
	fmt.Printf("[%30X]\n", inf)
	fmt.Printf("[%-30X]\n", inf)
	fmt.Printf("[%030X]\n", inf)
	fmt.Printf("[%+030X]\n", inf)
	fmt.Printf("[%.0X]\n", inf)
	fmt.Printf("[%.1X]\n", inf)
	fmt.Printf("[%.3X]\n", inf)
	fmt.Printf("[%.10X]\n", inf)
	fmt.Printf("[%#.3X]\n", inf)
	fmt.Printf("[%#30X]\n", inf)
	fmt.Printf("[%X]\n", ninf)
	fmt.Printf("[%+X]\n", ninf)
	fmt.Printf("[%-X]\n", ninf)
	fmt.Printf("[% X]\n", ninf)
	fmt.Printf("[%0X]\n", ninf)
	fmt.Printf("[%#X]\n", ninf)
	fmt.Printf("[%30X]\n", ninf)
	fmt.Printf("[%-30X]\n", ninf)
	fmt.Printf("[%030X]\n", ninf)
	fmt.Printf("[%+030X]\n", ninf)
	fmt.Printf("[%.0X]\n", ninf)
	fmt.Printf("[%.1X]\n", ninf)
	fmt.Printf("[%.3X]\n", ninf)
	fmt.Printf("[%.10X]\n", ninf)
	fmt.Printf("[%#.3X]\n", ninf)
	fmt.Printf("[%#30X]\n", ninf)
	fmt.Printf("[%X]\n", nan)
	fmt.Printf("[%+X]\n", nan)
	fmt.Printf("[%-X]\n", nan)
	fmt.Printf("[% X]\n", nan)
	fmt.Printf("[%0X]\n", nan)
	fmt.Printf("[%#X]\n", nan)
	fmt.Printf("[%30X]\n", nan)
	fmt.Printf("[%-30X]\n", nan)
	fmt.Printf("[%030X]\n", nan)
	fmt.Printf("[%+030X]\n", nan)
	fmt.Printf("[%.0X]\n", nan)
	fmt.Printf("[%.1X]\n", nan)
	fmt.Printf("[%.3X]\n", nan)
	fmt.Printf("[%.10X]\n", nan)
	fmt.Printf("[%#.3X]\n", nan)
	fmt.Printf("[%#30X]\n", nan)
	fmt.Printf("[%X]\n", nzero)
	fmt.Printf("[%+X]\n", nzero)
	fmt.Printf("[%-X]\n", nzero)
	fmt.Printf("[% X]\n", nzero)
	fmt.Printf("[%0X]\n", nzero)
	fmt.Printf("[%#X]\n", nzero)
	fmt.Printf("[%30X]\n", nzero)
	fmt.Printf("[%-30X]\n", nzero)
	fmt.Printf("[%030X]\n", nzero)
	fmt.Printf("[%+030X]\n", nzero)
	fmt.Printf("[%.0X]\n", nzero)
	fmt.Printf("[%.1X]\n", nzero)
	fmt.Printf("[%.3X]\n", nzero)
	fmt.Printf("[%.10X]\n", nzero)
	fmt.Printf("[%#.3X]\n", nzero)
	fmt.Printf("[%#30X]\n", nzero)
	fmt.Printf("[%X]\n", 0.0)
	fmt.Printf("[%+X]\n", 0.0)
	fmt.Printf("[%-X]\n", 0.0)
	fmt.Printf("[% X]\n", 0.0)
	fmt.Printf("[%0X]\n", 0.0)
	fmt.Printf("[%#X]\n", 0.0)
	fmt.Printf("[%30X]\n", 0.0)
	fmt.Printf("[%-30X]\n", 0.0)
	fmt.Printf("[%030X]\n", 0.0)
	fmt.Printf("[%+030X]\n", 0.0)
	fmt.Printf("[%.0X]\n", 0.0)
	fmt.Printf("[%.1X]\n", 0.0)
	fmt.Printf("[%.3X]\n", 0.0)
	fmt.Printf("[%.10X]\n", 0.0)
	fmt.Printf("[%#.3X]\n", 0.0)
	fmt.Printf("[%#30X]\n", 0.0)
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
// [-0X0P+00]
// [-0X0P+00]
// [-0X0P+00]
// [-0X0P+00]
// [-0X0P+00]
// [-0X0.P+00]
// [                      -0X0P+00]
// [-0X0P+00                      ]
// [-00000000000000000000000X0P+00]
// [-00000000000000000000000X0P+00]
// [-0X0P+00]
// [-0X0.0P+00]
// [-0X0.000P+00]
// [-0X0.0000000000P+00]
// [-0X0.000P+00]
// [                     -0X0.P+00]
// [0X0P+00]
// [+0X0P+00]
// [0X0P+00]
// [ 0X0P+00]
// [0X0P+00]
// [0X0.P+00]
// [                       0X0P+00]
// [0X0P+00                       ]
// [000000000000000000000000X0P+00]
// [+00000000000000000000000X0P+00]
// [0X0P+00]
// [0X0.0P+00]
// [0X0.000P+00]
// [0X0.0000000000P+00]
// [0X0.000P+00]
// [                      0X0.P+00]
