// bench: matmul
// category: cpu
// size: 800
// smoke: 40
// twin: yes
// about: dense float64 matrix multiplication of n x n matrices stored as slices of rows
package main

import "fmt"

func main() {
	n := benchN
	a := make([][]float64, n)
	b := make([][]float64, n)
	c := make([][]float64, n)
	for i := 0; i < n; i++ {
		a[i] = make([]float64, n)
		b[i] = make([]float64, n)
		c[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			a[i][j] = float64((i*j)%7) + 0.5
			b[i][j] = float64((i+j)%5) - 1.25
		}
	}
	for i := 0; i < n; i++ {
		ci := c[i]
		for k := 0; k < n; k++ {
			aik := a[i][k]
			bk := b[k]
			for j := 0; j < n; j++ {
				ci[j] += aik * bk[j]
			}
		}
	}
	trace, sum := 0.0, 0.0
	for i := 0; i < n; i++ {
		trace += c[i][i]
		for j := 0; j < n; j++ {
			sum += c[i][j]
		}
	}
	fmt.Printf("%.6f %.6f\n", trace, sum)
}
