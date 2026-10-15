// phase: 03
// diff: stderr

// Complex numbers as map keys and values, and in slices, arrays, struct fields
// (also as keys), pointers and closures (step 3.15c). Keys compare with ==: +0
// and -0 are one key, and a key with a NaN part is a new entry each time, never
// found again. Go's map order is random, so the program prints sums and counts.
package main

type Point struct {
	at   complex128
	name string
}

type Grid [2]complex64

func sum(m map[complex128]int) (n, total int) {
	for k, v := range m {
		if k == k {
			n++
		}
		total += v
	}
	return
}

func rotate(p *complex128, by complex128) {
	*p *= by
}

func pair() (complex64, complex128) {
	return 1 - 1i, 2i
}

func main() {
	zero := 0.0
	negz := -zero
	nan := zero / zero

	m := map[complex128]int{}
	m[complex(1, zero)] = 1
	m[complex(1, negz)] = 2
	m[complex(negz, negz)] = 3
	m[0] += 10
	m[complex(nan, 0)] = 4
	m[complex(nan, 0)] = 5
	m[complex(0, nan)] = 6
	m[2i]++
	m[2i] *= 3
	v, ok := m[complex(nan, 0)]
	v1, ok1 := m[complex(1, 0)]
	println(len(m), m[1], m[complex(negz, 0)], m[2i], v, ok, v1, ok1)
	n, total := sum(m)
	println(n, total)
	delete(m, complex(nan, 0))
	delete(m, complex(1, negz))
	println(len(m))
	clear(m)
	println(len(m))

	m64 := map[complex64]complex64{1: 1i, 1i: -1}
	m64[1] *= m64[1i]
	m64[1+1i] = m64[1] + m64[1i]
	println(len(m64), m64[1], m64[1+1i], m64[complex64(complex(1, negz))])

	pk := map[Point]int{{1i, "a"}: 1}
	pk[Point{complex(negz, 1), "a"}] += 5
	pk[Point{complex(nan, 1), "a"}] = 7
	println(len(pk), pk[Point{1i, "a"}])

	gk := map[Grid]string{{1, 2}: "x"}
	gk[Grid{1, complex64(complex(2, negz))}] += "y"
	println(len(gk), gk[Grid{1, 2}])

	s := []complex128{1, 2, 3}
	s = append(s, s[0]*s[1])
	for i, c := range s {
		s[i] = c * 1i
	}
	println(len(s), s[0], s[3])
	var g Grid
	g[1] = 3 - 4i
	g2 := g
	g2[0]++
	println(g[0], g[1], g2[0], g == g2, g2 != Grid{1, 3 - 4i})

	p := Point{at: 1}
	rotate(&p.at, 1i)
	rotate(&s[0], 1i)
	c := 2 + 0i
	q := &c
	*q /= 4
	println(p.at, s[0], c, *q)

	acc := complex64(0)
	add := func(d complex64) complex64 {
		acc += d
		return acc
	}
	add(1)
	add(1i)
	println(add(-0.5), acc)
	x, y := pair()
	println(x, y, complex128(x)+y)
}
