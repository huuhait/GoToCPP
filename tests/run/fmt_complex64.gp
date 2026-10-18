// phase: 05

// complex64 prints its parts as float32; named complex types; the error forms.
package main

import "fmt"

type Z complex128

func main() {
	fmt.Println(complex64(complex(0.1, 0.2)), complex(float32(1), 2), Z(complex(3, -4)))
	fmt.Printf("%v %.2f %g %e %8.1f|%-12v|\n", complex64(complex(0.1, 0.2)), complex64(complex(1.005, 2.675)), complex64(complex(1e10, 1e-10)), complex64(1+2i), complex64(3+4i), complex64(5i))
	fmt.Printf("%d %s %t %q\n", complex(1, 2), complex(1, 2), complex(1, 2), complex(1, 2))
	fmt.Printf("%T %T %T %#v\n", complex64(1), complex128(1), Z(1), Z(1+2i))
}

// Output:
// (0.1+0.2i) (1+2i) (3-4i)
// (0.1+0.2i) (1.00+2.67i) (1e+10+1e-10i) (1.000000e+00+2.000000e+00i) (     3.0    +4.0i)|(0           +5          i)|
// %!d(complex128=(1+2i)) %!s(complex128=(1+2i)) %!t(complex128=(1+2i)) %!q(complex128=(1+2i))
// complex64 complex128 main.Z (1+2i)
