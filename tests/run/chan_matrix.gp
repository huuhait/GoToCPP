// phase: 08

// A matrix product with one goroutine per row; each sends its row index and the row.
package main

import "fmt"

type row struct {
	i    int
	vals [4]int
}

func main() {
	var a, b [4][4]int
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			a[i][j] = i + j
			b[i][j] = i*j + 1
		}
	}
	rows := make(chan row)
	for i := 0; i < 4; i++ {
		go func() {
			var r row
			r.i = i
			for j := 0; j < 4; j++ {
				for k := 0; k < 4; k++ {
					r.vals[j] += a[i][k] * b[k][j]
				}
			}
			rows <- r
		}()
	}
	var c [4][4]int
	for n := 0; n < 4; n++ {
		r := <-rows
		c[r.i] = r.vals
	}
	for _, r := range c {
		fmt.Println(r)
	}
}

// Output:
// [6 20 34 48]
// [10 30 50 70]
// [14 40 66 92]
// [18 50 82 114]
