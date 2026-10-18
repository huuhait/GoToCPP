// phase: 05

// Go's %b of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%b]\n", inf)
	fmt.Printf("[%+b]\n", inf)
	fmt.Printf("[%-b]\n", inf)
	fmt.Printf("[% b]\n", inf)
	fmt.Printf("[%0b]\n", inf)
	fmt.Printf("[%#b]\n", inf)
	fmt.Printf("[%30b]\n", inf)
	fmt.Printf("[%-30b]\n", inf)
	fmt.Printf("[%030b]\n", inf)
	fmt.Printf("[%+030b]\n", inf)
	fmt.Printf("[%.0b]\n", inf)
	fmt.Printf("[%.1b]\n", inf)
	fmt.Printf("[%.3b]\n", inf)
	fmt.Printf("[%.10b]\n", inf)
	fmt.Printf("[%#.3b]\n", inf)
	fmt.Printf("[%#30b]\n", inf)
	fmt.Printf("[%b]\n", ninf)
	fmt.Printf("[%+b]\n", ninf)
	fmt.Printf("[%-b]\n", ninf)
	fmt.Printf("[% b]\n", ninf)
	fmt.Printf("[%0b]\n", ninf)
	fmt.Printf("[%#b]\n", ninf)
	fmt.Printf("[%30b]\n", ninf)
	fmt.Printf("[%-30b]\n", ninf)
	fmt.Printf("[%030b]\n", ninf)
	fmt.Printf("[%+030b]\n", ninf)
	fmt.Printf("[%.0b]\n", ninf)
	fmt.Printf("[%.1b]\n", ninf)
	fmt.Printf("[%.3b]\n", ninf)
	fmt.Printf("[%.10b]\n", ninf)
	fmt.Printf("[%#.3b]\n", ninf)
	fmt.Printf("[%#30b]\n", ninf)
	fmt.Printf("[%b]\n", nan)
	fmt.Printf("[%+b]\n", nan)
	fmt.Printf("[%-b]\n", nan)
	fmt.Printf("[% b]\n", nan)
	fmt.Printf("[%0b]\n", nan)
	fmt.Printf("[%#b]\n", nan)
	fmt.Printf("[%30b]\n", nan)
	fmt.Printf("[%-30b]\n", nan)
	fmt.Printf("[%030b]\n", nan)
	fmt.Printf("[%+030b]\n", nan)
	fmt.Printf("[%.0b]\n", nan)
	fmt.Printf("[%.1b]\n", nan)
	fmt.Printf("[%.3b]\n", nan)
	fmt.Printf("[%.10b]\n", nan)
	fmt.Printf("[%#.3b]\n", nan)
	fmt.Printf("[%#30b]\n", nan)
	fmt.Printf("[%b]\n", nzero)
	fmt.Printf("[%+b]\n", nzero)
	fmt.Printf("[%-b]\n", nzero)
	fmt.Printf("[% b]\n", nzero)
	fmt.Printf("[%0b]\n", nzero)
	fmt.Printf("[%#b]\n", nzero)
	fmt.Printf("[%30b]\n", nzero)
	fmt.Printf("[%-30b]\n", nzero)
	fmt.Printf("[%030b]\n", nzero)
	fmt.Printf("[%+030b]\n", nzero)
	fmt.Printf("[%.0b]\n", nzero)
	fmt.Printf("[%.1b]\n", nzero)
	fmt.Printf("[%.3b]\n", nzero)
	fmt.Printf("[%.10b]\n", nzero)
	fmt.Printf("[%#.3b]\n", nzero)
	fmt.Printf("[%#30b]\n", nzero)
	fmt.Printf("[%b]\n", 0.0)
	fmt.Printf("[%+b]\n", 0.0)
	fmt.Printf("[%-b]\n", 0.0)
	fmt.Printf("[% b]\n", 0.0)
	fmt.Printf("[%0b]\n", 0.0)
	fmt.Printf("[%#b]\n", 0.0)
	fmt.Printf("[%30b]\n", 0.0)
	fmt.Printf("[%-30b]\n", 0.0)
	fmt.Printf("[%030b]\n", 0.0)
	fmt.Printf("[%+030b]\n", 0.0)
	fmt.Printf("[%.0b]\n", 0.0)
	fmt.Printf("[%.1b]\n", 0.0)
	fmt.Printf("[%.3b]\n", 0.0)
	fmt.Printf("[%.10b]\n", 0.0)
	fmt.Printf("[%#.3b]\n", 0.0)
	fmt.Printf("[%#30b]\n", 0.0)
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
// [-0p-1074]
// [-0p-1074]
// [-0p-1074]
// [-0p-1074]
// [-0p-1074]
// [-0p-1074]
// [                      -0p-1074]
// [-0p-1074                      ]
// [-00000000000000000000000p-1074]
// [-00000000000000000000000p-1074]
// [-0p-1074]
// [-0p-1074]
// [-0p-1074]
// [-0p-1074]
// [-0p-1074]
// [                      -0p-1074]
// [0p-1074]
// [+0p-1074]
// [0p-1074]
// [ 0p-1074]
// [0p-1074]
// [0p-1074]
// [                       0p-1074]
// [0p-1074                       ]
// [000000000000000000000000p-1074]
// [+00000000000000000000000p-1074]
// [0p-1074]
// [0p-1074]
// [0p-1074]
// [0p-1074]
// [0p-1074]
// [                       0p-1074]
