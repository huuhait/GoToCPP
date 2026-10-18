// phase: 05

// Go's %X of float64 values (big) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", 1e6)
	fmt.Printf("[%+X]\n", 1e6)
	fmt.Printf("[%-X]\n", 1e6)
	fmt.Printf("[% X]\n", 1e6)
	fmt.Printf("[%0X]\n", 1e6)
	fmt.Printf("[%#X]\n", 1e6)
	fmt.Printf("[%30X]\n", 1e6)
	fmt.Printf("[%-30X]\n", 1e6)
	fmt.Printf("[%030X]\n", 1e6)
	fmt.Printf("[%+030X]\n", 1e6)
	fmt.Printf("[%.0X]\n", 1e6)
	fmt.Printf("[%.1X]\n", 1e6)
	fmt.Printf("[%.3X]\n", 1e6)
	fmt.Printf("[%.10X]\n", 1e6)
	fmt.Printf("[%#.3X]\n", 1e6)
	fmt.Printf("[%#30X]\n", 1e6)
	fmt.Printf("[%X]\n", 1e20)
	fmt.Printf("[%+X]\n", 1e20)
	fmt.Printf("[%-X]\n", 1e20)
	fmt.Printf("[% X]\n", 1e20)
	fmt.Printf("[%0X]\n", 1e20)
	fmt.Printf("[%#X]\n", 1e20)
	fmt.Printf("[%30X]\n", 1e20)
	fmt.Printf("[%-30X]\n", 1e20)
	fmt.Printf("[%030X]\n", 1e20)
	fmt.Printf("[%+030X]\n", 1e20)
	fmt.Printf("[%.0X]\n", 1e20)
	fmt.Printf("[%.1X]\n", 1e20)
	fmt.Printf("[%.3X]\n", 1e20)
	fmt.Printf("[%.10X]\n", 1e20)
	fmt.Printf("[%#.3X]\n", 1e20)
	fmt.Printf("[%#30X]\n", 1e20)
	fmt.Printf("[%X]\n", 1e21)
	fmt.Printf("[%+X]\n", 1e21)
	fmt.Printf("[%-X]\n", 1e21)
	fmt.Printf("[% X]\n", 1e21)
	fmt.Printf("[%0X]\n", 1e21)
	fmt.Printf("[%#X]\n", 1e21)
	fmt.Printf("[%30X]\n", 1e21)
	fmt.Printf("[%-30X]\n", 1e21)
	fmt.Printf("[%030X]\n", 1e21)
	fmt.Printf("[%+030X]\n", 1e21)
	fmt.Printf("[%.0X]\n", 1e21)
	fmt.Printf("[%.1X]\n", 1e21)
	fmt.Printf("[%.3X]\n", 1e21)
	fmt.Printf("[%.10X]\n", 1e21)
	fmt.Printf("[%#.3X]\n", 1e21)
	fmt.Printf("[%#30X]\n", 1e21)
	fmt.Printf("[%X]\n", 1e22)
	fmt.Printf("[%+X]\n", 1e22)
	fmt.Printf("[%-X]\n", 1e22)
	fmt.Printf("[% X]\n", 1e22)
	fmt.Printf("[%0X]\n", 1e22)
	fmt.Printf("[%#X]\n", 1e22)
	fmt.Printf("[%30X]\n", 1e22)
	fmt.Printf("[%-30X]\n", 1e22)
	fmt.Printf("[%030X]\n", 1e22)
	fmt.Printf("[%+030X]\n", 1e22)
	fmt.Printf("[%.0X]\n", 1e22)
	fmt.Printf("[%.1X]\n", 1e22)
	fmt.Printf("[%.3X]\n", 1e22)
	fmt.Printf("[%.10X]\n", 1e22)
	fmt.Printf("[%#.3X]\n", 1e22)
	fmt.Printf("[%#30X]\n", 1e22)
	fmt.Printf("[%X]\n", 123456789.0)
	fmt.Printf("[%+X]\n", 123456789.0)
	fmt.Printf("[%-X]\n", 123456789.0)
	fmt.Printf("[% X]\n", 123456789.0)
	fmt.Printf("[%0X]\n", 123456789.0)
	fmt.Printf("[%#X]\n", 123456789.0)
	fmt.Printf("[%30X]\n", 123456789.0)
	fmt.Printf("[%-30X]\n", 123456789.0)
	fmt.Printf("[%030X]\n", 123456789.0)
	fmt.Printf("[%+030X]\n", 123456789.0)
	fmt.Printf("[%.0X]\n", 123456789.0)
	fmt.Printf("[%.1X]\n", 123456789.0)
	fmt.Printf("[%.3X]\n", 123456789.0)
	fmt.Printf("[%.10X]\n", 123456789.0)
	fmt.Printf("[%#.3X]\n", 123456789.0)
	fmt.Printf("[%#30X]\n", 123456789.0)
	fmt.Printf("[%X]\n", 12345678901234567890.0)
	fmt.Printf("[%+X]\n", 12345678901234567890.0)
	fmt.Printf("[%-X]\n", 12345678901234567890.0)
	fmt.Printf("[% X]\n", 12345678901234567890.0)
	fmt.Printf("[%0X]\n", 12345678901234567890.0)
	fmt.Printf("[%#X]\n", 12345678901234567890.0)
	fmt.Printf("[%30X]\n", 12345678901234567890.0)
	fmt.Printf("[%-30X]\n", 12345678901234567890.0)
	fmt.Printf("[%030X]\n", 12345678901234567890.0)
	fmt.Printf("[%+030X]\n", 12345678901234567890.0)
	fmt.Printf("[%.0X]\n", 12345678901234567890.0)
	fmt.Printf("[%.1X]\n", 12345678901234567890.0)
	fmt.Printf("[%.3X]\n", 12345678901234567890.0)
	fmt.Printf("[%.10X]\n", 12345678901234567890.0)
	fmt.Printf("[%#.3X]\n", 12345678901234567890.0)
	fmt.Printf("[%#30X]\n", 12345678901234567890.0)
	fmt.Printf("[%X]\n", 1e100)
	fmt.Printf("[%+X]\n", 1e100)
	fmt.Printf("[%-X]\n", 1e100)
	fmt.Printf("[% X]\n", 1e100)
	fmt.Printf("[%0X]\n", 1e100)
	fmt.Printf("[%#X]\n", 1e100)
	fmt.Printf("[%30X]\n", 1e100)
	fmt.Printf("[%-30X]\n", 1e100)
	fmt.Printf("[%030X]\n", 1e100)
	fmt.Printf("[%+030X]\n", 1e100)
	fmt.Printf("[%.0X]\n", 1e100)
	fmt.Printf("[%.1X]\n", 1e100)
	fmt.Printf("[%.3X]\n", 1e100)
	fmt.Printf("[%.10X]\n", 1e100)
	fmt.Printf("[%#.3X]\n", 1e100)
	fmt.Printf("[%#30X]\n", 1e100)
	fmt.Printf("[%X]\n", 1.7976931348623157e308)
	fmt.Printf("[%+X]\n", 1.7976931348623157e308)
	fmt.Printf("[%-X]\n", 1.7976931348623157e308)
	fmt.Printf("[% X]\n", 1.7976931348623157e308)
	fmt.Printf("[%0X]\n", 1.7976931348623157e308)
	fmt.Printf("[%#X]\n", 1.7976931348623157e308)
	fmt.Printf("[%30X]\n", 1.7976931348623157e308)
	fmt.Printf("[%-30X]\n", 1.7976931348623157e308)
	fmt.Printf("[%030X]\n", 1.7976931348623157e308)
	fmt.Printf("[%+030X]\n", 1.7976931348623157e308)
	fmt.Printf("[%.0X]\n", 1.7976931348623157e308)
	fmt.Printf("[%.1X]\n", 1.7976931348623157e308)
	fmt.Printf("[%.3X]\n", 1.7976931348623157e308)
	fmt.Printf("[%.10X]\n", 1.7976931348623157e308)
	fmt.Printf("[%#.3X]\n", 1.7976931348623157e308)
	fmt.Printf("[%#30X]\n", 1.7976931348623157e308)
	fmt.Printf("[%X]\n", float64(1<<53))
	fmt.Printf("[%+X]\n", float64(1<<53))
	fmt.Printf("[%-X]\n", float64(1<<53))
	fmt.Printf("[% X]\n", float64(1<<53))
	fmt.Printf("[%0X]\n", float64(1<<53))
	fmt.Printf("[%#X]\n", float64(1<<53))
	fmt.Printf("[%30X]\n", float64(1<<53))
	fmt.Printf("[%-30X]\n", float64(1<<53))
	fmt.Printf("[%030X]\n", float64(1<<53))
	fmt.Printf("[%+030X]\n", float64(1<<53))
	fmt.Printf("[%.0X]\n", float64(1<<53))
	fmt.Printf("[%.1X]\n", float64(1<<53))
	fmt.Printf("[%.3X]\n", float64(1<<53))
	fmt.Printf("[%.10X]\n", float64(1<<53))
	fmt.Printf("[%#.3X]\n", float64(1<<53))
	fmt.Printf("[%#30X]\n", float64(1<<53))
}

// Output:
// [0X1.E848P+19]
// [+0X1.E848P+19]
// [0X1.E848P+19]
// [ 0X1.E848P+19]
// [0X1.E848P+19]
// [0X1.E848P+19]
// [                  0X1.E848P+19]
// [0X1.E848P+19                  ]
// [0000000000000000000X1.E848P+19]
// [+000000000000000000X1.E848P+19]
// [0X1P+20]
// [0X1.FP+19]
// [0X1.E84P+19]
// [0X1.E848000000P+19]
// [0X1.E84P+19]
// [                  0X1.E848P+19]
// [0X1.5AF1D78B58C4P+66]
// [+0X1.5AF1D78B58C4P+66]
// [0X1.5AF1D78B58C4P+66]
// [ 0X1.5AF1D78B58C4P+66]
// [0X1.5AF1D78B58C4P+66]
// [0X1.5AF1D78B58C4P+66]
// [          0X1.5AF1D78B58C4P+66]
// [0X1.5AF1D78B58C4P+66          ]
// [00000000000X1.5AF1D78B58C4P+66]
// [+0000000000X1.5AF1D78B58C4P+66]
// [0X1P+66]
// [0X1.6P+66]
// [0X1.5AFP+66]
// [0X1.5AF1D78B59P+66]
// [0X1.5AFP+66]
// [          0X1.5AF1D78B58C4P+66]
// [0X1.B1AE4D6E2EF5P+69]
// [+0X1.B1AE4D6E2EF5P+69]
// [0X1.B1AE4D6E2EF5P+69]
// [ 0X1.B1AE4D6E2EF5P+69]
// [0X1.B1AE4D6E2EF5P+69]
// [0X1.B1AE4D6E2EF5P+69]
// [          0X1.B1AE4D6E2EF5P+69]
// [0X1.B1AE4D6E2EF5P+69          ]
// [00000000000X1.B1AE4D6E2EF5P+69]
// [+0000000000X1.B1AE4D6E2EF5P+69]
// [0X1P+70]
// [0X1.BP+69]
// [0X1.B1BP+69]
// [0X1.B1AE4D6E2FP+69]
// [0X1.B1BP+69]
// [          0X1.B1AE4D6E2EF5P+69]
// [0X1.0F0CF064DD592P+73]
// [+0X1.0F0CF064DD592P+73]
// [0X1.0F0CF064DD592P+73]
// [ 0X1.0F0CF064DD592P+73]
// [0X1.0F0CF064DD592P+73]
// [0X1.0F0CF064DD592P+73]
// [         0X1.0F0CF064DD592P+73]
// [0X1.0F0CF064DD592P+73         ]
// [0000000000X1.0F0CF064DD592P+73]
// [+000000000X1.0F0CF064DD592P+73]
// [0X1P+73]
// [0X1.1P+73]
// [0X1.0F1P+73]
// [0X1.0F0CF064DDP+73]
// [0X1.0F1P+73]
// [         0X1.0F0CF064DD592P+73]
// [0X1.D6F3454P+26]
// [+0X1.D6F3454P+26]
// [0X1.D6F3454P+26]
// [ 0X1.D6F3454P+26]
// [0X1.D6F3454P+26]
// [0X1.D6F3454P+26]
// [               0X1.D6F3454P+26]
// [0X1.D6F3454P+26               ]
// [0000000000000000X1.D6F3454P+26]
// [+000000000000000X1.D6F3454P+26]
// [0X1P+27]
// [0X1.DP+26]
// [0X1.D6FP+26]
// [0X1.D6F3454000P+26]
// [0X1.D6FP+26]
// [               0X1.D6F3454P+26]
// [0X1.56A95319D63E1P+63]
// [+0X1.56A95319D63E1P+63]
// [0X1.56A95319D63E1P+63]
// [ 0X1.56A95319D63E1P+63]
// [0X1.56A95319D63E1P+63]
// [0X1.56A95319D63E1P+63]
// [         0X1.56A95319D63E1P+63]
// [0X1.56A95319D63E1P+63         ]
// [0000000000X1.56A95319D63E1P+63]
// [+000000000X1.56A95319D63E1P+63]
// [0X1P+63]
// [0X1.5P+63]
// [0X1.56BP+63]
// [0X1.56A95319D6P+63]
// [0X1.56BP+63]
// [         0X1.56A95319D63E1P+63]
// [0X1.249AD2594C37DP+332]
// [+0X1.249AD2594C37DP+332]
// [0X1.249AD2594C37DP+332]
// [ 0X1.249AD2594C37DP+332]
// [0X1.249AD2594C37DP+332]
// [0X1.249AD2594C37DP+332]
// [        0X1.249AD2594C37DP+332]
// [0X1.249AD2594C37DP+332        ]
// [000000000X1.249AD2594C37DP+332]
// [+00000000X1.249AD2594C37DP+332]
// [0X1P+332]
// [0X1.2P+332]
// [0X1.24AP+332]
// [0X1.249AD2594CP+332]
// [0X1.24AP+332]
// [        0X1.249AD2594C37DP+332]
// [0X1.FFFFFFFFFFFFFP+1023]
// [+0X1.FFFFFFFFFFFFFP+1023]
// [0X1.FFFFFFFFFFFFFP+1023]
// [ 0X1.FFFFFFFFFFFFFP+1023]
// [0X1.FFFFFFFFFFFFFP+1023]
// [0X1.FFFFFFFFFFFFFP+1023]
// [       0X1.FFFFFFFFFFFFFP+1023]
// [0X1.FFFFFFFFFFFFFP+1023       ]
// [00000000X1.FFFFFFFFFFFFFP+1023]
// [+0000000X1.FFFFFFFFFFFFFP+1023]
// [0X1P+1024]
// [0X1.0P+1024]
// [0X1.000P+1024]
// [0X1.0000000000P+1024]
// [0X1.000P+1024]
// [       0X1.FFFFFFFFFFFFFP+1023]
// [0X1P+53]
// [+0X1P+53]
// [0X1P+53]
// [ 0X1P+53]
// [0X1P+53]
// [0X1.P+53]
// [                       0X1P+53]
// [0X1P+53                       ]
// [000000000000000000000000X1P+53]
// [+00000000000000000000000X1P+53]
// [0X1P+53]
// [0X1.0P+53]
// [0X1.000P+53]
// [0X1.0000000000P+53]
// [0X1.000P+53]
// [                      0X1.P+53]
