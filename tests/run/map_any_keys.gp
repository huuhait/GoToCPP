// phase: 05

// map[any]V, map[Iface]V and struct keys that hold interfaces (step 5.6): keys of different
// dynamic types that look alike are different keys (1, int64(1), float64(1)); a NaN key is never
// found; +0 and -0 are one key; pointers are keys by address; deleting during iteration.
package main

import "fmt"

type K struct {
	a int
	b any
}
type Sh interface{ Area() int }
type Sq struct{ s int }

func (q Sq) Area() int { return q.s * q.s }

type Ci struct{ r int }

func (c *Ci) Area() int { return c.r }

type Named int
type Arr [2]any

func keys(m map[any]int) []string {
	var out []string
	for k := range m {
		out = append(out, fmt.Sprintf("%v", k))
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func main() {
	m := map[any]int{}
	m[1] = 1
	m[int64(1)] = 2
	m[float64(1)] = 3
	m[Named(1)] = 4
	m[uint8(1)] = 5
	m["1"] = 6
	m[true] = 7
	m[nil] = 8
	m[1.0] = 9 // float64(1) again
	fmt.Println(len(m), m[1], m[int64(1)], m[float64(1)], m[Named(1)], m[uint8(1)], m["1"], m[true], m[nil])
	_, ok := m[int32(1)]
	fmt.Println(ok)
	// NaN
	nan := 0.0
	nan /= nan
	n := map[any]int{}
	n[nan] = 1
	n[nan] = 2
	fmt.Println(len(n), n[nan])
	total := 0
	for k, v := range n {
		fmt.Println(k, k != k)
		total += v
	}
	fmt.Println(total)
	delete(n, nan)
	fmt.Println(len(n))
	clear(n)
	fmt.Println(len(n))
	// zeros
	z := map[any]string{}
	var nz float64 = 0
	nz = -nz
	z[0.0] = "pos"
	z[nz] = "neg"
	fmt.Println(len(z), z[0.0], z[nz])
	for k := range z {
		fmt.Println(1/k.(float64) > 0)
	}
	// struct keys with ifaces
	sk := map[K]int{}
	sk[K{1, "a"}] = 1
	sk[K{1, "a"}] += 1
	sk[K{1, 2}] = 3
	sk[K{1, nil}] = 4
	fmt.Println(len(sk), sk[K{1, "a"}], sk[K{1, 2}], sk[K{1, nil}], sk[K{2, nil}])
	ak := map[Arr]int{}
	ak[Arr{1, "x"}] = 1
	ak[Arr{1, "x"}]++
	fmt.Println(ak[Arr{1, "x"}], len(ak))
	// interface keys
	im := map[Sh]int{}
	c := &Ci{3}
	im[Sq{2}] = 1
	im[c] = 2
	im[&Ci{3}] = 3
	im[Sq{2}]++
	fmt.Println(len(im), im[Sq{2}], im[c], im[&Ci{3}], im[nil])
	var nilsh Sh
	im[nilsh] = 9
	fmt.Println(im[nil], len(im))
	var pc *Ci
	im[pc] = 10
	fmt.Println(im[nil], im[pc], len(im))
	// delete during iteration
	big := map[any]int{}
	for i := 0; i < 100; i++ {
		big[i] = i
		big[fmt.Sprint(i)] = i
	}
	cnt := 0
	for k := range big {
		cnt++
		delete(big, k)
	}
	fmt.Println(cnt, len(big))
	for i := 0; i < 10; i++ {
		big[i] = i
	}
	for k := range big {
		if ki, ok := k.(int); ok && ki%2 == 0 {
			delete(big, ki+1)
		}
		delete(big, k)
	}
	fmt.Println(len(big))
	fmt.Println(keys(m))
	// error as key
	em := map[error]string{}
	var e1 error = &myE{"a"}
	em[e1] = "one"
	fmt.Println(em[e1], len(em[&myE{"a"}]))
	// pointer keys
	p1, p2 := new(int), new(int)
	pm := map[any]int{p1: 1, p2: 2}
	fmt.Println(pm[p1], pm[p2], len(pm))
	// big many keys, mixed
	mix := map[any]int{}
	for i := 0; i < 2000; i++ {
		mix[i] = i
		mix[int64(i)] = -i
		mix[float64(i)] = i * 2
		mix[fmt.Sprint(i)] = i * 3
		mix[K{i, i}] = i * 4
	}
	sum := 0
	for i := 0; i < 2000; i++ {
		sum += mix[i] + mix[int64(i)] + mix[float64(i)] + mix[fmt.Sprint(i)] + mix[K{i, i}]
	}
	fmt.Println(len(mix), sum)
}

type myE struct{ s string }

func (e *myE) Error() string { return e.s }

// Output:
// 8 1 2 9 4 5 6 7 8
// false
// 2 0
// NaN true
// NaN true
// 3
// 2
// 0
// 1 neg neg
// false
// 3 2 3 4 0
// 2 1
// 3 2 2 0 0
// 9 4
// 9 10 5
// 200 0
// 0
// [1 1 1 1 1 1 <nil> true]
// one 0
// 1 2 2
// 10000 17991000
