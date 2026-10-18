// phase: 05

// Go's %X of float64 values (round) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%X]\n", 2.675)
	fmt.Printf("[%+X]\n", 2.675)
	fmt.Printf("[%-X]\n", 2.675)
	fmt.Printf("[% X]\n", 2.675)
	fmt.Printf("[%0X]\n", 2.675)
	fmt.Printf("[%#X]\n", 2.675)
	fmt.Printf("[%30X]\n", 2.675)
	fmt.Printf("[%-30X]\n", 2.675)
	fmt.Printf("[%030X]\n", 2.675)
	fmt.Printf("[%+030X]\n", 2.675)
	fmt.Printf("[%.0X]\n", 2.675)
	fmt.Printf("[%.1X]\n", 2.675)
	fmt.Printf("[%.3X]\n", 2.675)
	fmt.Printf("[%.10X]\n", 2.675)
	fmt.Printf("[%#.3X]\n", 2.675)
	fmt.Printf("[%#30X]\n", 2.675)
	fmt.Printf("[%X]\n", 1.005)
	fmt.Printf("[%+X]\n", 1.005)
	fmt.Printf("[%-X]\n", 1.005)
	fmt.Printf("[% X]\n", 1.005)
	fmt.Printf("[%0X]\n", 1.005)
	fmt.Printf("[%#X]\n", 1.005)
	fmt.Printf("[%30X]\n", 1.005)
	fmt.Printf("[%-30X]\n", 1.005)
	fmt.Printf("[%030X]\n", 1.005)
	fmt.Printf("[%+030X]\n", 1.005)
	fmt.Printf("[%.0X]\n", 1.005)
	fmt.Printf("[%.1X]\n", 1.005)
	fmt.Printf("[%.3X]\n", 1.005)
	fmt.Printf("[%.10X]\n", 1.005)
	fmt.Printf("[%#.3X]\n", 1.005)
	fmt.Printf("[%#30X]\n", 1.005)
	fmt.Printf("[%X]\n", 0.125)
	fmt.Printf("[%+X]\n", 0.125)
	fmt.Printf("[%-X]\n", 0.125)
	fmt.Printf("[% X]\n", 0.125)
	fmt.Printf("[%0X]\n", 0.125)
	fmt.Printf("[%#X]\n", 0.125)
	fmt.Printf("[%30X]\n", 0.125)
	fmt.Printf("[%-30X]\n", 0.125)
	fmt.Printf("[%030X]\n", 0.125)
	fmt.Printf("[%+030X]\n", 0.125)
	fmt.Printf("[%.0X]\n", 0.125)
	fmt.Printf("[%.1X]\n", 0.125)
	fmt.Printf("[%.3X]\n", 0.125)
	fmt.Printf("[%.10X]\n", 0.125)
	fmt.Printf("[%#.3X]\n", 0.125)
	fmt.Printf("[%#30X]\n", 0.125)
	fmt.Printf("[%X]\n", 0.375)
	fmt.Printf("[%+X]\n", 0.375)
	fmt.Printf("[%-X]\n", 0.375)
	fmt.Printf("[% X]\n", 0.375)
	fmt.Printf("[%0X]\n", 0.375)
	fmt.Printf("[%#X]\n", 0.375)
	fmt.Printf("[%30X]\n", 0.375)
	fmt.Printf("[%-30X]\n", 0.375)
	fmt.Printf("[%030X]\n", 0.375)
	fmt.Printf("[%+030X]\n", 0.375)
	fmt.Printf("[%.0X]\n", 0.375)
	fmt.Printf("[%.1X]\n", 0.375)
	fmt.Printf("[%.3X]\n", 0.375)
	fmt.Printf("[%.10X]\n", 0.375)
	fmt.Printf("[%#.3X]\n", 0.375)
	fmt.Printf("[%#30X]\n", 0.375)
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
	fmt.Printf("[%X]\n", 3.5)
	fmt.Printf("[%+X]\n", 3.5)
	fmt.Printf("[%-X]\n", 3.5)
	fmt.Printf("[% X]\n", 3.5)
	fmt.Printf("[%0X]\n", 3.5)
	fmt.Printf("[%#X]\n", 3.5)
	fmt.Printf("[%30X]\n", 3.5)
	fmt.Printf("[%-30X]\n", 3.5)
	fmt.Printf("[%030X]\n", 3.5)
	fmt.Printf("[%+030X]\n", 3.5)
	fmt.Printf("[%.0X]\n", 3.5)
	fmt.Printf("[%.1X]\n", 3.5)
	fmt.Printf("[%.3X]\n", 3.5)
	fmt.Printf("[%.10X]\n", 3.5)
	fmt.Printf("[%#.3X]\n", 3.5)
	fmt.Printf("[%#30X]\n", 3.5)
	fmt.Printf("[%X]\n", -2.5)
	fmt.Printf("[%+X]\n", -2.5)
	fmt.Printf("[%-X]\n", -2.5)
	fmt.Printf("[% X]\n", -2.5)
	fmt.Printf("[%0X]\n", -2.5)
	fmt.Printf("[%#X]\n", -2.5)
	fmt.Printf("[%30X]\n", -2.5)
	fmt.Printf("[%-30X]\n", -2.5)
	fmt.Printf("[%030X]\n", -2.5)
	fmt.Printf("[%+030X]\n", -2.5)
	fmt.Printf("[%.0X]\n", -2.5)
	fmt.Printf("[%.1X]\n", -2.5)
	fmt.Printf("[%.3X]\n", -2.5)
	fmt.Printf("[%.10X]\n", -2.5)
	fmt.Printf("[%#.3X]\n", -2.5)
	fmt.Printf("[%#30X]\n", -2.5)
	fmt.Printf("[%X]\n", 0.045)
	fmt.Printf("[%+X]\n", 0.045)
	fmt.Printf("[%-X]\n", 0.045)
	fmt.Printf("[% X]\n", 0.045)
	fmt.Printf("[%0X]\n", 0.045)
	fmt.Printf("[%#X]\n", 0.045)
	fmt.Printf("[%30X]\n", 0.045)
	fmt.Printf("[%-30X]\n", 0.045)
	fmt.Printf("[%030X]\n", 0.045)
	fmt.Printf("[%+030X]\n", 0.045)
	fmt.Printf("[%.0X]\n", 0.045)
	fmt.Printf("[%.1X]\n", 0.045)
	fmt.Printf("[%.3X]\n", 0.045)
	fmt.Printf("[%.10X]\n", 0.045)
	fmt.Printf("[%#.3X]\n", 0.045)
	fmt.Printf("[%#30X]\n", 0.045)
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
	fmt.Printf("[%X]\n", 999999.5)
	fmt.Printf("[%+X]\n", 999999.5)
	fmt.Printf("[%-X]\n", 999999.5)
	fmt.Printf("[% X]\n", 999999.5)
	fmt.Printf("[%0X]\n", 999999.5)
	fmt.Printf("[%#X]\n", 999999.5)
	fmt.Printf("[%30X]\n", 999999.5)
	fmt.Printf("[%-30X]\n", 999999.5)
	fmt.Printf("[%030X]\n", 999999.5)
	fmt.Printf("[%+030X]\n", 999999.5)
	fmt.Printf("[%.0X]\n", 999999.5)
	fmt.Printf("[%.1X]\n", 999999.5)
	fmt.Printf("[%.3X]\n", 999999.5)
	fmt.Printf("[%.10X]\n", 999999.5)
	fmt.Printf("[%#.3X]\n", 999999.5)
	fmt.Printf("[%#30X]\n", 999999.5)
	fmt.Printf("[%X]\n", 0.9999995)
	fmt.Printf("[%+X]\n", 0.9999995)
	fmt.Printf("[%-X]\n", 0.9999995)
	fmt.Printf("[% X]\n", 0.9999995)
	fmt.Printf("[%0X]\n", 0.9999995)
	fmt.Printf("[%#X]\n", 0.9999995)
	fmt.Printf("[%30X]\n", 0.9999995)
	fmt.Printf("[%-30X]\n", 0.9999995)
	fmt.Printf("[%030X]\n", 0.9999995)
	fmt.Printf("[%+030X]\n", 0.9999995)
	fmt.Printf("[%.0X]\n", 0.9999995)
	fmt.Printf("[%.1X]\n", 0.9999995)
	fmt.Printf("[%.3X]\n", 0.9999995)
	fmt.Printf("[%.10X]\n", 0.9999995)
	fmt.Printf("[%#.3X]\n", 0.9999995)
	fmt.Printf("[%#30X]\n", 0.9999995)
}

// Output:
// [0X1.5666666666666P+01]
// [+0X1.5666666666666P+01]
// [0X1.5666666666666P+01]
// [ 0X1.5666666666666P+01]
// [0X1.5666666666666P+01]
// [0X1.5666666666666P+01]
// [         0X1.5666666666666P+01]
// [0X1.5666666666666P+01         ]
// [0000000000X1.5666666666666P+01]
// [+000000000X1.5666666666666P+01]
// [0X1P+01]
// [0X1.5P+01]
// [0X1.566P+01]
// [0X1.5666666666P+01]
// [0X1.566P+01]
// [         0X1.5666666666666P+01]
// [0X1.0147AE147AE14P+00]
// [+0X1.0147AE147AE14P+00]
// [0X1.0147AE147AE14P+00]
// [ 0X1.0147AE147AE14P+00]
// [0X1.0147AE147AE14P+00]
// [0X1.0147AE147AE14P+00]
// [         0X1.0147AE147AE14P+00]
// [0X1.0147AE147AE14P+00         ]
// [0000000000X1.0147AE147AE14P+00]
// [+000000000X1.0147AE147AE14P+00]
// [0X1P+00]
// [0X1.0P+00]
// [0X1.014P+00]
// [0X1.0147AE147BP+00]
// [0X1.014P+00]
// [         0X1.0147AE147AE14P+00]
// [0X1P-03]
// [+0X1P-03]
// [0X1P-03]
// [ 0X1P-03]
// [0X1P-03]
// [0X1.P-03]
// [                       0X1P-03]
// [0X1P-03                       ]
// [000000000000000000000000X1P-03]
// [+00000000000000000000000X1P-03]
// [0X1P-03]
// [0X1.0P-03]
// [0X1.000P-03]
// [0X1.0000000000P-03]
// [0X1.000P-03]
// [                      0X1.P-03]
// [0X1.8P-02]
// [+0X1.8P-02]
// [0X1.8P-02]
// [ 0X1.8P-02]
// [0X1.8P-02]
// [0X1.8P-02]
// [                     0X1.8P-02]
// [0X1.8P-02                     ]
// [0000000000000000000000X1.8P-02]
// [+000000000000000000000X1.8P-02]
// [0X1P-01]
// [0X1.8P-02]
// [0X1.800P-02]
// [0X1.8000000000P-02]
// [0X1.800P-02]
// [                     0X1.8P-02]
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
// [0X1.CP+01]
// [+0X1.CP+01]
// [0X1.CP+01]
// [ 0X1.CP+01]
// [0X1.CP+01]
// [0X1.CP+01]
// [                     0X1.CP+01]
// [0X1.CP+01                     ]
// [0000000000000000000000X1.CP+01]
// [+000000000000000000000X1.CP+01]
// [0X1P+02]
// [0X1.CP+01]
// [0X1.C00P+01]
// [0X1.C000000000P+01]
// [0X1.C00P+01]
// [                     0X1.CP+01]
// [-0X1.4P+01]
// [-0X1.4P+01]
// [-0X1.4P+01]
// [-0X1.4P+01]
// [-0X1.4P+01]
// [-0X1.4P+01]
// [                    -0X1.4P+01]
// [-0X1.4P+01                    ]
// [-000000000000000000000X1.4P+01]
// [-000000000000000000000X1.4P+01]
// [-0X1P+01]
// [-0X1.4P+01]
// [-0X1.400P+01]
// [-0X1.4000000000P+01]
// [-0X1.400P+01]
// [                    -0X1.4P+01]
// [0X1.70A3D70A3D70AP-05]
// [+0X1.70A3D70A3D70AP-05]
// [0X1.70A3D70A3D70AP-05]
// [ 0X1.70A3D70A3D70AP-05]
// [0X1.70A3D70A3D70AP-05]
// [0X1.70A3D70A3D70AP-05]
// [         0X1.70A3D70A3D70AP-05]
// [0X1.70A3D70A3D70AP-05         ]
// [0000000000X1.70A3D70A3D70AP-05]
// [+000000000X1.70A3D70A3D70AP-05]
// [0X1P-05]
// [0X1.7P-05]
// [0X1.70AP-05]
// [0X1.70A3D70A3DP-05]
// [0X1.70AP-05]
// [         0X1.70A3D70A3D70AP-05]
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
// [0X1.E847FP+19]
// [+0X1.E847FP+19]
// [0X1.E847FP+19]
// [ 0X1.E847FP+19]
// [0X1.E847FP+19]
// [0X1.E847FP+19]
// [                 0X1.E847FP+19]
// [0X1.E847FP+19                 ]
// [000000000000000000X1.E847FP+19]
// [+00000000000000000X1.E847FP+19]
// [0X1P+20]
// [0X1.FP+19]
// [0X1.E84P+19]
// [0X1.E847F00000P+19]
// [0X1.E84P+19]
// [                 0X1.E847FP+19]
// [0X1.FFFFEF39085F5P-01]
// [+0X1.FFFFEF39085F5P-01]
// [0X1.FFFFEF39085F5P-01]
// [ 0X1.FFFFEF39085F5P-01]
// [0X1.FFFFEF39085F5P-01]
// [0X1.FFFFEF39085F5P-01]
// [         0X1.FFFFEF39085F5P-01]
// [0X1.FFFFEF39085F5P-01         ]
// [0000000000X1.FFFFEF39085F5P-01]
// [+000000000X1.FFFFEF39085F5P-01]
// [0X1P+00]
// [0X1.0P+00]
// [0X1.000P+00]
// [0X1.FFFFEF3908P-01]
// [0X1.000P+00]
// [         0X1.FFFFEF39085F5P-01]
