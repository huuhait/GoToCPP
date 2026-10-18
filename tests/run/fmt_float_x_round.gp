// phase: 05

// Go's %x of float64 values (round) with flags, width and precision, as strconv.AppendFloat and
// fmt.fmtFloat do.
package main

import "fmt"

func main() {
	fmt.Printf("[%x]\n", 2.675)
	fmt.Printf("[%+x]\n", 2.675)
	fmt.Printf("[%-x]\n", 2.675)
	fmt.Printf("[% x]\n", 2.675)
	fmt.Printf("[%0x]\n", 2.675)
	fmt.Printf("[%#x]\n", 2.675)
	fmt.Printf("[%30x]\n", 2.675)
	fmt.Printf("[%-30x]\n", 2.675)
	fmt.Printf("[%030x]\n", 2.675)
	fmt.Printf("[%+030x]\n", 2.675)
	fmt.Printf("[%.0x]\n", 2.675)
	fmt.Printf("[%.1x]\n", 2.675)
	fmt.Printf("[%.3x]\n", 2.675)
	fmt.Printf("[%.10x]\n", 2.675)
	fmt.Printf("[%#.3x]\n", 2.675)
	fmt.Printf("[%#30x]\n", 2.675)
	fmt.Printf("[%x]\n", 1.005)
	fmt.Printf("[%+x]\n", 1.005)
	fmt.Printf("[%-x]\n", 1.005)
	fmt.Printf("[% x]\n", 1.005)
	fmt.Printf("[%0x]\n", 1.005)
	fmt.Printf("[%#x]\n", 1.005)
	fmt.Printf("[%30x]\n", 1.005)
	fmt.Printf("[%-30x]\n", 1.005)
	fmt.Printf("[%030x]\n", 1.005)
	fmt.Printf("[%+030x]\n", 1.005)
	fmt.Printf("[%.0x]\n", 1.005)
	fmt.Printf("[%.1x]\n", 1.005)
	fmt.Printf("[%.3x]\n", 1.005)
	fmt.Printf("[%.10x]\n", 1.005)
	fmt.Printf("[%#.3x]\n", 1.005)
	fmt.Printf("[%#30x]\n", 1.005)
	fmt.Printf("[%x]\n", 0.125)
	fmt.Printf("[%+x]\n", 0.125)
	fmt.Printf("[%-x]\n", 0.125)
	fmt.Printf("[% x]\n", 0.125)
	fmt.Printf("[%0x]\n", 0.125)
	fmt.Printf("[%#x]\n", 0.125)
	fmt.Printf("[%30x]\n", 0.125)
	fmt.Printf("[%-30x]\n", 0.125)
	fmt.Printf("[%030x]\n", 0.125)
	fmt.Printf("[%+030x]\n", 0.125)
	fmt.Printf("[%.0x]\n", 0.125)
	fmt.Printf("[%.1x]\n", 0.125)
	fmt.Printf("[%.3x]\n", 0.125)
	fmt.Printf("[%.10x]\n", 0.125)
	fmt.Printf("[%#.3x]\n", 0.125)
	fmt.Printf("[%#30x]\n", 0.125)
	fmt.Printf("[%x]\n", 0.375)
	fmt.Printf("[%+x]\n", 0.375)
	fmt.Printf("[%-x]\n", 0.375)
	fmt.Printf("[% x]\n", 0.375)
	fmt.Printf("[%0x]\n", 0.375)
	fmt.Printf("[%#x]\n", 0.375)
	fmt.Printf("[%30x]\n", 0.375)
	fmt.Printf("[%-30x]\n", 0.375)
	fmt.Printf("[%030x]\n", 0.375)
	fmt.Printf("[%+030x]\n", 0.375)
	fmt.Printf("[%.0x]\n", 0.375)
	fmt.Printf("[%.1x]\n", 0.375)
	fmt.Printf("[%.3x]\n", 0.375)
	fmt.Printf("[%.10x]\n", 0.375)
	fmt.Printf("[%#.3x]\n", 0.375)
	fmt.Printf("[%#30x]\n", 0.375)
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
	fmt.Printf("[%x]\n", 3.5)
	fmt.Printf("[%+x]\n", 3.5)
	fmt.Printf("[%-x]\n", 3.5)
	fmt.Printf("[% x]\n", 3.5)
	fmt.Printf("[%0x]\n", 3.5)
	fmt.Printf("[%#x]\n", 3.5)
	fmt.Printf("[%30x]\n", 3.5)
	fmt.Printf("[%-30x]\n", 3.5)
	fmt.Printf("[%030x]\n", 3.5)
	fmt.Printf("[%+030x]\n", 3.5)
	fmt.Printf("[%.0x]\n", 3.5)
	fmt.Printf("[%.1x]\n", 3.5)
	fmt.Printf("[%.3x]\n", 3.5)
	fmt.Printf("[%.10x]\n", 3.5)
	fmt.Printf("[%#.3x]\n", 3.5)
	fmt.Printf("[%#30x]\n", 3.5)
	fmt.Printf("[%x]\n", -2.5)
	fmt.Printf("[%+x]\n", -2.5)
	fmt.Printf("[%-x]\n", -2.5)
	fmt.Printf("[% x]\n", -2.5)
	fmt.Printf("[%0x]\n", -2.5)
	fmt.Printf("[%#x]\n", -2.5)
	fmt.Printf("[%30x]\n", -2.5)
	fmt.Printf("[%-30x]\n", -2.5)
	fmt.Printf("[%030x]\n", -2.5)
	fmt.Printf("[%+030x]\n", -2.5)
	fmt.Printf("[%.0x]\n", -2.5)
	fmt.Printf("[%.1x]\n", -2.5)
	fmt.Printf("[%.3x]\n", -2.5)
	fmt.Printf("[%.10x]\n", -2.5)
	fmt.Printf("[%#.3x]\n", -2.5)
	fmt.Printf("[%#30x]\n", -2.5)
	fmt.Printf("[%x]\n", 0.045)
	fmt.Printf("[%+x]\n", 0.045)
	fmt.Printf("[%-x]\n", 0.045)
	fmt.Printf("[% x]\n", 0.045)
	fmt.Printf("[%0x]\n", 0.045)
	fmt.Printf("[%#x]\n", 0.045)
	fmt.Printf("[%30x]\n", 0.045)
	fmt.Printf("[%-30x]\n", 0.045)
	fmt.Printf("[%030x]\n", 0.045)
	fmt.Printf("[%+030x]\n", 0.045)
	fmt.Printf("[%.0x]\n", 0.045)
	fmt.Printf("[%.1x]\n", 0.045)
	fmt.Printf("[%.3x]\n", 0.045)
	fmt.Printf("[%.10x]\n", 0.045)
	fmt.Printf("[%#.3x]\n", 0.045)
	fmt.Printf("[%#30x]\n", 0.045)
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
	fmt.Printf("[%x]\n", 999999.5)
	fmt.Printf("[%+x]\n", 999999.5)
	fmt.Printf("[%-x]\n", 999999.5)
	fmt.Printf("[% x]\n", 999999.5)
	fmt.Printf("[%0x]\n", 999999.5)
	fmt.Printf("[%#x]\n", 999999.5)
	fmt.Printf("[%30x]\n", 999999.5)
	fmt.Printf("[%-30x]\n", 999999.5)
	fmt.Printf("[%030x]\n", 999999.5)
	fmt.Printf("[%+030x]\n", 999999.5)
	fmt.Printf("[%.0x]\n", 999999.5)
	fmt.Printf("[%.1x]\n", 999999.5)
	fmt.Printf("[%.3x]\n", 999999.5)
	fmt.Printf("[%.10x]\n", 999999.5)
	fmt.Printf("[%#.3x]\n", 999999.5)
	fmt.Printf("[%#30x]\n", 999999.5)
	fmt.Printf("[%x]\n", 0.9999995)
	fmt.Printf("[%+x]\n", 0.9999995)
	fmt.Printf("[%-x]\n", 0.9999995)
	fmt.Printf("[% x]\n", 0.9999995)
	fmt.Printf("[%0x]\n", 0.9999995)
	fmt.Printf("[%#x]\n", 0.9999995)
	fmt.Printf("[%30x]\n", 0.9999995)
	fmt.Printf("[%-30x]\n", 0.9999995)
	fmt.Printf("[%030x]\n", 0.9999995)
	fmt.Printf("[%+030x]\n", 0.9999995)
	fmt.Printf("[%.0x]\n", 0.9999995)
	fmt.Printf("[%.1x]\n", 0.9999995)
	fmt.Printf("[%.3x]\n", 0.9999995)
	fmt.Printf("[%.10x]\n", 0.9999995)
	fmt.Printf("[%#.3x]\n", 0.9999995)
	fmt.Printf("[%#30x]\n", 0.9999995)
}

// Output:
// [0x1.5666666666666p+01]
// [+0x1.5666666666666p+01]
// [0x1.5666666666666p+01]
// [ 0x1.5666666666666p+01]
// [0x1.5666666666666p+01]
// [0x1.5666666666666p+01]
// [         0x1.5666666666666p+01]
// [0x1.5666666666666p+01         ]
// [0000000000x1.5666666666666p+01]
// [+000000000x1.5666666666666p+01]
// [0x1p+01]
// [0x1.5p+01]
// [0x1.566p+01]
// [0x1.5666666666p+01]
// [0x1.566p+01]
// [         0x1.5666666666666p+01]
// [0x1.0147ae147ae14p+00]
// [+0x1.0147ae147ae14p+00]
// [0x1.0147ae147ae14p+00]
// [ 0x1.0147ae147ae14p+00]
// [0x1.0147ae147ae14p+00]
// [0x1.0147ae147ae14p+00]
// [         0x1.0147ae147ae14p+00]
// [0x1.0147ae147ae14p+00         ]
// [0000000000x1.0147ae147ae14p+00]
// [+000000000x1.0147ae147ae14p+00]
// [0x1p+00]
// [0x1.0p+00]
// [0x1.014p+00]
// [0x1.0147ae147bp+00]
// [0x1.014p+00]
// [         0x1.0147ae147ae14p+00]
// [0x1p-03]
// [+0x1p-03]
// [0x1p-03]
// [ 0x1p-03]
// [0x1p-03]
// [0x1.0000p-03]
// [                       0x1p-03]
// [0x1p-03                       ]
// [000000000000000000000000x1p-03]
// [+00000000000000000000000x1p-03]
// [0x1p-03]
// [0x1.0p-03]
// [0x1.000p-03]
// [0x1.0000000000p-03]
// [0x1.000p-03]
// [                  0x1.0000p-03]
// [0x1.8p-02]
// [+0x1.8p-02]
// [0x1.8p-02]
// [ 0x1.8p-02]
// [0x1.8p-02]
// [0x1.8000p-02]
// [                     0x1.8p-02]
// [0x1.8p-02                     ]
// [0000000000000000000000x1.8p-02]
// [+000000000000000000000x1.8p-02]
// [0x1p-01]
// [0x1.8p-02]
// [0x1.800p-02]
// [0x1.8000000000p-02]
// [0x1.800p-02]
// [                  0x1.8000p-02]
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
// [0x1.cp+01]
// [+0x1.cp+01]
// [0x1.cp+01]
// [ 0x1.cp+01]
// [0x1.cp+01]
// [0x1.c000p+01]
// [                     0x1.cp+01]
// [0x1.cp+01                     ]
// [0000000000000000000000x1.cp+01]
// [+000000000000000000000x1.cp+01]
// [0x1p+02]
// [0x1.cp+01]
// [0x1.c00p+01]
// [0x1.c000000000p+01]
// [0x1.c00p+01]
// [                  0x1.c000p+01]
// [-0x1.4p+01]
// [-0x1.4p+01]
// [-0x1.4p+01]
// [-0x1.4p+01]
// [-0x1.4p+01]
// [-0x1.4000p+01]
// [                    -0x1.4p+01]
// [-0x1.4p+01                    ]
// [-000000000000000000000x1.4p+01]
// [-000000000000000000000x1.4p+01]
// [-0x1p+01]
// [-0x1.4p+01]
// [-0x1.400p+01]
// [-0x1.4000000000p+01]
// [-0x1.400p+01]
// [                 -0x1.4000p+01]
// [0x1.70a3d70a3d70ap-05]
// [+0x1.70a3d70a3d70ap-05]
// [0x1.70a3d70a3d70ap-05]
// [ 0x1.70a3d70a3d70ap-05]
// [0x1.70a3d70a3d70ap-05]
// [0x1.70a3d70a3d70ap-05]
// [         0x1.70a3d70a3d70ap-05]
// [0x1.70a3d70a3d70ap-05         ]
// [0000000000x1.70a3d70a3d70ap-05]
// [+000000000x1.70a3d70a3d70ap-05]
// [0x1p-05]
// [0x1.7p-05]
// [0x1.70ap-05]
// [0x1.70a3d70a3dp-05]
// [0x1.70ap-05]
// [         0x1.70a3d70a3d70ap-05]
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
// [0x1.e847fp+19]
// [+0x1.e847fp+19]
// [0x1.e847fp+19]
// [ 0x1.e847fp+19]
// [0x1.e847fp+19]
// [0x1.e847fp+19]
// [                 0x1.e847fp+19]
// [0x1.e847fp+19                 ]
// [000000000000000000x1.e847fp+19]
// [+00000000000000000x1.e847fp+19]
// [0x1p+20]
// [0x1.fp+19]
// [0x1.e84p+19]
// [0x1.e847f00000p+19]
// [0x1.e84p+19]
// [                 0x1.e847fp+19]
// [0x1.ffffef39085f5p-01]
// [+0x1.ffffef39085f5p-01]
// [0x1.ffffef39085f5p-01]
// [ 0x1.ffffef39085f5p-01]
// [0x1.ffffef39085f5p-01]
// [0x1.ffffef39085f5p-01]
// [         0x1.ffffef39085f5p-01]
// [0x1.ffffef39085f5p-01         ]
// [0000000000x1.ffffef39085f5p-01]
// [+000000000x1.ffffef39085f5p-01]
// [0x1p+00]
// [0x1.0p+00]
// [0x1.000p+00]
// [0x1.ffffef3908p-01]
// [0x1.000p+00]
// [         0x1.ffffef39085f5p-01]
