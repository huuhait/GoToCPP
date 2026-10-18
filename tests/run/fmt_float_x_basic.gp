// phase: 05

// Go's %x of float64 values (basic) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
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
	fmt.Printf("[%x]\n", 1.0)
	fmt.Printf("[%+x]\n", 1.0)
	fmt.Printf("[%-x]\n", 1.0)
	fmt.Printf("[% x]\n", 1.0)
	fmt.Printf("[%0x]\n", 1.0)
	fmt.Printf("[%#x]\n", 1.0)
	fmt.Printf("[%30x]\n", 1.0)
	fmt.Printf("[%-30x]\n", 1.0)
	fmt.Printf("[%030x]\n", 1.0)
	fmt.Printf("[%+030x]\n", 1.0)
	fmt.Printf("[%.0x]\n", 1.0)
	fmt.Printf("[%.1x]\n", 1.0)
	fmt.Printf("[%.3x]\n", 1.0)
	fmt.Printf("[%.10x]\n", 1.0)
	fmt.Printf("[%#.3x]\n", 1.0)
	fmt.Printf("[%#30x]\n", 1.0)
	fmt.Printf("[%x]\n", -1.0)
	fmt.Printf("[%+x]\n", -1.0)
	fmt.Printf("[%-x]\n", -1.0)
	fmt.Printf("[% x]\n", -1.0)
	fmt.Printf("[%0x]\n", -1.0)
	fmt.Printf("[%#x]\n", -1.0)
	fmt.Printf("[%30x]\n", -1.0)
	fmt.Printf("[%-30x]\n", -1.0)
	fmt.Printf("[%030x]\n", -1.0)
	fmt.Printf("[%+030x]\n", -1.0)
	fmt.Printf("[%.0x]\n", -1.0)
	fmt.Printf("[%.1x]\n", -1.0)
	fmt.Printf("[%.3x]\n", -1.0)
	fmt.Printf("[%.10x]\n", -1.0)
	fmt.Printf("[%#.3x]\n", -1.0)
	fmt.Printf("[%#30x]\n", -1.0)
	fmt.Printf("[%x]\n", 3.14159265358979)
	fmt.Printf("[%+x]\n", 3.14159265358979)
	fmt.Printf("[%-x]\n", 3.14159265358979)
	fmt.Printf("[% x]\n", 3.14159265358979)
	fmt.Printf("[%0x]\n", 3.14159265358979)
	fmt.Printf("[%#x]\n", 3.14159265358979)
	fmt.Printf("[%30x]\n", 3.14159265358979)
	fmt.Printf("[%-30x]\n", 3.14159265358979)
	fmt.Printf("[%030x]\n", 3.14159265358979)
	fmt.Printf("[%+030x]\n", 3.14159265358979)
	fmt.Printf("[%.0x]\n", 3.14159265358979)
	fmt.Printf("[%.1x]\n", 3.14159265358979)
	fmt.Printf("[%.3x]\n", 3.14159265358979)
	fmt.Printf("[%.10x]\n", 3.14159265358979)
	fmt.Printf("[%#.3x]\n", 3.14159265358979)
	fmt.Printf("[%#30x]\n", 3.14159265358979)
	fmt.Printf("[%x]\n", 2.5)
	fmt.Printf("[%+x]\n", 2.5)
	fmt.Printf("[%-x]\n", 2.5)
	fmt.Printf("[% x]\n", 2.5)
	fmt.Printf("[%0x]\n", 2.5)
	fmt.Printf("[%#x]\n", 2.5)
	fmt.Printf("[%30x]\n", 2.5)
	fmt.Printf("[%-30x]\n", 2.5)
	fmt.Printf("[%030x]\n", 2.5)
	fmt.Printf("[%+030x]\n", 2.5)
	fmt.Printf("[%.0x]\n", 2.5)
	fmt.Printf("[%.1x]\n", 2.5)
	fmt.Printf("[%.3x]\n", 2.5)
	fmt.Printf("[%.10x]\n", 2.5)
	fmt.Printf("[%#.3x]\n", 2.5)
	fmt.Printf("[%#30x]\n", 2.5)
	fmt.Printf("[%x]\n", 100.0)
	fmt.Printf("[%+x]\n", 100.0)
	fmt.Printf("[%-x]\n", 100.0)
	fmt.Printf("[% x]\n", 100.0)
	fmt.Printf("[%0x]\n", 100.0)
	fmt.Printf("[%#x]\n", 100.0)
	fmt.Printf("[%30x]\n", 100.0)
	fmt.Printf("[%-30x]\n", 100.0)
	fmt.Printf("[%030x]\n", 100.0)
	fmt.Printf("[%+030x]\n", 100.0)
	fmt.Printf("[%.0x]\n", 100.0)
	fmt.Printf("[%.1x]\n", 100.0)
	fmt.Printf("[%.3x]\n", 100.0)
	fmt.Printf("[%.10x]\n", 100.0)
	fmt.Printf("[%#.3x]\n", 100.0)
	fmt.Printf("[%#30x]\n", 100.0)
	fmt.Printf("[%x]\n", 0.5)
	fmt.Printf("[%+x]\n", 0.5)
	fmt.Printf("[%-x]\n", 0.5)
	fmt.Printf("[% x]\n", 0.5)
	fmt.Printf("[%0x]\n", 0.5)
	fmt.Printf("[%#x]\n", 0.5)
	fmt.Printf("[%30x]\n", 0.5)
	fmt.Printf("[%-30x]\n", 0.5)
	fmt.Printf("[%030x]\n", 0.5)
	fmt.Printf("[%+030x]\n", 0.5)
	fmt.Printf("[%.0x]\n", 0.5)
	fmt.Printf("[%.1x]\n", 0.5)
	fmt.Printf("[%.3x]\n", 0.5)
	fmt.Printf("[%.10x]\n", 0.5)
	fmt.Printf("[%#.3x]\n", 0.5)
	fmt.Printf("[%#30x]\n", 0.5)
	fmt.Printf("[%x]\n", 123456.789)
	fmt.Printf("[%+x]\n", 123456.789)
	fmt.Printf("[%-x]\n", 123456.789)
	fmt.Printf("[% x]\n", 123456.789)
	fmt.Printf("[%0x]\n", 123456.789)
	fmt.Printf("[%#x]\n", 123456.789)
	fmt.Printf("[%30x]\n", 123456.789)
	fmt.Printf("[%-30x]\n", 123456.789)
	fmt.Printf("[%030x]\n", 123456.789)
	fmt.Printf("[%+030x]\n", 123456.789)
	fmt.Printf("[%.0x]\n", 123456.789)
	fmt.Printf("[%.1x]\n", 123456.789)
	fmt.Printf("[%.3x]\n", 123456.789)
	fmt.Printf("[%.10x]\n", 123456.789)
	fmt.Printf("[%#.3x]\n", 123456.789)
	fmt.Printf("[%#30x]\n", 123456.789)
}

// Output:
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
// [0x1p+00]
// [+0x1p+00]
// [0x1p+00]
// [ 0x1p+00]
// [0x1p+00]
// [0x1.0000p+00]
// [                       0x1p+00]
// [0x1p+00                       ]
// [000000000000000000000000x1p+00]
// [+00000000000000000000000x1p+00]
// [0x1p+00]
// [0x1.0p+00]
// [0x1.000p+00]
// [0x1.0000000000p+00]
// [0x1.000p+00]
// [                  0x1.0000p+00]
// [-0x1p+00]
// [-0x1p+00]
// [-0x1p+00]
// [-0x1p+00]
// [-0x1p+00]
// [-0x1.0000p+00]
// [                      -0x1p+00]
// [-0x1p+00                      ]
// [-00000000000000000000000x1p+00]
// [-00000000000000000000000x1p+00]
// [-0x1p+00]
// [-0x1.0p+00]
// [-0x1.000p+00]
// [-0x1.0000000000p+00]
// [-0x1.000p+00]
// [                 -0x1.0000p+00]
// [0x1.921fb54442d11p+01]
// [+0x1.921fb54442d11p+01]
// [0x1.921fb54442d11p+01]
// [ 0x1.921fb54442d11p+01]
// [0x1.921fb54442d11p+01]
// [0x1.921fb54442d11p+01]
// [         0x1.921fb54442d11p+01]
// [0x1.921fb54442d11p+01         ]
// [0000000000x1.921fb54442d11p+01]
// [+000000000x1.921fb54442d11p+01]
// [0x1p+02]
// [0x1.9p+01]
// [0x1.922p+01]
// [0x1.921fb54443p+01]
// [0x1.922p+01]
// [         0x1.921fb54442d11p+01]
// [0x1.4p+01]
// [+0x1.4p+01]
// [0x1.4p+01]
// [ 0x1.4p+01]
// [0x1.4p+01]
// [0x1.4000p+01]
// [                     0x1.4p+01]
// [0x1.4p+01                     ]
// [0000000000000000000000x1.4p+01]
// [+000000000000000000000x1.4p+01]
// [0x1p+01]
// [0x1.4p+01]
// [0x1.400p+01]
// [0x1.4000000000p+01]
// [0x1.400p+01]
// [                  0x1.4000p+01]
// [0x1.9p+06]
// [+0x1.9p+06]
// [0x1.9p+06]
// [ 0x1.9p+06]
// [0x1.9p+06]
// [0x1.9000p+06]
// [                     0x1.9p+06]
// [0x1.9p+06                     ]
// [0000000000000000000000x1.9p+06]
// [+000000000000000000000x1.9p+06]
// [0x1p+07]
// [0x1.9p+06]
// [0x1.900p+06]
// [0x1.9000000000p+06]
// [0x1.900p+06]
// [                  0x1.9000p+06]
// [0x1p-01]
// [+0x1p-01]
// [0x1p-01]
// [ 0x1p-01]
// [0x1p-01]
// [0x1.0000p-01]
// [                       0x1p-01]
// [0x1p-01                       ]
// [000000000000000000000000x1p-01]
// [+00000000000000000000000x1p-01]
// [0x1p-01]
// [0x1.0p-01]
// [0x1.000p-01]
// [0x1.0000000000p-01]
// [0x1.000p-01]
// [                  0x1.0000p-01]
// [0x1.e240c9fbe76c9p+16]
// [+0x1.e240c9fbe76c9p+16]
// [0x1.e240c9fbe76c9p+16]
// [ 0x1.e240c9fbe76c9p+16]
// [0x1.e240c9fbe76c9p+16]
// [0x1.e240c9fbe76c9p+16]
// [         0x1.e240c9fbe76c9p+16]
// [0x1.e240c9fbe76c9p+16         ]
// [0000000000x1.e240c9fbe76c9p+16]
// [+000000000x1.e240c9fbe76c9p+16]
// [0x1p+17]
// [0x1.ep+16]
// [0x1.e24p+16]
// [0x1.e240c9fbe7p+16]
// [0x1.e24p+16]
// [         0x1.e240c9fbe76c9p+16]
