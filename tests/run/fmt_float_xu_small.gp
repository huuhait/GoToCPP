// phase: 05

// Go's %X of float64 values (small) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", 1e-4)
	fmt.Printf("[%+X]\n", 1e-4)
	fmt.Printf("[%-X]\n", 1e-4)
	fmt.Printf("[% X]\n", 1e-4)
	fmt.Printf("[%0X]\n", 1e-4)
	fmt.Printf("[%#X]\n", 1e-4)
	fmt.Printf("[%30X]\n", 1e-4)
	fmt.Printf("[%-30X]\n", 1e-4)
	fmt.Printf("[%030X]\n", 1e-4)
	fmt.Printf("[%+030X]\n", 1e-4)
	fmt.Printf("[%.0X]\n", 1e-4)
	fmt.Printf("[%.1X]\n", 1e-4)
	fmt.Printf("[%.3X]\n", 1e-4)
	fmt.Printf("[%.10X]\n", 1e-4)
	fmt.Printf("[%#.3X]\n", 1e-4)
	fmt.Printf("[%#30X]\n", 1e-4)
	fmt.Printf("[%X]\n", 1e-5)
	fmt.Printf("[%+X]\n", 1e-5)
	fmt.Printf("[%-X]\n", 1e-5)
	fmt.Printf("[% X]\n", 1e-5)
	fmt.Printf("[%0X]\n", 1e-5)
	fmt.Printf("[%#X]\n", 1e-5)
	fmt.Printf("[%30X]\n", 1e-5)
	fmt.Printf("[%-30X]\n", 1e-5)
	fmt.Printf("[%030X]\n", 1e-5)
	fmt.Printf("[%+030X]\n", 1e-5)
	fmt.Printf("[%.0X]\n", 1e-5)
	fmt.Printf("[%.1X]\n", 1e-5)
	fmt.Printf("[%.3X]\n", 1e-5)
	fmt.Printf("[%.10X]\n", 1e-5)
	fmt.Printf("[%#.3X]\n", 1e-5)
	fmt.Printf("[%#30X]\n", 1e-5)
	fmt.Printf("[%X]\n", 0.0001234)
	fmt.Printf("[%+X]\n", 0.0001234)
	fmt.Printf("[%-X]\n", 0.0001234)
	fmt.Printf("[% X]\n", 0.0001234)
	fmt.Printf("[%0X]\n", 0.0001234)
	fmt.Printf("[%#X]\n", 0.0001234)
	fmt.Printf("[%30X]\n", 0.0001234)
	fmt.Printf("[%-30X]\n", 0.0001234)
	fmt.Printf("[%030X]\n", 0.0001234)
	fmt.Printf("[%+030X]\n", 0.0001234)
	fmt.Printf("[%.0X]\n", 0.0001234)
	fmt.Printf("[%.1X]\n", 0.0001234)
	fmt.Printf("[%.3X]\n", 0.0001234)
	fmt.Printf("[%.10X]\n", 0.0001234)
	fmt.Printf("[%#.3X]\n", 0.0001234)
	fmt.Printf("[%#30X]\n", 0.0001234)
	fmt.Printf("[%X]\n", 1e-7)
	fmt.Printf("[%+X]\n", 1e-7)
	fmt.Printf("[%-X]\n", 1e-7)
	fmt.Printf("[% X]\n", 1e-7)
	fmt.Printf("[%0X]\n", 1e-7)
	fmt.Printf("[%#X]\n", 1e-7)
	fmt.Printf("[%30X]\n", 1e-7)
	fmt.Printf("[%-30X]\n", 1e-7)
	fmt.Printf("[%030X]\n", 1e-7)
	fmt.Printf("[%+030X]\n", 1e-7)
	fmt.Printf("[%.0X]\n", 1e-7)
	fmt.Printf("[%.1X]\n", 1e-7)
	fmt.Printf("[%.3X]\n", 1e-7)
	fmt.Printf("[%.10X]\n", 1e-7)
	fmt.Printf("[%#.3X]\n", 1e-7)
	fmt.Printf("[%#30X]\n", 1e-7)
	fmt.Printf("[%X]\n", 5e-324)
	fmt.Printf("[%+X]\n", 5e-324)
	fmt.Printf("[%-X]\n", 5e-324)
	fmt.Printf("[% X]\n", 5e-324)
	fmt.Printf("[%0X]\n", 5e-324)
	fmt.Printf("[%#X]\n", 5e-324)
	fmt.Printf("[%30X]\n", 5e-324)
	fmt.Printf("[%-30X]\n", 5e-324)
	fmt.Printf("[%030X]\n", 5e-324)
	fmt.Printf("[%+030X]\n", 5e-324)
	fmt.Printf("[%.0X]\n", 5e-324)
	fmt.Printf("[%.1X]\n", 5e-324)
	fmt.Printf("[%.3X]\n", 5e-324)
	fmt.Printf("[%.10X]\n", 5e-324)
	fmt.Printf("[%#.3X]\n", 5e-324)
	fmt.Printf("[%#30X]\n", 5e-324)
	fmt.Printf("[%X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%+X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%-X]\n", 2.2250738585072014e-308)
	fmt.Printf("[% X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%0X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%#X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%30X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%-30X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%030X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%+030X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.0X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.1X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.3X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%.10X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%#.3X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%#30X]\n", 2.2250738585072014e-308)
	fmt.Printf("[%X]\n", 1.5e-10)
	fmt.Printf("[%+X]\n", 1.5e-10)
	fmt.Printf("[%-X]\n", 1.5e-10)
	fmt.Printf("[% X]\n", 1.5e-10)
	fmt.Printf("[%0X]\n", 1.5e-10)
	fmt.Printf("[%#X]\n", 1.5e-10)
	fmt.Printf("[%30X]\n", 1.5e-10)
	fmt.Printf("[%-30X]\n", 1.5e-10)
	fmt.Printf("[%030X]\n", 1.5e-10)
	fmt.Printf("[%+030X]\n", 1.5e-10)
	fmt.Printf("[%.0X]\n", 1.5e-10)
	fmt.Printf("[%.1X]\n", 1.5e-10)
	fmt.Printf("[%.3X]\n", 1.5e-10)
	fmt.Printf("[%.10X]\n", 1.5e-10)
	fmt.Printf("[%#.3X]\n", 1.5e-10)
	fmt.Printf("[%#30X]\n", 1.5e-10)
	fmt.Printf("[%X]\n", 0.1)
	fmt.Printf("[%+X]\n", 0.1)
	fmt.Printf("[%-X]\n", 0.1)
	fmt.Printf("[% X]\n", 0.1)
	fmt.Printf("[%0X]\n", 0.1)
	fmt.Printf("[%#X]\n", 0.1)
	fmt.Printf("[%30X]\n", 0.1)
	fmt.Printf("[%-30X]\n", 0.1)
	fmt.Printf("[%030X]\n", 0.1)
	fmt.Printf("[%+030X]\n", 0.1)
	fmt.Printf("[%.0X]\n", 0.1)
	fmt.Printf("[%.1X]\n", 0.1)
	fmt.Printf("[%.3X]\n", 0.1)
	fmt.Printf("[%.10X]\n", 0.1)
	fmt.Printf("[%#.3X]\n", 0.1)
	fmt.Printf("[%#30X]\n", 0.1)
	fmt.Printf("[%X]\n", 0.2+0.1)
	fmt.Printf("[%+X]\n", 0.2+0.1)
	fmt.Printf("[%-X]\n", 0.2+0.1)
	fmt.Printf("[% X]\n", 0.2+0.1)
	fmt.Printf("[%0X]\n", 0.2+0.1)
	fmt.Printf("[%#X]\n", 0.2+0.1)
	fmt.Printf("[%30X]\n", 0.2+0.1)
	fmt.Printf("[%-30X]\n", 0.2+0.1)
	fmt.Printf("[%030X]\n", 0.2+0.1)
	fmt.Printf("[%+030X]\n", 0.2+0.1)
	fmt.Printf("[%.0X]\n", 0.2+0.1)
	fmt.Printf("[%.1X]\n", 0.2+0.1)
	fmt.Printf("[%.3X]\n", 0.2+0.1)
	fmt.Printf("[%.10X]\n", 0.2+0.1)
	fmt.Printf("[%#.3X]\n", 0.2+0.1)
	fmt.Printf("[%#30X]\n", 0.2+0.1)
}

// Output:
// [0X1.A36E2EB1C432DP-14]
// [+0X1.A36E2EB1C432DP-14]
// [0X1.A36E2EB1C432DP-14]
// [ 0X1.A36E2EB1C432DP-14]
// [0X1.A36E2EB1C432DP-14]
// [0X1.A36E2EB1C432DP-14]
// [         0X1.A36E2EB1C432DP-14]
// [0X1.A36E2EB1C432DP-14         ]
// [0000000000X1.A36E2EB1C432DP-14]
// [+000000000X1.A36E2EB1C432DP-14]
// [0X1P-13]
// [0X1.AP-14]
// [0X1.A37P-14]
// [0X1.A36E2EB1C4P-14]
// [0X1.A37P-14]
// [         0X1.A36E2EB1C432DP-14]
// [0X1.4F8B588E368F1P-17]
// [+0X1.4F8B588E368F1P-17]
// [0X1.4F8B588E368F1P-17]
// [ 0X1.4F8B588E368F1P-17]
// [0X1.4F8B588E368F1P-17]
// [0X1.4F8B588E368F1P-17]
// [         0X1.4F8B588E368F1P-17]
// [0X1.4F8B588E368F1P-17         ]
// [0000000000X1.4F8B588E368F1P-17]
// [+000000000X1.4F8B588E368F1P-17]
// [0X1P-17]
// [0X1.5P-17]
// [0X1.4F9P-17]
// [0X1.4F8B588E37P-17]
// [0X1.4F9P-17]
// [         0X1.4F8B588E368F1P-17]
// [0X1.02C9DEDBC309DP-13]
// [+0X1.02C9DEDBC309DP-13]
// [0X1.02C9DEDBC309DP-13]
// [ 0X1.02C9DEDBC309DP-13]
// [0X1.02C9DEDBC309DP-13]
// [0X1.02C9DEDBC309DP-13]
// [         0X1.02C9DEDBC309DP-13]
// [0X1.02C9DEDBC309DP-13         ]
// [0000000000X1.02C9DEDBC309DP-13]
// [+000000000X1.02C9DEDBC309DP-13]
// [0X1P-13]
// [0X1.0P-13]
// [0X1.02DP-13]
// [0X1.02C9DEDBC3P-13]
// [0X1.02DP-13]
// [         0X1.02C9DEDBC309DP-13]
// [0X1.AD7F29ABCAF48P-24]
// [+0X1.AD7F29ABCAF48P-24]
// [0X1.AD7F29ABCAF48P-24]
// [ 0X1.AD7F29ABCAF48P-24]
// [0X1.AD7F29ABCAF48P-24]
// [0X1.AD7F29ABCAF48P-24]
// [         0X1.AD7F29ABCAF48P-24]
// [0X1.AD7F29ABCAF48P-24         ]
// [0000000000X1.AD7F29ABCAF48P-24]
// [+000000000X1.AD7F29ABCAF48P-24]
// [0X1P-23]
// [0X1.BP-24]
// [0X1.AD8P-24]
// [0X1.AD7F29ABCBP-24]
// [0X1.AD8P-24]
// [         0X1.AD7F29ABCAF48P-24]
// [0X1P-1074]
// [+0X1P-1074]
// [0X1P-1074]
// [ 0X1P-1074]
// [0X1P-1074]
// [0X1.P-1074]
// [                     0X1P-1074]
// [0X1P-1074                     ]
// [0000000000000000000000X1P-1074]
// [+000000000000000000000X1P-1074]
// [0X1P-1074]
// [0X1.0P-1074]
// [0X1.000P-1074]
// [0X1.0000000000P-1074]
// [0X1.000P-1074]
// [                    0X1.P-1074]
// [0X1P-1022]
// [+0X1P-1022]
// [0X1P-1022]
// [ 0X1P-1022]
// [0X1P-1022]
// [0X1.P-1022]
// [                     0X1P-1022]
// [0X1P-1022                     ]
// [0000000000000000000000X1P-1022]
// [+000000000000000000000X1P-1022]
// [0X1P-1022]
// [0X1.0P-1022]
// [0X1.000P-1022]
// [0X1.0000000000P-1022]
// [0X1.000P-1022]
// [                    0X1.P-1022]
// [0X1.49DA7E361CE4CP-33]
// [+0X1.49DA7E361CE4CP-33]
// [0X1.49DA7E361CE4CP-33]
// [ 0X1.49DA7E361CE4CP-33]
// [0X1.49DA7E361CE4CP-33]
// [0X1.49DA7E361CE4CP-33]
// [         0X1.49DA7E361CE4CP-33]
// [0X1.49DA7E361CE4CP-33         ]
// [0000000000X1.49DA7E361CE4CP-33]
// [+000000000X1.49DA7E361CE4CP-33]
// [0X1P-33]
// [0X1.5P-33]
// [0X1.49EP-33]
// [0X1.49DA7E361DP-33]
// [0X1.49EP-33]
// [         0X1.49DA7E361CE4CP-33]
// [0X1.999999999999AP-04]
// [+0X1.999999999999AP-04]
// [0X1.999999999999AP-04]
// [ 0X1.999999999999AP-04]
// [0X1.999999999999AP-04]
// [0X1.999999999999AP-04]
// [         0X1.999999999999AP-04]
// [0X1.999999999999AP-04         ]
// [0000000000X1.999999999999AP-04]
// [+000000000X1.999999999999AP-04]
// [0X1P-03]
// [0X1.AP-04]
// [0X1.99AP-04]
// [0X1.999999999AP-04]
// [0X1.99AP-04]
// [         0X1.999999999999AP-04]
// [0X1.3333333333333P-02]
// [+0X1.3333333333333P-02]
// [0X1.3333333333333P-02]
// [ 0X1.3333333333333P-02]
// [0X1.3333333333333P-02]
// [0X1.3333333333333P-02]
// [         0X1.3333333333333P-02]
// [0X1.3333333333333P-02         ]
// [0000000000X1.3333333333333P-02]
// [+000000000X1.3333333333333P-02]
// [0X1P-02]
// [0X1.3P-02]
// [0X1.333P-02]
// [0X1.3333333333P-02]
// [0X1.333P-02]
// [         0X1.3333333333333P-02]
