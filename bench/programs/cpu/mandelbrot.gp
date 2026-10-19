// bench: mandelbrot
// category: cpu
// size: 2000
// smoke: 100
// twin: yes
// about: Mandelbrot set escape-time loop over an n x n grid (float64)
package main

import "fmt"

func main() {
	n := benchN
	inside, total := 0, 0
	for y := 0; y < n; y++ {
		ci := 2.0*float64(y)/float64(n) - 1.0
		for x := 0; x < n; x++ {
			cr := 2.0*float64(x)/float64(n) - 1.5
			zr, zi := 0.0, 0.0
			i := 0
			for ; i < 50 && zr*zr+zi*zi <= 4.0; i++ {
				zr, zi = zr*zr-zi*zi+cr, 2.0*zr*zi+ci
			}
			total += i
			if i == 50 {
				inside++
			}
		}
	}
	fmt.Printf("%d %d\n", inside, total)
}
