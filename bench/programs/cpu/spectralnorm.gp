// bench: spectralnorm
// category: cpu
// size: 2000
// smoke: 100
// twin: yes
// about: spectral norm of an infinite matrix (float64 loops with a division per element)
package main

import "fmt"

func evalA(i, j int) float64 {
	return 1 / float64((i+j)*(i+j+1)/2+i+1)
}

func times(out, v []float64) {
	for i := range out {
		sum := 0.0
		for j := range v {
			sum += evalA(i, j) * v[j]
		}
		out[i] = sum
	}
}

func timesTransp(out, v []float64) {
	for i := range out {
		sum := 0.0
		for j := range v {
			sum += evalA(j, i) * v[j]
		}
		out[i] = sum
	}
}

func timesAtA(out, v, tmp []float64) {
	times(tmp, v)
	timesTransp(out, tmp)
}

func main() {
	n := benchN
	u := make([]float64, n)
	v := make([]float64, n)
	tmp := make([]float64, n)
	for i := range u {
		u[i] = 1
	}
	for i := 0; i < 10; i++ {
		timesAtA(v, u, tmp)
		timesAtA(u, v, tmp)
	}
	var vBv, vv float64
	for i := 0; i < n; i++ {
		vBv += u[i] * v[i]
		vv += v[i] * v[i]
	}
	fmt.Printf("%.9f\n", sqrt(vBv/vv))
}
