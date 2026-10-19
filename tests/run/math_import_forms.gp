// phase: 09
// Import forms: an alias, local names that look like math functions (exp, log, sqrt, floor), and package-level variables initialized from math.

package main

import (
	"fmt"
	m "math"
)

var (
	tau = 2 * m.Pi
	root2 = m.Sqrt(2)
	table = [...]float64{m.Sin(1), m.Cos(1), m.Exp(1), m.Log(2)}
	big = m.Pow(2, 100)
)


func exp(x float64) float64 { return m.Exp(x) * 2 }

func main() {
	log := m.Log
	sqrt := func(x float64) float64 { return m.Sqrt(x) + 1 }
	floor := 3.0
	abs := -2.5
	fmt.Println(tau, root2, table, big, exp(1), log(10), sqrt(4), floor, m.Abs(abs))
	fmt.Printf("%016x %016x %016x\n", m.Float64bits(tau), m.Float64bits(root2), m.Float64bits(big))
	const pi2 = m.Pi / 2
	var x float32 = m.Pi
	fmt.Println(pi2, x, m.MaxInt32, m.MaxUint32, uint32(m.MaxUint32))
}

// Output:
// 6.283185307179586 1.4142135623730951 [0.8414709848078965 0.5403023058681398 2.718281828459045 0.6931471805599453] 1.2676506002282294e+30 5.43656365691809 2.302585092994046 3 3 2.5
// 401921fb54442d18 3ff6a09e667f3bcd 4630000000000000
// 1.5707963267948966 3.1415927 2147483647 4294967295 4294967295
