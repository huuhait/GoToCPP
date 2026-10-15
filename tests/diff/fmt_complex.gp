// phase: 03

// fmt prints complex numbers as Go's fmtComplex does (step 3.15c): each part
// as a float of half the size, the imaginary part always signed, a width and a
// precision for each part; a verb that doesn't fit gives the error form with the
// whole number (%!d(complex128=(1.5-2i))); complex map keys sort by the real
// part, then the imaginary part, NaN first; named complex types with and without
// a String method; complex numbers in slices, arrays, structs and map values. And
// the order: an argument that is not a variable is copied in its place among the
// calls (gc copies a complex number into an interface), a variable is read after
// them.
package main

import "fmt"

type Phasor complex128

type Named complex64

func (n Named) String() string { return fmt.Sprintf("N<%v>", complex64(n)) }

type Box struct {
	C  complex64
	p  Phasor
	Ns []Named
}

func main() {
	zero := 0.0
	inf, nan := 1/zero, zero/zero
	a := complex(1.5, -2)
	var b complex64 = 0.1 + 0.2i
	fmt.Println(a, b, complex(-zero, -zero), complex(inf, nan), complex(nan, -inf), 1i, 0i)
	fmt.Printf("%v|%f|%.2f|%8.2f|%-8.1f|%+v|%+.1f|% v|%08.3f\n", a, a, b, a, b, a, a, b, a)
	fmt.Printf("%d|%s|%q|%t|%5d\n", a, b, a, b, a)
	fmt.Printf("%v %f\n", complex(1e21, 1e-7), complex64(complex(1e21, 3.4e38)))
	fmt.Print(a, b, "x", a, "\n")
	s := fmt.Sprint(Phasor(a), Named(b), []complex64{1, 2i}, [2]complex128{-1, 1i})
	fmt.Println(s)
	fmt.Printf("%v %d %s %.1f\n", Phasor(1+1i), Phasor(2), Named(3i), Named(1+1i))
	fmt.Println(Box{1, 2i, []Named{1i}})
	fmt.Printf("%+v %v\n", Box{1i, -1, nil}, &Box{})
	fmt.Printf("%d %v\n", []complex128{1, 2}, map[string]complex64{"a": 1})
	m := map[complex128]string{}
	for _, c := range []complex128{2, 1i, -1, complex(nan, 0), complex(1, nan), complex(1, -1), complex(-inf, 5), complex(-zero, 0), 1 + 1i, complex(nan, 1)} {
		m[c] = fmt.Sprint(c)
	}
	fmt.Println(len(m), m)
	pm := map[Phasor]int{1: 1, 1i: 2, -1i: 3, -1: 4}
	nm := map[Named]bool{1: true, 1i: false, 2: true}
	fmt.Println(pm, nm)
	km := map[[2]complex64]int{{1, 2}: 1, {1, 1}: 2, {0, 9}: 3}
	fmt.Println(km)
	fmt.Println(fmt.Sprintf("%v", complex(float32(f32()), 1)), real(a), imag(b))
	fmt.Println(g, bump())
	fmt.Println(g*1, bump(), -g, bump())
	fmt.Println(complex(real(g), 0), bump(), Phasor(g), bump())
}

func f32() float32 { return 1.0 / 3 }

var g = 1 + 1i

func bump() complex128 {
	g *= 2
	return g
}
