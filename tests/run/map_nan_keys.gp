// phase: 05

// NaN and signed zeros in keys that hold floats (step 5.6): a NaN is never equal to anything,
// so every insertion adds an entry that only range and clear reach; +0 and -0 are one key (the
// later insertion keeps its key's sign); structs, arrays and interfaces with NaN inside behave
// the same.
package main

import "fmt"

type K struct {
	a int
	f float64
}

func main() {
	nan := 0.0
	nan /= nan
	zero := 0.0
	negzero := -zero

	m := map[float64]int{}
	m[nan] = 1
	m[nan] = 2
	m[nan]++
	fmt.Println(len(m), m[nan])
	_, ok := m[nan]
	fmt.Println(ok)
	delete(m, nan)
	fmt.Println(len(m))
	sum := 0
	for k, v := range m {
		fmt.Println(k != k)
		sum += v
	}
	fmt.Println(sum)
	clear(m)
	fmt.Println(len(m))

	// any keys
	a := map[any]int{}
	a[nan] = 1
	a[nan] = 2
	a[float32(nan)] = 3
	a[complex(nan, 0)] = 4
	a[1] = 5
	fmt.Println(len(a), a[nan], a[1])

	// struct and array keys with NaN
	sk := map[K]int{}
	sk[K{1, nan}] = 1
	sk[K{1, nan}] = 2
	sk[K{1, 1}] = 3
	sk[K{1, 1}] = 4
	fmt.Println(len(sk), sk[K{1, nan}], sk[K{1, 1}])
	ak := map[[2]float64]int{}
	ak[[2]float64{1, nan}] = 1
	ak[[2]float64{1, nan}] = 2
	fmt.Println(len(ak))
	ik := map[any]int{}
	ik[K{1, nan}] = 1
	ik[K{1, nan}] = 2
	ik[[2]float64{nan, nan}] = 3
	fmt.Println(len(ik))

	// signed zeros
	z := map[float64]string{}
	z[zero] = "pos"
	z[negzero] = "neg"
	fmt.Println(len(z), z[0], z[negzero])
	zk := map[K]int{}
	zk[K{1, zero}] = 1
	zk[K{1, negzero}] = 2
	fmt.Println(len(zk), zk[K{1, 0}])
	za := map[any]int{}
	za[zero] = 1
	za[negzero] = 2
	za[[1]float64{zero}] = 3
	za[[1]float64{negzero}] = 4
	za[complex(zero, negzero)] = 5
	za[complex(negzero, zero)] = 6
	fmt.Println(len(za), za[0.0], za[[1]float64{0}], za[complex(0, 0)])
	for k := range za {
		if f, ok := k.(float64); ok {
			fmt.Println("key sign negative:", 1/f < 0)
		}
	}
}

// Output:
// 3 0
// false
// 3
// true
// true
// true
// 4
// 0
// 5 0 5
// 3 0 4
// 2
// 3
// 1 neg neg
// 1 2
// 3 2 4 6
// key sign negative: true
