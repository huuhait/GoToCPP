// phase: 03

// m[k] = append(m[k], f()) where f changes k, for keys of named types (step 3.9; the unnamed
// ones are in maps_append_key.gp): gc converts a key to its fast map call's key type (uint32,
// uint64 or string) unless the key already has that exact type (walk/order.go mapKeyTemp), so a
// named type over uint32, uint64 or string is read before f, like int and pointers, while
// named int8, bool and float64 keys (slow map calls) are read after f. Each line prints which
// key got the value.
package main

import "fmt"

type (
	NI   int
	NU32 uint32
	NU64 uint64
	NS   string
	NI8  int8
	NB   bool
	NF   float64
	NP   *int
	NA   [2]int32
	NA8  [2]uint8
)

var (
	ki   NI   = 1
	ku32 NU32 = 1
	ku64 NU64 = 1
	ks   NS   = "one"
	ki8  NI8  = 1
	kb   NB
	kf   NF = 1
	x, y int
	kp   NP = &x
	ka   = NA{1, 1}
	ka8  = NA8{1, 1}
)

func change() int {
	ki, ku32, ku64, ks, ki8, kb, kf, kp = 5, 5, 5, "five", 5, true, 5, &y
	ka, ka8 = NA{5, 5}, NA8{5, 5}
	return 7
}

func reset() {
	ki, ku32, ku64, ks, ki8, kb, kf, kp = 1, 1, 1, "one", 1, false, 1, &x
	ka, ka8 = NA{1, 1}, NA8{1, 1}
}

func main() {
	mi := map[NI][]int{}
	mi[ki] = append(mi[ki], change())
	fmt.Println("NI", len(mi[1]), len(mi[5]))
	reset()
	mu32 := map[NU32][]int{}
	mu32[ku32] = append(mu32[ku32], change())
	fmt.Println("NU32", len(mu32[1]), len(mu32[5]))
	reset()
	mu64 := map[NU64][]int{}
	mu64[ku64] = append(mu64[ku64], change())
	fmt.Println("NU64", len(mu64[1]), len(mu64[5]))
	reset()
	ms := map[NS][]int{}
	ms[ks] = append(ms[ks], change())
	fmt.Println("NS", len(ms["one"]), len(ms["five"]))
	reset()
	mi8 := map[NI8][]int{}
	mi8[ki8] = append(mi8[ki8], change())
	fmt.Println("NI8", len(mi8[1]), len(mi8[5]))
	reset()
	mb := map[NB][]int{}
	mb[kb] = append(mb[kb], change())
	fmt.Println("NB", len(mb[false]), len(mb[true]))
	reset()
	mf := map[NF][]int{}
	mf[kf] = append(mf[kf], change())
	fmt.Println("NF", len(mf[1]), len(mf[5]))
	reset()
	mp := map[NP][]int{}
	mp[kp] = append(mp[kp], change())
	fmt.Println("NP", len(mp[&x]), len(mp[&y]))
	reset()
	ma := map[NA][]int{}
	ma[ka] = append(ma[ka], change())
	fmt.Println("NA", len(ma[NA{1, 1}]), len(ma[NA{5, 5}]))
	reset()
	ma8 := map[NA8][]int{}
	ma8[ka8] = append(ma8[ka8], change())
	fmt.Println("NA8", len(ma8[NA8{1, 1}]), len(ma8[NA8{5, 5}]))
	reset()

	// A map of a named map type, with a named slice value.
	type Lists []int
	type Index map[NS]Lists
	ix := Index{}
	ix[ks] = append(ix[ks], change())
	fmt.Println("Index", len(ix["one"]), len(ix["five"]))
}
