// phase: 03
// diff: stderr

// The evaluation order around complex numbers (step 3.15c): complex, real and
// imag are operators in gc, not calls, so their operands are evaluated like
// any operands (calls first, from left to right, then the reads), and they read
// a package variable or a pointer after the calls of their statement.
package main

var g complex128 = 1 + 1i

var n float64

func next() float64 {
	n++
	return n
}

func bump() complex128 {
	g *= 2
	return g
}

func set(p *complex128) float64 {
	*p = 10i
	return 0
}

func main() {
	c := complex(next(), next())
	println(c, complex(next(), -next()))
	println(real(g)+real(bump()), imag(bump())+imag(g), g)
	println(complex(real(g), next()), complex(next(), imag(bump())))
	x := 3 + 4i
	p := &x
	println(real(*p)+set(p), imag(x), complex(set(p), imag(*p)))
	s := []complex64{1, 2}
	i := 0
	f := func() int {
		s[0] = 5i
		i = 1
		return 0
	}
	println(real(s[i])+float32(f()), imag(s[0]))
	arr := [2]complex128{bump(), bump()}
	println(arr[0], arr[1], real(arr[0]) < real(arr[1]))
	m := map[complex128]float64{}
	m[complex(next(), 0)] = real(complex(next(), 0))
	for k, v := range m {
		println(k, v)
	}
}
