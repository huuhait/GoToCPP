// phase: 03

// Matrices as arrays of arrays (values, copied) and as slices of slices (shared rows):
// multiplication, transposition, identity, equality of arrays, and a jagged triangle.
package main

import "fmt"

type M3 [3][3]int

func mul(a, b M3) (c M3) {
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				c[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return
}

func (m M3) T() M3 {
	for i := 0; i < 3; i++ {
		for j := i + 1; j < 3; j++ {
			m[i][j], m[j][i] = m[j][i], m[i][j]
		}
	}
	return m
}

func grid(r, c int) [][]float64 {
	g := make([][]float64, r)
	for i := range g {
		g[i] = make([]float64, c)
		for j := range g[i] {
			g[i][j] = float64(i) + float64(j)/10
		}
	}
	return g
}

func main() {
	id := M3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	a := M3{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}}
	fmt.Println(mul(a, id) == a, mul(id, a) == a)
	fmt.Println(mul(a, a))
	t := a.T()
	fmt.Println(t, a[0], t.T() == a)

	g := grid(2, 3)
	row := g[1]
	row[2] = -1 // shares g's row
	h := g      // shares everything
	h[0][0] = 42
	fmt.Println(g, len(g), len(g[0]))

	tri := [][]int{}
	for i := 0; i < 5; i++ {
		r := make([]int, i+1)
		r[0], r[i] = 1, 1
		for j := 1; j < i; j++ {
			r[j] = tri[i-1][j-1] + tri[i-1][j]
		}
		tri = append(tri, r)
	}
	fmt.Println(tri)
}

// Output:
// true true
// [[30 36 42] [66 81 96] [102 126 150]]
// [[1 4 7] [2 5 8] [3 6 9]] [1 2 3] true
// [[42 0.1 0.2] [1 1.1 -1]] 2 3
// [[1] [1 1] [1 2 1] [1 3 3 1] [1 4 6 4 1]]
