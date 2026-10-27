// phase: 03

// m[k] = append(m[k], f()) where f changes a variable in k: gc takes the map entry once only
// when the two k are the same expression that is safe to evaluate twice (walk/order.go with
// ir.SameSafeExpr), and "the same expression" is what Go has after typecheck, not what the source
// says. A string + is an OADDSTR by then, which SameSafeExpr has no case for: the key is read
// after f, and the append goes to the key that f left. A number + is safe: the key is read
// before f, from the old value. A conversion that gc makes an OCONVNOP is looked through on both
// sides before the comparison, at any depth, so m[k] = append(m[int(k)], f()) is safe too; a
// conversion that survives as OCONV is safe when its result is a simple type (SameSafeExpr's
// OCONV case). A field selected through an embedded struct is the same chain of ODOT/ODOTPTR
// nodes to gc either way (t.V is rewritten to the explicit t.Inner.V, with an Implicit marker on
// the inner node), and SameSafeExpr compares the nodes one by one. An index of a pointer to an
// array is wrapped in an implicit deref by typecheck's implicitstar, so parr[i] and (*parr)[i]
// are the same chain too. Each line prints the map after one append, or the counts at the old and
// the new key where the key is not printable.
package main

import (
	"fmt"
	"unsafe"
)

var (
	ki   int     = 1
	ku32 uint32  = 1
	kf64 float64 = 1
	k8   int8    = 1
	ks           = "one"
	ko           = "x"
	tv           = T{Inner{1}}
	x, y int
	kp   = &x
	arr  = [2]int{1, 2}
	parr = &arr
)

type Inner struct{ V int }
type T struct{ Inner }
type TP struct{ *Inner }
type MyInt int
type S string

var (
	tagp TP
	anp  *int
	myi  MyInt = 1
)

// changeArr sets arr[0] as change sets the variables, for the pointer-to-array index rows.
func changeArr() int {
	arr[0] = 5
	return 7
}

func change() int {
	ki, ku32, kf64, k8, ks, myi = 5, 5, 5, 5, "five", 5
	return 7
}

func reset() {
	ki, ku32, kf64, k8, ks, myi = 1, 1, 1, 1, "one", 1
}

// setkf sets kf64 as change sets the others, for the float conversion that gc does not strip.
func setkf() int {
	kf64 = 5
	return 7
}

// changeField sets the field of the T that p points to, as change sets the variables.
func changeField(p *T) int {
	p.Inner.V = 5
	return 7
}

func main() {
	// One side of the int and uint32 rows carries a conversion that gc makes an OCONVNOP, so the
	// row needs stripConvNop to see one expression; as the same source node on both sides it
	// would pass on a sameSafe that never strips anything. The float64 row's conversion is the
	// one gc keeps as an OCONV, so that row goes the other way (SameSafeExpr's OCONV case).
	reset()
	mi := map[int][]int{}
	mi[ki+1] = append(mi[int(ki)+1], change())
	fmt.Println("int + int:", mi)
	reset()
	mi = map[int][]int{}
	mi[1+ki] = append(mi[1+int(ki)], change())
	fmt.Println("constant + int:", mi)
	reset()
	mu := map[uint32][]int{}
	mu[ku32+1] = append(mu[uint32(ku32)+1], change())
	fmt.Println("uint32 + uint32:", mu)
	reset()
	mf := map[float64][]int{}
	// This row's printed map cannot tell the fast path from the slow one: a key that is not a
	// name goes into a temporary either way, and both temporaries are read before change(). The
	// float + rule is pinned by TestMapAppendShape's "float addition" row instead.
	mf[kf64+1] = append(mf[float64(kf64)+1], change())
	fmt.Println("float64 + float64:", mf)

	// A conversion of the same kind is the one conversion gc does not make an OCONVNOP: tcConv
	// keeps float64(kf) of a float64 an OCONV, and SameSafeExpr's OCONV case does not strip it,
	// because it rounds and a NaN would make the two keys differ. Both keys exist here, so the
	// fast path would send the value to hold[1]; gc reads the append key after setkf instead.
	reset()
	hold := map[float64][]int{1: {1}, 5: {99}}
	hold[float64(kf64)] = append(hold[kf64], setkf())
	fmt.Println("float conversion split:", hold[1], hold[5])
	reset()
	ms := map[string][]int{}
	ms[ks+"x"] = append(ms[ks+"x"], change())
	fmt.Println("string + string:", ms)
	reset()
	ms = map[string][]int{}
	ms["x"+ks] = append(ms["x"+ks], change())
	fmt.Println("constant + string:", ms)
	reset()
	ms = map[string][]int{}
	ms[ks+ko] = append(ms[ks+ko], change())
	fmt.Println("string + a string that doesn't change:", ms)
	reset()
	ms = map[string][]int{}
	ms[ks+ks] = append(ms[ks+ks], change())
	fmt.Println("string + itself:", ms)

	// string(ks) of a string is an OCONVNOP (typecheck's convertOp rule 1), which SameSafeExpr
	// strips from both sides: the two keys are the same ONAME, so gc takes the entry once and
	// reads it before f. The right hand side stays m[ks] below, so this also pins that only one
	// side needs the conversion.
	reset()
	ms = map[string][]int{}
	ms[string(ks)] = append(ms[string(ks)], change())
	fmt.Println("string conversion on both sides:", ms)
	reset()
	ms = map[string][]int{}
	ms[string(ks)] = append(ms[ks], change())
	fmt.Println("string conversion on the left:", ms)

	// A conversion over a named type is an OCONVNOP too, of the same representation: S(ks) of a
	// named string, MyInt(myi) of a named int. One side keeps the plain name, so the strip has to
	// make the two keys one expression; without it they are different nodes and gc's order (the
	// key before f) is lost.
	reset()
	mS := map[S][]int{}
	mS[S(ks)] = append(mS[S(ks)], change())
	fmt.Println("named string conversion:", len(mS["one"]), len(mS["five"]))
	reset()
	mm := map[MyInt][]int{}
	mm[MyInt(myi)] = append(mm[myi], change())
	fmt.Println("named int conversion:", len(mm[1]), len(mm[5]))
	reset()
	mi = map[int][]int{}
	mi[ki] = append(mi[int(ki)], change())
	fmt.Println("no-op conversion on the right:", mi)
	reset()
	mi = map[int][]int{}
	mi[int(ki)] = append(mi[ki], change())
	fmt.Println("no-op conversion on the left:", mi)
	reset()
	m64 := map[int64][]int{}
	m64[int64(int(ki))] = append(m64[int64(ki)], change())
	fmt.Println("int -> int64:", m64)
	// A chain of no-op conversions has no depth limit in gc (SameSafeExpr loops while the node is
	// an OCONVNOP), so a chain deeper than any cap gocpp might set leaves the two keys one
	// expression. The chain here is 150 conversions long.
	reset()
	mi = map[int][]int{}
	mi[int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(int(ki))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))))] = append(mi[ki], change())
	fmt.Println("deep conversion chain:", mi)
	// gc wraps an int64 key in a conversion before the map call (wrappedKey), an int8 key not. So
	// the int8 row's left key is only a temporary and its right key is the plain name, read after
	// f unless gc takes the entry once; both keys are prefilled, so the two orders leave the
	// value in different places. As the same conversion on both sides the row would pass either
	// way.
	reset()
	mi8 := map[int8][]int{1: {1}, 5: {99}}
	mi8[int8(k8)] = append(mi8[k8], change())
	fmt.Println("int -> int8:", mi8[1], mi8[5])
	tv = T{Inner{1}}
	mt := map[int][]int{}
	t := &tv
	mt[t.V] = append(mt[t.Inner.V], changeField(t))
	fmt.Println("promoted and explicit field:", mt)
	tv = T{Inner{1}}
	mv := map[int][]int{}
	mv[t.Inner.V] = append(mv[t.V], changeField(t))
	fmt.Println("explicit and promoted field:", mv)

	// gc's OCONVNOP set is wider than the numeric conversions: a pointer to unsafe.Pointer and
	// back (convertOp rules 8 and 9) and nil to a nillable type (assignOp rule 5) are OCONVNOPs
	// too, and SameSafeExpr looks through them. (A channel to one direction is one as well,
	// assignOp rule 4, but a channel is a Phase 08 feature. Two struct types that differ only in
	// a field tag are one by rule 2, but a struct key is copied by the slow path either way, so
	// the order is the same here whether SameSafeExpr takes the entry once or not.)
	x, y = 0, 0
	kp = &x
	mup := map[unsafe.Pointer][]int{}
	mup[unsafe.Pointer(kp)] = append(mup[unsafe.Pointer(kp)], func() int { kp = &y; return 7 }())
	fmt.Println("pointer to unsafe.Pointer:", len(mup[unsafe.Pointer(&x)]), len(mup[unsafe.Pointer(&y)]))
	anp = nil
	mnil := map[*int][]int{}
	mnil[(*int)(anp)] = append(mnil[(*int)(anp)], func() int { anp = &x; return 7 }())
	fmt.Println("nil to a pointer:", len(mnil[nil]), len(mnil[&x]))
	x, y = 0, 0
	pp := &x
	mpp := map[*int][]int{}
	mpp[(*int)(pp)] = append(mpp[(*int)(pp)], func() int { pp = &y; return 7 }())
	fmt.Println("pointer to the same pointer type:", len(mpp[&x]), len(mpp[&y]))

	// A field promoted through an embedded pointer: gc builds a DOTPTR over an implicit DOT for
	// t.V and an explicit t.Inner.V, and SameSafeExpr compares those nodes one by one.
	tagp = TP{&Inner{1}}
	pt := &tagp
	mep := map[int][]int{}
	mep[tagp.V] = append(mep[tagp.Inner.V], func() int { pt.Inner = &Inner{5}; return 7 }())
	fmt.Println("promoted through an embedded pointer:", len(mep[1]), len(mep[5]))

	// An index of a pointer to an array: typecheck's implicitstar wraps parr[i] in an implicit
	// ODEREF, so parr[0] and (*parr)[0] are one ODEREF(ONAME) chain to SameSafeExpr and gc takes
	// the entry once, whatever spelling the two sides use. changeArr sets arr[0], so the two
	// spellings must leave the value at key 1, the key read before it.
	arr = [2]int{1, 2}
	marr := map[int][]int{}
	marr[(*parr)[0]] = append(marr[parr[0]], changeArr())
	fmt.Println("pointer to array, star on the left:", marr)
	arr = [2]int{1, 2}
	marr = map[int][]int{}
	marr[parr[0]] = append(marr[(*parr)[0]], changeArr())
	fmt.Println("pointer to array, star on the right:", marr)
}
