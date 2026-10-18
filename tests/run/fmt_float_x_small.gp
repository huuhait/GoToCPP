// phase: 05

// Go's %x of float64 values (small) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", 1e-4)
	fmt.Printf("[%+x]\n", 1e-4)
	fmt.Printf("[%-x]\n", 1e-4)
	fmt.Printf("[% x]\n", 1e-4)
	fmt.Printf("[%0x]\n", 1e-4)
	fmt.Printf("[%#x]\n", 1e-4)
	fmt.Printf("[%30x]\n", 1e-4)
	fmt.Printf("[%-30x]\n", 1e-4)
	fmt.Printf("[%030x]\n", 1e-4)
	fmt.Printf("[%+030x]\n", 1e-4)
	fmt.Printf("[%.0x]\n", 1e-4)
	fmt.Printf("[%.1x]\n", 1e-4)
	fmt.Printf("[%.3x]\n", 1e-4)
	fmt.Printf("[%.10x]\n", 1e-4)
	fmt.Printf("[%#.3x]\n", 1e-4)
	fmt.Printf("[%#30x]\n", 1e-4)
	fmt.Printf("[%x]\n", 1e-5)
	fmt.Printf("[%+x]\n", 1e-5)
	fmt.Printf("[%-x]\n", 1e-5)
	fmt.Printf("[% x]\n", 1e-5)
	fmt.Printf("[%0x]\n", 1e-5)
	fmt.Printf("[%#x]\n", 1e-5)
	fmt.Printf("[%30x]\n", 1e-5)
	fmt.Printf("[%-30x]\n", 1e-5)
	fmt.Printf("[%030x]\n", 1e-5)
	fmt.Printf("[%+030x]\n", 1e-5)
	fmt.Printf("[%.0x]\n", 1e-5)
	fmt.Printf("[%.1x]\n", 1e-5)
	fmt.Printf("[%.3x]\n", 1e-5)
	fmt.Printf("[%.10x]\n", 1e-5)
	fmt.Printf("[%#.3x]\n", 1e-5)
	fmt.Printf("[%#30x]\n", 1e-5)
	fmt.Printf("[%x]\n", 0.0001234)
	fmt.Printf("[%+x]\n", 0.0001234)
	fmt.Printf("[%-x]\n", 0.0001234)
	fmt.Printf("[% x]\n", 0.0001234)
	fmt.Printf("[%0x]\n", 0.0001234)
	fmt.Printf("[%#x]\n", 0.0001234)
	fmt.Printf("[%30x]\n", 0.0001234)
	fmt.Printf("[%-30x]\n", 0.0001234)
	fmt.Printf("[%030x]\n", 0.0001234)
	fmt.Printf("[%+030x]\n", 0.0001234)
	fmt.Printf("[%.0x]\n", 0.0001234)
	fmt.Printf("[%.1x]\n", 0.0001234)
	fmt.Printf("[%.3x]\n", 0.0001234)
	fmt.Printf("[%.10x]\n", 0.0001234)
	fmt.Printf("[%#.3x]\n", 0.0001234)
	fmt.Printf("[%#30x]\n", 0.0001234)
	fmt.Printf("[%x]\n", 1e-7)
	fmt.Printf("[%+x]\n", 1e-7)
	fmt.Printf("[%-x]\n", 1e-7)
	fmt.Printf("[% x]\n", 1e-7)
	fmt.Printf("[%0x]\n", 1e-7)
	fmt.Printf("[%#x]\n", 1e-7)
	fmt.Printf("[%30x]\n", 1e-7)
	fmt.Printf("[%-30x]\n", 1e-7)
	fmt.Printf("[%030x]\n", 1e-7)
	fmt.Printf("[%+030x]\n", 1e-7)
	fmt.Printf("[%.0x]\n", 1e-7)
	fmt.Printf("[%.1x]\n", 1e-7)
	fmt.Printf("[%.3x]\n", 1e-7)
	fmt.Printf("[%.10x]\n", 1e-7)
	fmt.Printf("[%#.3x]\n", 1e-7)
	fmt.Printf("[%#30x]\n", 1e-7)
	fmt.Printf("[%x]\n", 5e-324)
	fmt.Printf("[%+x]\n", 5e-324)
	fmt.Printf("[%-x]\n", 5e-324)
	fmt.Printf("[% x]\n", 5e-324)
	fmt.Printf("[%0x]\n", 5e-324)
	fmt.Printf("[%#x]\n", 5e-324)
	fmt.Printf("[%30x]\n", 5e-324)
	fmt.Printf("[%-30x]\n", 5e-324)
	fmt.Printf("[%030x]\n", 5e-324)
	fmt.Printf("[%+030x]\n", 5e-324)
	fmt.Printf("[%.0x]\n", 5e-324)
	fmt.Printf("[%.1x]\n", 5e-324)
	fmt.Printf("[%.3x]\n", 5e-324)
	fmt.Printf("[%.10x]\n", 5e-324)
	fmt.Printf("[%#.3x]\n", 5e-324)
	fmt.Printf("[%#30x]\n", 5e-324)
	fmt.Printf("[%x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%+x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%-x]\n", 2.2250738585072014e-308)
	fmt.Printf("[% x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%0x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%#x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%30x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%-30x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%030x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%+030x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.0x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.1x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.3x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.10x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%#.3x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%#30x]\n", 2.2250738585072014e-308)
	fmt.Printf("[%x]\n", 1.5e-10)
	fmt.Printf("[%+x]\n", 1.5e-10)
	fmt.Printf("[%-x]\n", 1.5e-10)
	fmt.Printf("[% x]\n", 1.5e-10)
	fmt.Printf("[%0x]\n", 1.5e-10)
	fmt.Printf("[%#x]\n", 1.5e-10)
	fmt.Printf("[%30x]\n", 1.5e-10)
	fmt.Printf("[%-30x]\n", 1.5e-10)
	fmt.Printf("[%030x]\n", 1.5e-10)
	fmt.Printf("[%+030x]\n", 1.5e-10)
	fmt.Printf("[%.0x]\n", 1.5e-10)
	fmt.Printf("[%.1x]\n", 1.5e-10)
	fmt.Printf("[%.3x]\n", 1.5e-10)
	fmt.Printf("[%.10x]\n", 1.5e-10)
	fmt.Printf("[%#.3x]\n", 1.5e-10)
	fmt.Printf("[%#30x]\n", 1.5e-10)
	fmt.Printf("[%x]\n", 0.1)
	fmt.Printf("[%+x]\n", 0.1)
	fmt.Printf("[%-x]\n", 0.1)
	fmt.Printf("[% x]\n", 0.1)
	fmt.Printf("[%0x]\n", 0.1)
	fmt.Printf("[%#x]\n", 0.1)
	fmt.Printf("[%30x]\n", 0.1)
	fmt.Printf("[%-30x]\n", 0.1)
	fmt.Printf("[%030x]\n", 0.1)
	fmt.Printf("[%+030x]\n", 0.1)
	fmt.Printf("[%.0x]\n", 0.1)
	fmt.Printf("[%.1x]\n", 0.1)
	fmt.Printf("[%.3x]\n", 0.1)
	fmt.Printf("[%.10x]\n", 0.1)
	fmt.Printf("[%#.3x]\n", 0.1)
	fmt.Printf("[%#30x]\n", 0.1)
	fmt.Printf("[%x]\n", 0.2+0.1)
	fmt.Printf("[%+x]\n", 0.2+0.1)
	fmt.Printf("[%-x]\n", 0.2+0.1)
	fmt.Printf("[% x]\n", 0.2+0.1)
	fmt.Printf("[%0x]\n", 0.2+0.1)
	fmt.Printf("[%#x]\n", 0.2+0.1)
	fmt.Printf("[%30x]\n", 0.2+0.1)
	fmt.Printf("[%-30x]\n", 0.2+0.1)
	fmt.Printf("[%030x]\n", 0.2+0.1)
	fmt.Printf("[%+030x]\n", 0.2+0.1)
	fmt.Printf("[%.0x]\n", 0.2+0.1)
	fmt.Printf("[%.1x]\n", 0.2+0.1)
	fmt.Printf("[%.3x]\n", 0.2+0.1)
	fmt.Printf("[%.10x]\n", 0.2+0.1)
	fmt.Printf("[%#.3x]\n", 0.2+0.1)
	fmt.Printf("[%#30x]\n", 0.2+0.1)
}

// Output:
// [0x1.a36e2eb1c432dp-14]
// [+0x1.a36e2eb1c432dp-14]
// [0x1.a36e2eb1c432dp-14]
// [ 0x1.a36e2eb1c432dp-14]
// [0x1.a36e2eb1c432dp-14]
// [0x1.a36e2eb1c432dp-14]
// [         0x1.a36e2eb1c432dp-14]
// [0x1.a36e2eb1c432dp-14         ]
// [0000000000x1.a36e2eb1c432dp-14]
// [+000000000x1.a36e2eb1c432dp-14]
// [0x1p-13]
// [0x1.ap-14]
// [0x1.a37p-14]
// [0x1.a36e2eb1c4p-14]
// [0x1.a37p-14]
// [         0x1.a36e2eb1c432dp-14]
// [0x1.4f8b588e368f1p-17]
// [+0x1.4f8b588e368f1p-17]
// [0x1.4f8b588e368f1p-17]
// [ 0x1.4f8b588e368f1p-17]
// [0x1.4f8b588e368f1p-17]
// [0x1.4f8b588e368f1p-17]
// [         0x1.4f8b588e368f1p-17]
// [0x1.4f8b588e368f1p-17         ]
// [0000000000x1.4f8b588e368f1p-17]
// [+000000000x1.4f8b588e368f1p-17]
// [0x1p-17]
// [0x1.5p-17]
// [0x1.4f9p-17]
// [0x1.4f8b588e37p-17]
// [0x1.4f9p-17]
// [         0x1.4f8b588e368f1p-17]
// [0x1.02c9dedbc309dp-13]
// [+0x1.02c9dedbc309dp-13]
// [0x1.02c9dedbc309dp-13]
// [ 0x1.02c9dedbc309dp-13]
// [0x1.02c9dedbc309dp-13]
// [0x1.02c9dedbc309dp-13]
// [         0x1.02c9dedbc309dp-13]
// [0x1.02c9dedbc309dp-13         ]
// [0000000000x1.02c9dedbc309dp-13]
// [+000000000x1.02c9dedbc309dp-13]
// [0x1p-13]
// [0x1.0p-13]
// [0x1.02dp-13]
// [0x1.02c9dedbc3p-13]
// [0x1.02dp-13]
// [         0x1.02c9dedbc309dp-13]
// [0x1.ad7f29abcaf48p-24]
// [+0x1.ad7f29abcaf48p-24]
// [0x1.ad7f29abcaf48p-24]
// [ 0x1.ad7f29abcaf48p-24]
// [0x1.ad7f29abcaf48p-24]
// [0x1.ad7f29abcaf48p-24]
// [         0x1.ad7f29abcaf48p-24]
// [0x1.ad7f29abcaf48p-24         ]
// [0000000000x1.ad7f29abcaf48p-24]
// [+000000000x1.ad7f29abcaf48p-24]
// [0x1p-23]
// [0x1.bp-24]
// [0x1.ad8p-24]
// [0x1.ad7f29abcbp-24]
// [0x1.ad8p-24]
// [         0x1.ad7f29abcaf48p-24]
// [0x1p-1074]
// [+0x1p-1074]
// [0x1p-1074]
// [ 0x1p-1074]
// [0x1p-1074]
// [0x1.0000p-1074]
// [                     0x1p-1074]
// [0x1p-1074                     ]
// [0000000000000000000000x1p-1074]
// [+000000000000000000000x1p-1074]
// [0x1p-1074]
// [0x1.0p-1074]
// [0x1.000p-1074]
// [0x1.0000000000p-1074]
// [0x1.000p-1074]
// [                0x1.0000p-1074]
// [0x1p-1022]
// [+0x1p-1022]
// [0x1p-1022]
// [ 0x1p-1022]
// [0x1p-1022]
// [0x1.0000p-1022]
// [                     0x1p-1022]
// [0x1p-1022                     ]
// [0000000000000000000000x1p-1022]
// [+000000000000000000000x1p-1022]
// [0x1p-1022]
// [0x1.0p-1022]
// [0x1.000p-1022]
// [0x1.0000000000p-1022]
// [0x1.000p-1022]
// [                0x1.0000p-1022]
// [0x1.49da7e361ce4cp-33]
// [+0x1.49da7e361ce4cp-33]
// [0x1.49da7e361ce4cp-33]
// [ 0x1.49da7e361ce4cp-33]
// [0x1.49da7e361ce4cp-33]
// [0x1.49da7e361ce4cp-33]
// [         0x1.49da7e361ce4cp-33]
// [0x1.49da7e361ce4cp-33         ]
// [0000000000x1.49da7e361ce4cp-33]
// [+000000000x1.49da7e361ce4cp-33]
// [0x1p-33]
// [0x1.5p-33]
// [0x1.49ep-33]
// [0x1.49da7e361dp-33]
// [0x1.49ep-33]
// [         0x1.49da7e361ce4cp-33]
// [0x1.999999999999ap-04]
// [+0x1.999999999999ap-04]
// [0x1.999999999999ap-04]
// [ 0x1.999999999999ap-04]
// [0x1.999999999999ap-04]
// [0x1.999999999999ap-04]
// [         0x1.999999999999ap-04]
// [0x1.999999999999ap-04         ]
// [0000000000x1.999999999999ap-04]
// [+000000000x1.999999999999ap-04]
// [0x1p-03]
// [0x1.ap-04]
// [0x1.99ap-04]
// [0x1.999999999ap-04]
// [0x1.99ap-04]
// [         0x1.999999999999ap-04]
// [0x1.3333333333333p-02]
// [+0x1.3333333333333p-02]
// [0x1.3333333333333p-02]
// [ 0x1.3333333333333p-02]
// [0x1.3333333333333p-02]
// [0x1.3333333333333p-02]
// [         0x1.3333333333333p-02]
// [0x1.3333333333333p-02         ]
// [0000000000x1.3333333333333p-02]
// [+000000000x1.3333333333333p-02]
// [0x1p-02]
// [0x1.3p-02]
// [0x1.333p-02]
// [0x1.3333333333p-02]
// [0x1.333p-02]
// [         0x1.3333333333333p-02]
