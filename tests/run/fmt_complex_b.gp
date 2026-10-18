// phase: 05

// Go's %b of complex128: both parts with the verb, the imaginary part always signed.
package main

import "fmt"

func main() {
	zero := 0.0
	inf := 1 / zero
	ninf := -1 / zero
	nan := zero / zero
	nzero := -zero
	_, _, _, _ = inf, ninf, nan, nzero
	fmt.Printf("[%b]\n", complex(1, 2))
	fmt.Printf("[%+b]\n", complex(1, 2))
	fmt.Printf("[%-b]\n", complex(1, 2))
	fmt.Printf("[% b]\n", complex(1, 2))
	fmt.Printf("[%10b]\n", complex(1, 2))
	fmt.Printf("[%-10b]\n", complex(1, 2))
	fmt.Printf("[%010b]\n", complex(1, 2))
	fmt.Printf("[%.2b]\n", complex(1, 2))
	fmt.Printf("[%10.3b]\n", complex(1, 2))
	fmt.Printf("[%#b]\n", complex(1, 2))
	fmt.Printf("[%+.1b]\n", complex(1, 2))
	fmt.Printf("[%b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-b]\n", complex(-1.5, 0.25))
	fmt.Printf("[% b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%-10b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%010b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%.2b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%10.3b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%#b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%+.1b]\n", complex(-1.5, 0.25))
	fmt.Printf("[%b]\n", complex(0, 0))
	fmt.Printf("[%+b]\n", complex(0, 0))
	fmt.Printf("[%-b]\n", complex(0, 0))
	fmt.Printf("[% b]\n", complex(0, 0))
	fmt.Printf("[%10b]\n", complex(0, 0))
	fmt.Printf("[%-10b]\n", complex(0, 0))
	fmt.Printf("[%010b]\n", complex(0, 0))
	fmt.Printf("[%.2b]\n", complex(0, 0))
	fmt.Printf("[%10.3b]\n", complex(0, 0))
	fmt.Printf("[%#b]\n", complex(0, 0))
	fmt.Printf("[%+.1b]\n", complex(0, 0))
	fmt.Printf("[%b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-b]\n", complex(1e21, 1e-7))
	fmt.Printf("[% b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%-10b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%010b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%.2b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%10.3b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%#b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%+.1b]\n", complex(1e21, 1e-7))
	fmt.Printf("[%b]\n", complex(inf, nan))
	fmt.Printf("[%+b]\n", complex(inf, nan))
	fmt.Printf("[%-b]\n", complex(inf, nan))
	fmt.Printf("[% b]\n", complex(inf, nan))
	fmt.Printf("[%10b]\n", complex(inf, nan))
	fmt.Printf("[%-10b]\n", complex(inf, nan))
	fmt.Printf("[%010b]\n", complex(inf, nan))
	fmt.Printf("[%.2b]\n", complex(inf, nan))
	fmt.Printf("[%10.3b]\n", complex(inf, nan))
	fmt.Printf("[%#b]\n", complex(inf, nan))
	fmt.Printf("[%+.1b]\n", complex(inf, nan))
	fmt.Printf("[%b]\n", complex(2.675, -2.675))
	fmt.Printf("[%+b]\n", complex(2.675, -2.675))
	fmt.Printf("[%-b]\n", complex(2.675, -2.675))
	fmt.Printf("[% b]\n", complex(2.675, -2.675))
	fmt.Printf("[%10b]\n", complex(2.675, -2.675))
	fmt.Printf("[%-10b]\n", complex(2.675, -2.675))
	fmt.Printf("[%010b]\n", complex(2.675, -2.675))
	fmt.Printf("[%.2b]\n", complex(2.675, -2.675))
	fmt.Printf("[%10.3b]\n", complex(2.675, -2.675))
	fmt.Printf("[%#b]\n", complex(2.675, -2.675))
	fmt.Printf("[%+.1b]\n", complex(2.675, -2.675))
}

// Output:
// [(4503599627370496p-52+4503599627370496p-51i)]
// [(+4503599627370496p-52+4503599627370496p-51i)]
// [(4503599627370496p-52+4503599627370496p-51i)]
// [( 4503599627370496p-52+4503599627370496p-51i)]
// [(4503599627370496p-52+4503599627370496p-51i)]
// [(4503599627370496p-52+4503599627370496p-51i)]
// [(4503599627370496p-52+4503599627370496p-51i)]
// [(4503599627370496p-52+4503599627370496p-51i)]
// [(4503599627370496p-52+4503599627370496p-51i)]
// [(4503599627370496p-52+4503599627370496p-51i)]
// [(+4503599627370496p-52+4503599627370496p-51i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(-6755399441055744p-52+4503599627370496p-54i)]
// [(0p-1074+0p-1074i)]
// [(+0p-1074+0p-1074i)]
// [(0p-1074+0p-1074i)]
// [( 0p-1074+0p-1074i)]
// [(   0p-1074  +0p-1074i)]
// [(0p-1074   +0p-1074  i)]
// [(0000p-1074+000p-1074i)]
// [(0p-1074+0p-1074i)]
// [(   0p-1074  +0p-1074i)]
// [(0p-1074+0p-1074i)]
// [(+0p-1074+0p-1074i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [(+7629394531250000p+17+7555786372591432p-76i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [( 7629394531250000p+17+7555786372591432p-76i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [(7629394531250000p+17+7555786372591432p-76i)]
// [(+7629394531250000p+17+7555786372591432p-76i)]
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
// [(6023564501608038p-51-6023564501608038p-51i)]
// [(+6023564501608038p-51-6023564501608038p-51i)]
// [(6023564501608038p-51-6023564501608038p-51i)]
// [( 6023564501608038p-51-6023564501608038p-51i)]
// [(6023564501608038p-51-6023564501608038p-51i)]
// [(6023564501608038p-51-6023564501608038p-51i)]
// [(6023564501608038p-51-6023564501608038p-51i)]
// [(6023564501608038p-51-6023564501608038p-51i)]
// [(6023564501608038p-51-6023564501608038p-51i)]
// [(6023564501608038p-51-6023564501608038p-51i)]
// [(+6023564501608038p-51-6023564501608038p-51i)]
