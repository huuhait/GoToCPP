// phase: 05

// Go's %x of float64 values (big) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", 1e6)
	fmt.Printf("[%+x]\n", 1e6)
	fmt.Printf("[%-x]\n", 1e6)
	fmt.Printf("[% x]\n", 1e6)
	fmt.Printf("[%0x]\n", 1e6)
	fmt.Printf("[%#x]\n", 1e6)
	fmt.Printf("[%30x]\n", 1e6)
	fmt.Printf("[%-30x]\n", 1e6)
	fmt.Printf("[%030x]\n", 1e6)
	fmt.Printf("[%+030x]\n", 1e6)
	fmt.Printf("[%.0x]\n", 1e6)
	fmt.Printf("[%.1x]\n", 1e6)
	fmt.Printf("[%.3x]\n", 1e6)
	fmt.Printf("[%.10x]\n", 1e6)
	fmt.Printf("[%#.3x]\n", 1e6)
	fmt.Printf("[%#30x]\n", 1e6)
	fmt.Printf("[%x]\n", 1e20)
	fmt.Printf("[%+x]\n", 1e20)
	fmt.Printf("[%-x]\n", 1e20)
	fmt.Printf("[% x]\n", 1e20)
	fmt.Printf("[%0x]\n", 1e20)
	fmt.Printf("[%#x]\n", 1e20)
	fmt.Printf("[%30x]\n", 1e20)
	fmt.Printf("[%-30x]\n", 1e20)
	fmt.Printf("[%030x]\n", 1e20)
	fmt.Printf("[%+030x]\n", 1e20)
	fmt.Printf("[%.0x]\n", 1e20)
	fmt.Printf("[%.1x]\n", 1e20)
	fmt.Printf("[%.3x]\n", 1e20)
	fmt.Printf("[%.10x]\n", 1e20)
	fmt.Printf("[%#.3x]\n", 1e20)
	fmt.Printf("[%#30x]\n", 1e20)
	fmt.Printf("[%x]\n", 1e21)
	fmt.Printf("[%+x]\n", 1e21)
	fmt.Printf("[%-x]\n", 1e21)
	fmt.Printf("[% x]\n", 1e21)
	fmt.Printf("[%0x]\n", 1e21)
	fmt.Printf("[%#x]\n", 1e21)
	fmt.Printf("[%30x]\n", 1e21)
	fmt.Printf("[%-30x]\n", 1e21)
	fmt.Printf("[%030x]\n", 1e21)
	fmt.Printf("[%+030x]\n", 1e21)
	fmt.Printf("[%.0x]\n", 1e21)
	fmt.Printf("[%.1x]\n", 1e21)
	fmt.Printf("[%.3x]\n", 1e21)
	fmt.Printf("[%.10x]\n", 1e21)
	fmt.Printf("[%#.3x]\n", 1e21)
	fmt.Printf("[%#30x]\n", 1e21)
	fmt.Printf("[%x]\n", 1e22)
	fmt.Printf("[%+x]\n", 1e22)
	fmt.Printf("[%-x]\n", 1e22)
	fmt.Printf("[% x]\n", 1e22)
	fmt.Printf("[%0x]\n", 1e22)
	fmt.Printf("[%#x]\n", 1e22)
	fmt.Printf("[%30x]\n", 1e22)
	fmt.Printf("[%-30x]\n", 1e22)
	fmt.Printf("[%030x]\n", 1e22)
	fmt.Printf("[%+030x]\n", 1e22)
	fmt.Printf("[%.0x]\n", 1e22)
	fmt.Printf("[%.1x]\n", 1e22)
	fmt.Printf("[%.3x]\n", 1e22)
	fmt.Printf("[%.10x]\n", 1e22)
	fmt.Printf("[%#.3x]\n", 1e22)
	fmt.Printf("[%#30x]\n", 1e22)
	fmt.Printf("[%x]\n", 123456789.0)
	fmt.Printf("[%+x]\n", 123456789.0)
	fmt.Printf("[%-x]\n", 123456789.0)
	fmt.Printf("[% x]\n", 123456789.0)
	fmt.Printf("[%0x]\n", 123456789.0)
	fmt.Printf("[%#x]\n", 123456789.0)
	fmt.Printf("[%30x]\n", 123456789.0)
	fmt.Printf("[%-30x]\n", 123456789.0)
	fmt.Printf("[%030x]\n", 123456789.0)
	fmt.Printf("[%+030x]\n", 123456789.0)
	fmt.Printf("[%.0x]\n", 123456789.0)
	fmt.Printf("[%.1x]\n", 123456789.0)
	fmt.Printf("[%.3x]\n", 123456789.0)
	fmt.Printf("[%.10x]\n", 123456789.0)
	fmt.Printf("[%#.3x]\n", 123456789.0)
	fmt.Printf("[%#30x]\n", 123456789.0)
	fmt.Printf("[%x]\n", 12345678901234567890.0)
	fmt.Printf("[%+x]\n", 12345678901234567890.0)
	fmt.Printf("[%-x]\n", 12345678901234567890.0)
	fmt.Printf("[% x]\n", 12345678901234567890.0)
	fmt.Printf("[%0x]\n", 12345678901234567890.0)
	fmt.Printf("[%#x]\n", 12345678901234567890.0)
	fmt.Printf("[%30x]\n", 12345678901234567890.0)
	fmt.Printf("[%-30x]\n", 12345678901234567890.0)
	fmt.Printf("[%030x]\n", 12345678901234567890.0)
	fmt.Printf("[%+030x]\n", 12345678901234567890.0)
	fmt.Printf("[%.0x]\n", 12345678901234567890.0)
	fmt.Printf("[%.1x]\n", 12345678901234567890.0)
	fmt.Printf("[%.3x]\n", 12345678901234567890.0)
	fmt.Printf("[%.10x]\n", 12345678901234567890.0)
	fmt.Printf("[%#.3x]\n", 12345678901234567890.0)
	fmt.Printf("[%#30x]\n", 12345678901234567890.0)
	fmt.Printf("[%x]\n", 1e100)
	fmt.Printf("[%+x]\n", 1e100)
	fmt.Printf("[%-x]\n", 1e100)
	fmt.Printf("[% x]\n", 1e100)
	fmt.Printf("[%0x]\n", 1e100)
	fmt.Printf("[%#x]\n", 1e100)
	fmt.Printf("[%30x]\n", 1e100)
	fmt.Printf("[%-30x]\n", 1e100)
	fmt.Printf("[%030x]\n", 1e100)
	fmt.Printf("[%+030x]\n", 1e100)
	fmt.Printf("[%.0x]\n", 1e100)
	fmt.Printf("[%.1x]\n", 1e100)
	fmt.Printf("[%.3x]\n", 1e100)
	fmt.Printf("[%.10x]\n", 1e100)
	fmt.Printf("[%#.3x]\n", 1e100)
	fmt.Printf("[%#30x]\n", 1e100)
	fmt.Printf("[%x]\n", 1.7976931348623157e308)
	fmt.Printf("[%+x]\n", 1.7976931348623157e308)
	fmt.Printf("[%-x]\n", 1.7976931348623157e308)
	fmt.Printf("[% x]\n", 1.7976931348623157e308)
	fmt.Printf("[%0x]\n", 1.7976931348623157e308)
	fmt.Printf("[%#x]\n", 1.7976931348623157e308)
	fmt.Printf("[%30x]\n", 1.7976931348623157e308)
	fmt.Printf("[%-30x]\n", 1.7976931348623157e308)
	fmt.Printf("[%030x]\n", 1.7976931348623157e308)
	fmt.Printf("[%+030x]\n", 1.7976931348623157e308)
	fmt.Printf("[%.0x]\n", 1.7976931348623157e308)
	fmt.Printf("[%.1x]\n", 1.7976931348623157e308)
	fmt.Printf("[%.3x]\n", 1.7976931348623157e308)
	fmt.Printf("[%.10x]\n", 1.7976931348623157e308)
	fmt.Printf("[%#.3x]\n", 1.7976931348623157e308)
	fmt.Printf("[%#30x]\n", 1.7976931348623157e308)
	fmt.Printf("[%x]\n", float64(1<<53))
	fmt.Printf("[%+x]\n", float64(1<<53))
	fmt.Printf("[%-x]\n", float64(1<<53))
	fmt.Printf("[% x]\n", float64(1<<53))
	fmt.Printf("[%0x]\n", float64(1<<53))
	fmt.Printf("[%#x]\n", float64(1<<53))
	fmt.Printf("[%30x]\n", float64(1<<53))
	fmt.Printf("[%-30x]\n", float64(1<<53))
	fmt.Printf("[%030x]\n", float64(1<<53))
	fmt.Printf("[%+030x]\n", float64(1<<53))
	fmt.Printf("[%.0x]\n", float64(1<<53))
	fmt.Printf("[%.1x]\n", float64(1<<53))
	fmt.Printf("[%.3x]\n", float64(1<<53))
	fmt.Printf("[%.10x]\n", float64(1<<53))
	fmt.Printf("[%#.3x]\n", float64(1<<53))
	fmt.Printf("[%#30x]\n", float64(1<<53))
}

// Output:
// [0x1.e848p+19]
// [+0x1.e848p+19]
// [0x1.e848p+19]
// [ 0x1.e848p+19]
// [0x1.e848p+19]
// [0x1.e848p+19]
// [                  0x1.e848p+19]
// [0x1.e848p+19                  ]
// [0000000000000000000x1.e848p+19]
// [+000000000000000000x1.e848p+19]
// [0x1p+20]
// [0x1.fp+19]
// [0x1.e84p+19]
// [0x1.e848000000p+19]
// [0x1.e84p+19]
// [                  0x1.e848p+19]
// [0x1.5af1d78b58c4p+66]
// [+0x1.5af1d78b58c4p+66]
// [0x1.5af1d78b58c4p+66]
// [ 0x1.5af1d78b58c4p+66]
// [0x1.5af1d78b58c4p+66]
// [0x1.5af1d78b58c4p+66]
// [          0x1.5af1d78b58c4p+66]
// [0x1.5af1d78b58c4p+66          ]
// [00000000000x1.5af1d78b58c4p+66]
// [+0000000000x1.5af1d78b58c4p+66]
// [0x1p+66]
// [0x1.6p+66]
// [0x1.5afp+66]
// [0x1.5af1d78b59p+66]
// [0x1.5afp+66]
// [          0x1.5af1d78b58c4p+66]
// [0x1.b1ae4d6e2ef5p+69]
// [+0x1.b1ae4d6e2ef5p+69]
// [0x1.b1ae4d6e2ef5p+69]
// [ 0x1.b1ae4d6e2ef5p+69]
// [0x1.b1ae4d6e2ef5p+69]
// [0x1.b1ae4d6e2ef5p+69]
// [          0x1.b1ae4d6e2ef5p+69]
// [0x1.b1ae4d6e2ef5p+69          ]
// [00000000000x1.b1ae4d6e2ef5p+69]
// [+0000000000x1.b1ae4d6e2ef5p+69]
// [0x1p+70]
// [0x1.bp+69]
// [0x1.b1bp+69]
// [0x1.b1ae4d6e2fp+69]
// [0x1.b1bp+69]
// [          0x1.b1ae4d6e2ef5p+69]
// [0x1.0f0cf064dd592p+73]
// [+0x1.0f0cf064dd592p+73]
// [0x1.0f0cf064dd592p+73]
// [ 0x1.0f0cf064dd592p+73]
// [0x1.0f0cf064dd592p+73]
// [0x1.0f0cf064dd592p+73]
// [         0x1.0f0cf064dd592p+73]
// [0x1.0f0cf064dd592p+73         ]
// [0000000000x1.0f0cf064dd592p+73]
// [+000000000x1.0f0cf064dd592p+73]
// [0x1p+73]
// [0x1.1p+73]
// [0x1.0f1p+73]
// [0x1.0f0cf064ddp+73]
// [0x1.0f1p+73]
// [         0x1.0f0cf064dd592p+73]
// [0x1.d6f3454p+26]
// [+0x1.d6f3454p+26]
// [0x1.d6f3454p+26]
// [ 0x1.d6f3454p+26]
// [0x1.d6f3454p+26]
// [0x1.d6f3454p+26]
// [               0x1.d6f3454p+26]
// [0x1.d6f3454p+26               ]
// [0000000000000000x1.d6f3454p+26]
// [+000000000000000x1.d6f3454p+26]
// [0x1p+27]
// [0x1.dp+26]
// [0x1.d6fp+26]
// [0x1.d6f3454000p+26]
// [0x1.d6fp+26]
// [               0x1.d6f3454p+26]
// [0x1.56a95319d63e1p+63]
// [+0x1.56a95319d63e1p+63]
// [0x1.56a95319d63e1p+63]
// [ 0x1.56a95319d63e1p+63]
// [0x1.56a95319d63e1p+63]
// [0x1.56a95319d63e1p+63]
// [         0x1.56a95319d63e1p+63]
// [0x1.56a95319d63e1p+63         ]
// [0000000000x1.56a95319d63e1p+63]
// [+000000000x1.56a95319d63e1p+63]
// [0x1p+63]
// [0x1.5p+63]
// [0x1.56bp+63]
// [0x1.56a95319d6p+63]
// [0x1.56bp+63]
// [         0x1.56a95319d63e1p+63]
// [0x1.249ad2594c37dp+332]
// [+0x1.249ad2594c37dp+332]
// [0x1.249ad2594c37dp+332]
// [ 0x1.249ad2594c37dp+332]
// [0x1.249ad2594c37dp+332]
// [0x1.249ad2594c37dp+332]
// [        0x1.249ad2594c37dp+332]
// [0x1.249ad2594c37dp+332        ]
// [000000000x1.249ad2594c37dp+332]
// [+00000000x1.249ad2594c37dp+332]
// [0x1p+332]
// [0x1.2p+332]
// [0x1.24ap+332]
// [0x1.249ad2594cp+332]
// [0x1.24ap+332]
// [        0x1.249ad2594c37dp+332]
// [0x1.fffffffffffffp+1023]
// [+0x1.fffffffffffffp+1023]
// [0x1.fffffffffffffp+1023]
// [ 0x1.fffffffffffffp+1023]
// [0x1.fffffffffffffp+1023]
// [0x1.fffffffffffffp+1023]
// [       0x1.fffffffffffffp+1023]
// [0x1.fffffffffffffp+1023       ]
// [00000000x1.fffffffffffffp+1023]
// [+0000000x1.fffffffffffffp+1023]
// [0x1p+1024]
// [0x1.0p+1024]
// [0x1.000p+1024]
// [0x1.0000000000p+1024]
// [0x1.000p+1024]
// [       0x1.fffffffffffffp+1023]
// [0x1p+53]
// [+0x1p+53]
// [0x1p+53]
// [ 0x1p+53]
// [0x1p+53]
// [0x1.0000p+53]
// [                       0x1p+53]
// [0x1p+53                       ]
// [000000000000000000000000x1p+53]
// [+00000000000000000000000x1p+53]
// [0x1p+53]
// [0x1.0p+53]
// [0x1.000p+53]
// [0x1.0000000000p+53]
// [0x1.000p+53]
// [                  0x1.0000p+53]
