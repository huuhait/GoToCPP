// phase: 05

// Go's %x of complex128: both parts with the verb, the imaginary part always signed.
package main

import "fmt"

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	fmt.Printf("[%x]\n", complex(1, 2))
	fmt.Printf("[%+x]\n", complex(1, 2))
	fmt.Printf("[%-x]\n", complex(1, 2))
	fmt.Printf("[% x]\n", complex(1, 2))
	fmt.Printf("[%10x]\n", complex(1, 2))
	fmt.Printf("[%-10x]\n", complex(1, 2))
	fmt.Printf("[%010x]\n", complex(1, 2))
	fmt.Printf("[%.2x]\n", complex(1, 2))
	fmt.Printf("[%10.3x]\n", complex(1, 2))
	fmt.Printf("[%#x]\n", complex(1, 2))
	fmt.Printf("[%+.1x]\n", complex(1, 2))
	fmt.Printf("[%x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-x]\n", complex(-1.5, 0.25))
	fmt.Printf("[% x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-10x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%010x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%.2x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10.3x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%#x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+.1x]\n", complex(-1.5, 0.25))
	fmt.Printf("[%x]\n", complex(0, 0))
	fmt.Printf("[%+x]\n", complex(0, 0))
	fmt.Printf("[%-x]\n", complex(0, 0))
	fmt.Printf("[% x]\n", complex(0, 0))
	fmt.Printf("[%10x]\n", complex(0, 0))
	fmt.Printf("[%-10x]\n", complex(0, 0))
	fmt.Printf("[%010x]\n", complex(0, 0))
	fmt.Printf("[%.2x]\n", complex(0, 0))
	fmt.Printf("[%10.3x]\n", complex(0, 0))
	fmt.Printf("[%#x]\n", complex(0, 0))
	fmt.Printf("[%+.1x]\n", complex(0, 0))
	fmt.Printf("[%x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-x]\n", complex(1e21, 1e-7))
	fmt.Printf("[% x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-10x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%010x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%.2x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10.3x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%#x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+.1x]\n", complex(1e21, 1e-7))
	fmt.Printf("[%x]\n", complex(inf, nan))
	fmt.Printf("[%+x]\n", complex(inf, nan))
	fmt.Printf("[%-x]\n", complex(inf, nan))
	fmt.Printf("[% x]\n", complex(inf, nan))
	fmt.Printf("[%10x]\n", complex(inf, nan))
	fmt.Printf("[%-10x]\n", complex(inf, nan))
	fmt.Printf("[%010x]\n", complex(inf, nan))
	fmt.Printf("[%.2x]\n", complex(inf, nan))
	fmt.Printf("[%10.3x]\n", complex(inf, nan))
	fmt.Printf("[%#x]\n", complex(inf, nan))
	fmt.Printf("[%+.1x]\n", complex(inf, nan))
	fmt.Printf("[%x]\n", complex(2.675, -2.675))
	fmt.Printf("[%+x]\n", complex(2.675, -2.675))
	fmt.Printf("[%-x]\n", complex(2.675, -2.675))
	fmt.Printf("[% x]\n", complex(2.675, -2.675))
	fmt.Printf("[%10x]\n", complex(2.675, -2.675))
	fmt.Printf("[%-10x]\n", complex(2.675, -2.675))
	fmt.Printf("[%010x]\n", complex(2.675, -2.675))
	fmt.Printf("[%.2x]\n", complex(2.675, -2.675))
	fmt.Printf("[%10.3x]\n", complex(2.675, -2.675))
	fmt.Printf("[%#x]\n", complex(2.675, -2.675))
	fmt.Printf("[%+.1x]\n", complex(2.675, -2.675))
}

// Output:
// [(0x1p+00+0x1p+01i)]
// [(+0x1p+00+0x1p+01i)]
// [(0x1p+00+0x1p+01i)]
// [( 0x1p+00+0x1p+01i)]
// [(   0x1p+00  +0x1p+01i)]
// [(0x1p+00   +0x1p+01  i)]
// [(0000x1p+00+000x1p+01i)]
// [(0x1.00p+00+0x1.00p+01i)]
// [(0x1.000p+00+0x1.000p+01i)]
// [(0x1.0000p+00+0x1.0000p+01i)]
// [(+0x1.0p+00+0x1.0p+01i)]
// [(-0x1.8p+00+0x1p-02i)]
// [(-0x1.8p+00+0x1p-02i)]
// [(-0x1.8p+00+0x1p-02i)]
// [(-0x1.8p+00+0x1p-02i)]
// [(-0x1.8p+00  +0x1p-02i)]
// [(-0x1.8p+00+0x1p-02  i)]
// [(-0x1.8p+00+000x1p-02i)]
// [(-0x1.80p+00+0x1.00p-02i)]
// [(-0x1.800p+00+0x1.000p-02i)]
// [(-0x1.8000p+00+0x1.0000p-02i)]
// [(-0x1.8p+00+0x1.0p-02i)]
// [(0x0p+00+0x0p+00i)]
// [(+0x0p+00+0x0p+00i)]
// [(0x0p+00+0x0p+00i)]
// [( 0x0p+00+0x0p+00i)]
// [(   0x0p+00  +0x0p+00i)]
// [(0x0p+00   +0x0p+00  i)]
// [(0000x0p+00+000x0p+00i)]
// [(0x0.00p+00+0x0.00p+00i)]
// [(0x0.000p+00+0x0.000p+00i)]
// [(0x0.0000p+00+0x0.0000p+00i)]
// [(+0x0.0p+00+0x0.0p+00i)]
// [(0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [(+0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [(0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [( 0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [(0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [(0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [(0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [(0x1.b2p+69+0x1.adp-24i)]
// [(0x1.b1bp+69+0x1.ad8p-24i)]
// [(0x1.b1ae4d6e2ef5p+69+0x1.ad7f29abcaf48p-24i)]
// [(+0x1.bp+69+0x1.bp-24i)]
// [(+Inf+NaNi)]
// [(+Inf+NaNi)]
// [(+Inf+NaNi)]
// [( Inf+NaNi)]
// [(      +Inf      +NaNi)]
// [(+Inf      +NaN      i)]
// [(      +Inf      +NaNi)]
// [(+Inf+NaNi)]
// [(      +Inf      +NaNi)]
// [(+Inf+NaNi)]
// [(+Inf+NaNi)]
// [(0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [(+0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [(0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [( 0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [(0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [(0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [(0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [(0x1.56p+01-0x1.56p+01i)]
// [(0x1.566p+01-0x1.566p+01i)]
// [(0x1.5666666666666p+01-0x1.5666666666666p+01i)]
// [(+0x1.5p+01-0x1.5p+01i)]
