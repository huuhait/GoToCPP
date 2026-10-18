// phase: 05

// Go's %v of float64 values (special) with flags, width and precision, as strconv.AppendFloat and
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
	fmt.Printf("[%v]\n", inf)
	fmt.Printf("[%+v]\n", inf)
	fmt.Printf("[%-v]\n", inf)
	fmt.Printf("[% v]\n", inf)
	fmt.Printf("[%0v]\n", inf)
	fmt.Printf("[%#v]\n", inf)
	fmt.Printf("[%10v]\n", inf)
	fmt.Printf("[%-10v]\n", inf)
	fmt.Printf("[%010v]\n", inf)
	fmt.Printf("[%+010v]\n", inf)
	fmt.Printf("[%10.3v]\n", inf)
	fmt.Printf("[%.0v]\n", inf)
	fmt.Printf("[%.1v]\n", inf)
	fmt.Printf("[%.3v]\n", inf)
	fmt.Printf("[%.10v]\n", inf)
	fmt.Printf("[%#.0v]\n", inf)
	fmt.Printf("[%#10.3v]\n", inf)
	fmt.Printf("[%+.2v]\n", inf)
	fmt.Printf("[% 10.2v]\n", inf)
	fmt.Printf("[%-#10.2v]\n", inf)
	fmt.Printf("[%20.15v]\n", inf)
	fmt.Printf("[%.20v]\n", inf)
	fmt.Printf("[%0-10v]\n", inf)
	fmt.Printf("[%v]\n", ninf)
	fmt.Printf("[%+v]\n", ninf)
	fmt.Printf("[%-v]\n", ninf)
	fmt.Printf("[% v]\n", ninf)
	fmt.Printf("[%0v]\n", ninf)
	fmt.Printf("[%#v]\n", ninf)
	fmt.Printf("[%10v]\n", ninf)
	fmt.Printf("[%-10v]\n", ninf)
	fmt.Printf("[%010v]\n", ninf)
	fmt.Printf("[%+010v]\n", ninf)
	fmt.Printf("[%10.3v]\n", ninf)
	fmt.Printf("[%.0v]\n", ninf)
	fmt.Printf("[%.1v]\n", ninf)
	fmt.Printf("[%.3v]\n", ninf)
	fmt.Printf("[%.10v]\n", ninf)
	fmt.Printf("[%#.0v]\n", ninf)
	fmt.Printf("[%#10.3v]\n", ninf)
	fmt.Printf("[%+.2v]\n", ninf)
	fmt.Printf("[% 10.2v]\n", ninf)
	fmt.Printf("[%-#10.2v]\n", ninf)
	fmt.Printf("[%20.15v]\n", ninf)
	fmt.Printf("[%.20v]\n", ninf)
	fmt.Printf("[%0-10v]\n", ninf)
	fmt.Printf("[%v]\n", nan)
	fmt.Printf("[%+v]\n", nan)
	fmt.Printf("[%-v]\n", nan)
	fmt.Printf("[% v]\n", nan)
	fmt.Printf("[%0v]\n", nan)
	fmt.Printf("[%#v]\n", nan)
	fmt.Printf("[%10v]\n", nan)
	fmt.Printf("[%-10v]\n", nan)
	fmt.Printf("[%010v]\n", nan)
	fmt.Printf("[%+010v]\n", nan)
	fmt.Printf("[%10.3v]\n", nan)
	fmt.Printf("[%.0v]\n", nan)
	fmt.Printf("[%.1v]\n", nan)
	fmt.Printf("[%.3v]\n", nan)
	fmt.Printf("[%.10v]\n", nan)
	fmt.Printf("[%#.0v]\n", nan)
	fmt.Printf("[%#10.3v]\n", nan)
	fmt.Printf("[%+.2v]\n", nan)
	fmt.Printf("[% 10.2v]\n", nan)
	fmt.Printf("[%-#10.2v]\n", nan)
	fmt.Printf("[%20.15v]\n", nan)
	fmt.Printf("[%.20v]\n", nan)
	fmt.Printf("[%0-10v]\n", nan)
	fmt.Printf("[%v]\n", nzero)
	fmt.Printf("[%+v]\n", nzero)
	fmt.Printf("[%-v]\n", nzero)
	fmt.Printf("[% v]\n", nzero)
	fmt.Printf("[%0v]\n", nzero)
	fmt.Printf("[%#v]\n", nzero)
	fmt.Printf("[%10v]\n", nzero)
	fmt.Printf("[%-10v]\n", nzero)
	fmt.Printf("[%010v]\n", nzero)
	fmt.Printf("[%+010v]\n", nzero)
	fmt.Printf("[%10.3v]\n", nzero)
	fmt.Printf("[%.0v]\n", nzero)
	fmt.Printf("[%.1v]\n", nzero)
	fmt.Printf("[%.3v]\n", nzero)
	fmt.Printf("[%.10v]\n", nzero)
	fmt.Printf("[%#.0v]\n", nzero)
	fmt.Printf("[%#10.3v]\n", nzero)
	fmt.Printf("[%+.2v]\n", nzero)
	fmt.Printf("[% 10.2v]\n", nzero)
	fmt.Printf("[%-#10.2v]\n", nzero)
	fmt.Printf("[%20.15v]\n", nzero)
	fmt.Printf("[%.20v]\n", nzero)
	fmt.Printf("[%0-10v]\n", nzero)
	fmt.Printf("[%v]\n", 0.0)
	fmt.Printf("[%+v]\n", 0.0)
	fmt.Printf("[%-v]\n", 0.0)
	fmt.Printf("[% v]\n", 0.0)
	fmt.Printf("[%0v]\n", 0.0)
	fmt.Printf("[%#v]\n", 0.0)
	fmt.Printf("[%10v]\n", 0.0)
	fmt.Printf("[%-10v]\n", 0.0)
	fmt.Printf("[%010v]\n", 0.0)
	fmt.Printf("[%+010v]\n", 0.0)
	fmt.Printf("[%10.3v]\n", 0.0)
	fmt.Printf("[%.0v]\n", 0.0)
	fmt.Printf("[%.1v]\n", 0.0)
	fmt.Printf("[%.3v]\n", 0.0)
	fmt.Printf("[%.10v]\n", 0.0)
	fmt.Printf("[%#.0v]\n", 0.0)
	fmt.Printf("[%#10.3v]\n", 0.0)
	fmt.Printf("[%+.2v]\n", 0.0)
	fmt.Printf("[% 10.2v]\n", 0.0)
	fmt.Printf("[%-#10.2v]\n", 0.0)
	fmt.Printf("[%20.15v]\n", 0.0)
	fmt.Printf("[%.20v]\n", 0.0)
	fmt.Printf("[%0-10v]\n", 0.0)
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
// [NaN]
// [NaN]
// [ NaN]
// [NaN]
// [NaN]
// [       NaN]
// [NaN       ]
// [       NaN]
// [       NaN]
// [       NaN]
// [NaN]
// [NaN]
// [NaN]
// [NaN]
// [NaN]
// [       NaN]
// [NaN]
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
// [-0]
// [        -0]
// [-0        ]
// [-000000000]
// [-000000000]
// [        -0]
// [-0]
// [-0]
// [-0]
// [-0]
// [-0]
// [        -0]
// [-0]
// [        -0]
// [-0        ]
// [                  -0]
// [-0]
// [-0        ]
// [0]
// [0]
// [0]
// [ 0]
// [0]
// [0]
// [         0]
// [0         ]
// [0000000000]
// [0000000000]
// [         0]
// [0]
// [0]
// [0]
// [0]
// [0]
// [         0]
// [0]
// [         0]
// [0         ]
// [                   0]
// [0]
// [0         ]
