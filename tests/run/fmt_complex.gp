// phase: 03

// fmt.Println and Printf of complex numbers (step 3.15c): (re+imi), with each
// part formatted like a float of its half size (a complex64 has float32
// parts), the imaginary part always with a sign; %f and a precision apply to
// both parts.
package main

import "fmt"

type Impedance complex128

func (z Impedance) String() string {
	return fmt.Sprintf("%.1f ohm", real(z))
}

func main() {
	v := complex(3, -4)
	var w complex64 = 0.1 + 1i
	fmt.Println(v, w, -v, v*v, v/2)
	fmt.Printf("%v %.2f %f %6.1f|\n", v, w, v, v)
	fmt.Printf("%d\n", v)
	fmt.Println([]complex128{1, 1i}, map[complex64]int{2i: 1, 1: 2, -1: 3})
	z := Impedance(50 + 10i)
	fmt.Println(z, []Impedance{z, 1})
	fmt.Printf("%v %d\n", z, z)
	fmt.Println(fmt.Sprint(complex64(1.0/3+0i)), fmt.Sprintln(0i))
}

// Output:
// (3-4i) (0.1+1i) (-3+4i) (-7-24i) (1.5-2i)
// (3-4i) (0.10+1.00i) (3.000000-4.000000i) (   3.0  -4.0i)|
// %!d(complex128=(3-4i))
// [(1+0i) (0+1i)] map[(-1+0i):3 (0+2i):1 (1+0i):2]
// 50.0 ohm [50.0 ohm 1.0 ohm]
// 50.0 ohm %!d(main.Impedance=(50+10i))
// (0.33333334+0i) (0+0i)
//
