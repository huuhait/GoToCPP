// phase: 03

// m[k] = append(m[k], f()) where f changes the variable k: gc reads k before f runs for the key
// types that its fast map calls convert (walk/order.go mapKeyTemp: int, int32, int64, uint,
// uintptr and pointers, and arrays that it hashes as 4 or 8 bytes of memory or as one string),
// and after f for the others (string, uint32, uint64, int8, int16, bool, float64, other arrays).
// Each line prints which key got the value. A later Go that changes the rule shows up here.
package main

import "fmt"

var (
	ki   int     = 1
	ki32 int32   = 1
	ki64 int64   = 1
	ku   uint    = 1
	kup  uintptr = 1
	ku32 uint32  = 1
	ku64 uint64  = 1
	ki8  int8    = 1
	ki16 int16   = 1
	kb           = false
	kf           = 1.0
	ks           = "one"
	x, y int
	kp   = &x
	ka32 = [2]int32{1, 1}
	ka64 = [2]int{1, 1}
	ka1s = [1]string{"one"}
	ka8  = [2]uint8{1, 1}
	ka1f = [1]float64{1}
)

func change() int {
	ki, ki32, ki64, ku, kup, ku32, ku64, ki8, ki16 = 5, 5, 5, 5, 5, 5, 5, 5, 5
	kb, kf, ks, kp = true, 5, "five", &y
	ka32, ka64, ka1s, ka8, ka1f = [2]int32{5, 5}, [2]int{5, 5}, [1]string{"five"}, [2]uint8{5, 5}, [1]float64{5}
	return 7
}

func reset() {
	ki, ki32, ki64, ku, kup, ku32, ku64, ki8, ki16 = 1, 1, 1, 1, 1, 1, 1, 1, 1
	kb, kf, ks, kp = false, 1, "one", &x
	ka32, ka64, ka1s, ka8, ka1f = [2]int32{1, 1}, [2]int{1, 1}, [1]string{"one"}, [2]uint8{1, 1}, [1]float64{1}
}

func main() {
	mi := map[int][]int{}
	mi[ki] = append(mi[ki], change())
	fmt.Println("int", len(mi[1]), len(mi[5]))
	reset()
	pm := &mi
	(*pm)[ki] = append((*pm)[ki], change())
	fmt.Println("int via *p", len(mi[1]), len(mi[5]))
	reset()
	mi32 := map[int32][]int{}
	mi32[ki32] = append(mi32[ki32], change())
	fmt.Println("int32", len(mi32[1]), len(mi32[5]))
	reset()
	mi64 := map[int64][]int{}
	mi64[ki64] = append(mi64[ki64], change())
	fmt.Println("int64", len(mi64[1]), len(mi64[5]))
	reset()
	mu := map[uint][]int{}
	mu[ku] = append(mu[ku], change())
	fmt.Println("uint", len(mu[1]), len(mu[5]))
	reset()
	mup := map[uintptr][]int{}
	mup[kup] = append(mup[kup], change())
	fmt.Println("uintptr", len(mup[1]), len(mup[5]))
	reset()
	mu32 := map[uint32][]int{}
	mu32[ku32] = append(mu32[ku32], change())
	fmt.Println("uint32", len(mu32[1]), len(mu32[5]))
	reset()
	mu64 := map[uint64][]int{}
	mu64[ku64] = append(mu64[ku64], change())
	fmt.Println("uint64", len(mu64[1]), len(mu64[5]))
	reset()
	mi8 := map[int8][]int{}
	mi8[ki8] = append(mi8[ki8], change())
	fmt.Println("int8", len(mi8[1]), len(mi8[5]))
	reset()
	mi16 := map[int16][]int{}
	mi16[ki16] = append(mi16[ki16], change())
	fmt.Println("int16", len(mi16[1]), len(mi16[5]))
	reset()
	mb := map[bool][]int{}
	mb[kb] = append(mb[kb], change())
	fmt.Println("bool", len(mb[false]), len(mb[true]))
	reset()
	mf := map[float64][]int{}
	mf[kf] = append(mf[kf], change())
	fmt.Println("float64", len(mf[1]), len(mf[5]))
	reset()
	ms := map[string][]int{}
	ms[ks] = append(ms[ks], change())
	fmt.Println("string", len(ms["one"]), len(ms["five"]))
	reset()
	mp := map[*int][]int{}
	mp[kp] = append(mp[kp], change())
	fmt.Println("pointer", len(mp[&x]), len(mp[&y]))
	reset()
	ma32 := map[[2]int32][]int{}
	ma32[ka32] = append(ma32[ka32], change())
	fmt.Println("[2]int32", len(ma32[[2]int32{1, 1}]), len(ma32[[2]int32{5, 5}]))
	reset()
	ma64 := map[[2]int][]int{}
	ma64[ka64] = append(ma64[ka64], change())
	fmt.Println("[2]int", len(ma64[[2]int{1, 1}]), len(ma64[[2]int{5, 5}]))
	reset()
	ma1s := map[[1]string][]int{}
	ma1s[ka1s] = append(ma1s[ka1s], change())
	fmt.Println("[1]string", len(ma1s[[1]string{"one"}]), len(ma1s[[1]string{"five"}]))
	reset()
	ma8 := map[[2]uint8][]int{}
	ma8[ka8] = append(ma8[ka8], change())
	fmt.Println("[2]uint8", len(ma8[[2]uint8{1, 1}]), len(ma8[[2]uint8{5, 5}]))
	reset()
	ma1f := map[[1]float64][]int{}
	ma1f[ka1f] = append(ma1f[ka1f], change())
	fmt.Println("[1]float64", len(ma1f[[1]float64{1}]), len(ma1f[[1]float64{5}]))
	reset()

	// A local key that a pointer changes (a boxed variable), and the other forms of a store,
	// which read every key after f.
	lk := 1
	plk := &lk
	mi = map[int][]int{}
	mi[lk] = append(mi[lk], set(plk))
	fmt.Println("boxed int", len(mi[1]), len(mi[5]))
	lk = 1
	mo := map[int]int{}
	mo[lk] = set(plk)
	lk = 1
	mo[lk] += set(plk)
	fmt.Println("store and op=", mo[1], mo[5])
}

func set(p *int) int {
	*p = 5
	return 7
}
