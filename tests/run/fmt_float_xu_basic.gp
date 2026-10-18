// phase: 05

// Go's %X of float64 values (basic) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
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
	fmt.Printf("[%X]\n", 1.0)
	fmt.Printf("[%+X]\n", 1.0)
	fmt.Printf("[%-X]\n", 1.0)
	fmt.Printf("[% X]\n", 1.0)
	fmt.Printf("[%0X]\n", 1.0)
	fmt.Printf("[%#X]\n", 1.0)
	fmt.Printf("[%30X]\n", 1.0)
	fmt.Printf("[%-30X]\n", 1.0)
	fmt.Printf("[%030X]\n", 1.0)
	fmt.Printf("[%+030X]\n", 1.0)
	fmt.Printf("[%.0X]\n", 1.0)
	fmt.Printf("[%.1X]\n", 1.0)
	fmt.Printf("[%.3X]\n", 1.0)
	fmt.Printf("[%.10X]\n", 1.0)
	fmt.Printf("[%#.3X]\n", 1.0)
	fmt.Printf("[%#30X]\n", 1.0)
	fmt.Printf("[%X]\n", -1.0)
	fmt.Printf("[%+X]\n", -1.0)
	fmt.Printf("[%-X]\n", -1.0)
	fmt.Printf("[% X]\n", -1.0)
	fmt.Printf("[%0X]\n", -1.0)
	fmt.Printf("[%#X]\n", -1.0)
	fmt.Printf("[%30X]\n", -1.0)
	fmt.Printf("[%-30X]\n", -1.0)
	fmt.Printf("[%030X]\n", -1.0)
	fmt.Printf("[%+030X]\n", -1.0)
	fmt.Printf("[%.0X]\n", -1.0)
	fmt.Printf("[%.1X]\n", -1.0)
	fmt.Printf("[%.3X]\n", -1.0)
	fmt.Printf("[%.10X]\n", -1.0)
	fmt.Printf("[%#.3X]\n", -1.0)
	fmt.Printf("[%#30X]\n", -1.0)
	fmt.Printf("[%X]\n", 3.14159265358979)
	fmt.Printf("[%+X]\n", 3.14159265358979)
	fmt.Printf("[%-X]\n", 3.14159265358979)
	fmt.Printf("[% X]\n", 3.14159265358979)
	fmt.Printf("[%0X]\n", 3.14159265358979)
	fmt.Printf("[%#X]\n", 3.14159265358979)
	fmt.Printf("[%30X]\n", 3.14159265358979)
	fmt.Printf("[%-30X]\n", 3.14159265358979)
	fmt.Printf("[%030X]\n", 3.14159265358979)
	fmt.Printf("[%+030X]\n", 3.14159265358979)
	fmt.Printf("[%.0X]\n", 3.14159265358979)
	fmt.Printf("[%.1X]\n", 3.14159265358979)
	fmt.Printf("[%.3X]\n", 3.14159265358979)
	fmt.Printf("[%.10X]\n", 3.14159265358979)
	fmt.Printf("[%#.3X]\n", 3.14159265358979)
	fmt.Printf("[%#30X]\n", 3.14159265358979)
	fmt.Printf("[%X]\n", 2.5)
	fmt.Printf("[%+X]\n", 2.5)
	fmt.Printf("[%-X]\n", 2.5)
	fmt.Printf("[% X]\n", 2.5)
	fmt.Printf("[%0X]\n", 2.5)
	fmt.Printf("[%#X]\n", 2.5)
	fmt.Printf("[%30X]\n", 2.5)
	fmt.Printf("[%-30X]\n", 2.5)
	fmt.Printf("[%030X]\n", 2.5)
	fmt.Printf("[%+030X]\n", 2.5)
	fmt.Printf("[%.0X]\n", 2.5)
	fmt.Printf("[%.1X]\n", 2.5)
	fmt.Printf("[%.3X]\n", 2.5)
	fmt.Printf("[%.10X]\n", 2.5)
	fmt.Printf("[%#.3X]\n", 2.5)
	fmt.Printf("[%#30X]\n", 2.5)
	fmt.Printf("[%X]\n", 100.0)
	fmt.Printf("[%+X]\n", 100.0)
	fmt.Printf("[%-X]\n", 100.0)
	fmt.Printf("[% X]\n", 100.0)
	fmt.Printf("[%0X]\n", 100.0)
	fmt.Printf("[%#X]\n", 100.0)
	fmt.Printf("[%30X]\n", 100.0)
	fmt.Printf("[%-30X]\n", 100.0)
	fmt.Printf("[%030X]\n", 100.0)
	fmt.Printf("[%+030X]\n", 100.0)
	fmt.Printf("[%.0X]\n", 100.0)
	fmt.Printf("[%.1X]\n", 100.0)
	fmt.Printf("[%.3X]\n", 100.0)
	fmt.Printf("[%.10X]\n", 100.0)
	fmt.Printf("[%#.3X]\n", 100.0)
	fmt.Printf("[%#30X]\n", 100.0)
	fmt.Printf("[%X]\n", 0.5)
	fmt.Printf("[%+X]\n", 0.5)
	fmt.Printf("[%-X]\n", 0.5)
	fmt.Printf("[% X]\n", 0.5)
	fmt.Printf("[%0X]\n", 0.5)
	fmt.Printf("[%#X]\n", 0.5)
	fmt.Printf("[%30X]\n", 0.5)
	fmt.Printf("[%-30X]\n", 0.5)
	fmt.Printf("[%030X]\n", 0.5)
	fmt.Printf("[%+030X]\n", 0.5)
	fmt.Printf("[%.0X]\n", 0.5)
	fmt.Printf("[%.1X]\n", 0.5)
	fmt.Printf("[%.3X]\n", 0.5)
	fmt.Printf("[%.10X]\n", 0.5)
	fmt.Printf("[%#.3X]\n", 0.5)
	fmt.Printf("[%#30X]\n", 0.5)
	fmt.Printf("[%X]\n", 123456.789)
	fmt.Printf("[%+X]\n", 123456.789)
	fmt.Printf("[%-X]\n", 123456.789)
	fmt.Printf("[% X]\n", 123456.789)
	fmt.Printf("[%0X]\n", 123456.789)
	fmt.Printf("[%#X]\n", 123456.789)
	fmt.Printf("[%30X]\n", 123456.789)
	fmt.Printf("[%-30X]\n", 123456.789)
	fmt.Printf("[%030X]\n", 123456.789)
	fmt.Printf("[%+030X]\n", 123456.789)
	fmt.Printf("[%.0X]\n", 123456.789)
	fmt.Printf("[%.1X]\n", 123456.789)
	fmt.Printf("[%.3X]\n", 123456.789)
	fmt.Printf("[%.10X]\n", 123456.789)
	fmt.Printf("[%#.3X]\n", 123456.789)
	fmt.Printf("[%#30X]\n", 123456.789)
}

// Output:
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
// [0X1P+00]
// [+0X1P+00]
// [0X1P+00]
// [ 0X1P+00]
// [0X1P+00]
// [0X1.P+00]
// [                       0X1P+00]
// [0X1P+00                       ]
// [000000000000000000000000X1P+00]
// [+00000000000000000000000X1P+00]
// [0X1P+00]
// [0X1.0P+00]
// [0X1.000P+00]
// [0X1.0000000000P+00]
// [0X1.000P+00]
// [                      0X1.P+00]
// [-0X1P+00]
// [-0X1P+00]
// [-0X1P+00]
// [-0X1P+00]
// [-0X1P+00]
// [-0X1.P+00]
// [                      -0X1P+00]
// [-0X1P+00                      ]
// [-00000000000000000000000X1P+00]
// [-00000000000000000000000X1P+00]
// [-0X1P+00]
// [-0X1.0P+00]
// [-0X1.000P+00]
// [-0X1.0000000000P+00]
// [-0X1.000P+00]
// [                     -0X1.P+00]
// [0X1.921FB54442D11P+01]
// [+0X1.921FB54442D11P+01]
// [0X1.921FB54442D11P+01]
// [ 0X1.921FB54442D11P+01]
// [0X1.921FB54442D11P+01]
// [0X1.921FB54442D11P+01]
// [         0X1.921FB54442D11P+01]
// [0X1.921FB54442D11P+01         ]
// [0000000000X1.921FB54442D11P+01]
// [+000000000X1.921FB54442D11P+01]
// [0X1P+02]
// [0X1.9P+01]
// [0X1.922P+01]
// [0X1.921FB54443P+01]
// [0X1.922P+01]
// [         0X1.921FB54442D11P+01]
// [0X1.4P+01]
// [+0X1.4P+01]
// [0X1.4P+01]
// [ 0X1.4P+01]
// [0X1.4P+01]
// [0X1.4P+01]
// [                     0X1.4P+01]
// [0X1.4P+01                     ]
// [0000000000000000000000X1.4P+01]
// [+000000000000000000000X1.4P+01]
// [0X1P+01]
// [0X1.4P+01]
// [0X1.400P+01]
// [0X1.4000000000P+01]
// [0X1.400P+01]
// [                     0X1.4P+01]
// [0X1.9P+06]
// [+0X1.9P+06]
// [0X1.9P+06]
// [ 0X1.9P+06]
// [0X1.9P+06]
// [0X1.9P+06]
// [                     0X1.9P+06]
// [0X1.9P+06                     ]
// [0000000000000000000000X1.9P+06]
// [+000000000000000000000X1.9P+06]
// [0X1P+07]
// [0X1.9P+06]
// [0X1.900P+06]
// [0X1.9000000000P+06]
// [0X1.900P+06]
// [                     0X1.9P+06]
// [0X1P-01]
// [+0X1P-01]
// [0X1P-01]
// [ 0X1P-01]
// [0X1P-01]
// [0X1.P-01]
// [                       0X1P-01]
// [0X1P-01                       ]
// [000000000000000000000000X1P-01]
// [+00000000000000000000000X1P-01]
// [0X1P-01]
// [0X1.0P-01]
// [0X1.000P-01]
// [0X1.0000000000P-01]
// [0X1.000P-01]
// [                      0X1.P-01]
// [0X1.E240C9FBE76C9P+16]
// [+0X1.E240C9FBE76C9P+16]
// [0X1.E240C9FBE76C9P+16]
// [ 0X1.E240C9FBE76C9P+16]
// [0X1.E240C9FBE76C9P+16]
// [0X1.E240C9FBE76C9P+16]
// [         0X1.E240C9FBE76C9P+16]
// [0X1.E240C9FBE76C9P+16         ]
// [0000000000X1.E240C9FBE76C9P+16]
// [+000000000X1.E240C9FBE76C9P+16]
// [0X1P+17]
// [0X1.EP+16]
// [0X1.E24P+16]
// [0X1.E240C9FBE7P+16]
// [0X1.E24P+16]
// [         0X1.E240C9FBE76C9P+16]
