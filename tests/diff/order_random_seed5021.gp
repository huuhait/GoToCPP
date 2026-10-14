// phase: 03

// The program of seed 5021 of TestRandomOrder (internal/cppgen/order_random_test.go), as the
// generator of step 3.9 wrote it (with aliases for the named types). Real Go printed other
// values than gocpp for it, but gocpp was right: gc 1.25 on amd64 folded the huge shift count
// in copy(gsl[(e(23, 3) >> (1<<(int((p - p)) & 127) - 2)) & 3:], gal) and then miscompiled the
// index (tests/diff/shift_huge_count.gp). The counts now read gz, as the generator writes them,
// so that both compilers agree with the Go spec.
package main

import "fmt"

var g0, g1 = 1, 2
var gs = "abcde"
var gb = true
var g8 uint8 = 200

// gz is always 0: the huge shift counts read it, so that gc can't fold them.
var gz = 0

// pg points to g0 or g1, pl to a boxed local of main, pc to a loop's variable (or g1).
var pg = &g0
var pl, pc *int

// e prints its tag and changes the package variables, also through the pointers, and where pg
// points.
func e(tag, v int) int {
	fmt.Print("e", tag, " ")
	g0 += tag
	g1 = g1*3 + v
	gb = !gb
	g8 += uint8(tag)
	if tag%3 == 0 {
		gs += "x"
	}
	if tag%4 == 1 {
		if pg == &g0 {
			pg = &g1
		} else {
			pg = &g0
		}
	}
	if pl != nil {
		*pl += tag
	}
	reshape(tag)
	rearray(tag)
	remap(tag)
	restruct(tag)
	renamed(tag)
	return v
}

// N is a named int8 and NS a named slice of them. gn and gns (always 4 elements) are changed by
// e: arithmetic that wraps, stores into gns and appends in place and into a new array.
type N = int8
type NS = []N

var gn N = 3
var gns = NS{1, 2, 3, 4}

func renamed(tag int) {
	switch tag % 5 {
	case 1:
		gn = gn*3 + N(tag)
	case 2:
		gns[tag&3] -= N(tag) << 2
	case 3:
		gns = append(gns[:3], gn)
	case 4:
		gns = append(NS(nil), gns...)
	}
}

// S is a struct with a nested anonymous struct and an array. gst is a package struct, gss a
// slice and gsa an array of them, pst points to one of them or to lst, a boxed local struct of
// main (pls points to it), and pfa to a field of one. e changes them all.
type S struct {
	a, b int
	in   struct{ c int }
	arr  [2]int
}

var gst = S{a: 1, b: 2}
var gss = []S{{a: 3}, {b: 4}}
var gsa [2]S
var pst = &gst
var pls *S
var pfa = &gst.b

func restruct(tag int) {
	switch tag % 6 {
	case 1:
		gst.a += tag
	case 2:
		gss[tag&1].b -= tag
	case 3:
		gsa[tag&1].in.c += tag
	case 4:
		if pls != nil {
			pls.arr[tag&1] *= 2
		}
	case 5:
		pst.b++
	}
	if tag%7 == 4 {
		switch tag % 3 {
		case 0:
			pst = &gss[tag&1]
		case 1:
			pst = &gsa[tag&1]
		default:
			if pls != nil {
				pst = pls
			} else {
				pst = &gst
			}
		}
	}
	if tag%8 == 3 {
		pfa = &gss[tag&1].a
	}
}

// gsl always has 5 elements, gal 2 of its array (or of an older one), and psl points into an
// array of gsl. e changes them: appends in place (gsl[:4] has room) and into a new array
// (gsl[:4:4] has none), so what the program prints doesn't depend on how a slice grows.
var gsl = []int{1, 2, 3, 4, 5}
var gal = gsl[1:3]
var psl = &gsl[2]

func reshape(tag int) {
	switch tag % 7 {
	case 2:
		gsl = append(gsl[:4], tag)
	case 3:
		gsl[tag&3] += tag
	case 4:
		gsl = append([]int(nil), gsl...)
	case 5:
		gal = gsl[tag&1 : 2+tag&1]
	case 6:
		gsl = append(gsl[:4:4], -tag)
	}
	if tag%6 == 1 {
		gal[tag&1] -= 1
	}
	if tag%8 == 6 {
		psl = &gsl[tag&3]
	}
}

// gar is a package array, gas a slice of its array (2 elements), and gpa points into it or into
// la, a boxed local array of main, which pla points to. e changes them all.
var gar = [4]int{10, 20, 30, 40}
var gas = gar[1:3]
var gpa = &gar[2]
var pla *[4]int

func rearray(tag int) {
	switch tag % 5 {
	case 1:
		gar[tag&3] += tag
	case 2:
		gas[tag&1] -= tag
	case 3:
		if pla != nil {
			pla[tag&3] *= 2
		}
	case 4:
		gar = [4]int{tag, gar[0], gar[1], gar[2]}
	}
	if tag%9 == 4 {
		gpa = &gar[tag&3]
	}
	if tag%7 == 3 {
		gas = gar[tag&1 : 2+tag&1]
	}
}

// gmp has keys in 0..3; e deletes from it, stores into it and makes a new one. gml shares its
// table with main's local map lm, so a call changes a local map. gms is a map of slices.
var gmp = map[int]int{0: 1, 2: 3}
var gml = map[int]int{1: 1}
var gms = map[int][]int{}

func remap(tag int) {
	switch tag % 5 {
	case 1:
		delete(gmp, tag&3)
	case 2:
		gmp[tag&3] += tag
	case 3:
		gml[tag&3] = tag
	case 4:
		if tag%3 == 0 {
			gmp = map[int]int{tag & 3: tag, 3: -tag}
		} else {
			gms[tag&3] = append(gms[tag&3], tag)
		}
	}
}

// sum prints its tag and returns the sum of s.
func sum(tag int, s []int) int {
	fmt.Print("s", tag, " ")
	t := 0
	for _, v := range s {
		t += v
	}
	return t
}

// pick prints its tag and returns p: a pointer from a call.
func pick(tag int, p *int) *int {
	fmt.Print("p", tag, " ")
	return p
}

func eb(tag int, v bool) bool {
	fmt.Print("b", tag, " ")
	g0 -= tag
	gb = v
	return v
}

func both(a, b int) int {
	return a*7 + b
}

func h0(p int) (int, int) {
	pp := &p
	*pp += 1
	for v1, v := range gsl[:2+p&3] {
		fmt.Println("v1", v1, v, e(1, -1))
		pst, pst.a = &gss[p & 1], v1
	}
	v2, v3 := 8, e(2, 1)
	fmt.Println(v2, v3)
	return len(append(gal, gsl[(0 * 5) & 3])), e(3, 1)
}

func k0(p int) int {
	gpa = &gar[-e(4, 0) & 3]
	return (5 - (len(gs) - (p - p)))
}

func h1(p int) (int, int) {
	fmt.Println(g8, gs[(1 / (k0(*pick(5, pfa)) | 1)) & 3:k0((e(6, 0) * *gpa)) & 3+1] + gs[:k0(e(7, e(8, 2))) & 3] + "-", !((gb && gb)), gal[-p & 1])
	for v4 := range (p + -0) & 3 {
		fmt.Println("v4", v4)
		var v4, v5 = e(9, 1), ((e(10, 0) - v4) * (e(11, -1) * v4))
		fmt.Println(v4, v5)
		switch k0(v5) {
		case -1:
			v6, v7 := h0(copy(gal, gsl[int(gs[*pg & 3]) & 1:]))
			fmt.Println(v6, v7, both(h0(*pfa)))
		case (int(gs[(1 % (p | 1)) & 3]) + (v5 * v5)), int(gs[*pg & 3]), ((e(12, 2) / (-e(13, 3) | 1)) * e(14, v4)):
			fmt.Println(gar == gar, copy(gal, gsl[e(15, 1) & 1:]))
		case v4, v4, k0(both(e(16, 0), p)):
			gar = [4]int{(v5 + p), *pick(17, psl), 3, 7}
		default:
			var v8, v9 = ((gsl[0 & 3] % ((int(gs[v4 & 3]) + int(gs[0 & 3])) | 1)) - (e(18, 3) + p)), v4
			fmt.Println(v8, v9)
		}
	}
	return (8 / (e(19, -2) | 1)), (((0 * 4) % (((p + e(20, 4)) * p) | 1)) % (k0(p) + (e(21, 4) << (uint(int((6 + e(22, 0)))) & 7))))
}

func k1(p int) int {
	_ = copy(gsl[(e(23, 3) >> (1<<(int((p - p))&127+gz) - 2)) & 3:], gal)
	return int(gs[(int(gs[gsl[(e(24, 3) - p) & 3] & 3]) * (e(25, 4) << (uint(int(both(p, e(26, 1)))) & 7))) & 3])
}

func h2(p int) (int, int) {
	pp := &p
	*pp += 0
	for v10 := 0; v10 < 3 && ((eb(28, eb(29, e(30, 3) < e(31, 0))) && !(g0 < len(gs)))); v10++ {
		fmt.Println((e(32, -1) < *pg && gmp[(e(33, -2) + e(34, 0)) & 3] == p), gsl[k1(p) & 3], int8(int((gst.in.c + (p - p)))), gs[(int(gs[((1 - v10) + k1(7)) & 3]) - e(37, *pg)) & 3:(-e(38, 2) + (*pg << (uint(int(k0(*pfa))) & 7))) & 3+1])
		gas[(p << (uint(int(both(e(41, 2), 4))) & 7)) & 1] /= (-e(40, -2) | 1)
	}
	for v11 := range gar {
		fmt.Println("v11", v11)
		v12 := 0
		for v12 < 3 && (gb && g0 < *psl) {
			v12++
			ln13 := NS{gn, gn}
			ln13 = append(ln13, (N(int(gs[-gmp[e(43, 1) & 3] & 3])) * gn))
			fmt.Println(len(ln13), ln13[0], ln13[2], (e(44, 3) - v12))
			fmt.Println(gar == gar, (int(g8) * p))
		}
	}
	return (e(45, (p % (((p % (e(46, p) | 1)) << (uint(int((5 % ((p % (k0(e(47, p)) | 1)) | 1)))) & 7)) | 1))) + ((p + 6) + 6)), (((9 + e(48, 0)) + (0 + e(49, 1))) << (uint(int((p + e(50, 3)))) & 7))
}

func k2(p int) int {
	gsl = gsl[:4]
	gsl = append(gsl, ((7 << (uint(int((gas[int(gs[e(51, 3) & 3]) & 1] % (e(52, e(53, 3)) | 1)))) & 7)) % ((-8 + e(54, p)) | 1)))
	return -7
}

func main() {
	l0, l1, l2 := 3, 3, 3
	pl = &l1
	lp := &l2
	ls := []int{l0, 7, l1, 8}
	lsp := &ls[1]
	lb := []int{1, 2, 3}
	psb := &lb
	la := [4]int{l0, 1, l2, 3}
	pla = &la
	lap := &la
	las := la[1:3]
	lst := S{b: l1, a: l0}
	pls = &lst
	lfp := &lst.in.c
	lw := S{a: l2}
	lar := [2]int{l0, l1}
	lm := gml
	var mv int
	var mok bool
	gmp = map[int]int{l0 & 3: (e(55, 0) / (len(gs) | 1)), 3: *pick(56, pl), 1: 7}
	_ = ((lap[e(57, e(58, 3)) & 3] + (*lfp + 4)) % ((len(gmp) * l0) | 1))
	for v14, v := range lap {
		fmt.Println("v14", v, (g0 % (((e(59, 1) - l0) % (len((gs + gs)) | 1)) | 1)))
		fmt.Println("v14", v14)
		fmt.Println(((5 + *gpa) >> (1<<(int((l0 + e(60, -1)))&127+gz) - 2)), e(61, 0))
	}
	if ((l1 + l2) * e(62, 4)) == ((*pg * l0) % (both(e(63, 2), -l2) | 1)) {
		fmt.Println(las[5 & 1], int8(int((8 / (len(gs[(k0(gsl[int(g8) & 3]) / (((e(64, 0) - *pfa) * l0) | 1)) & 3:]) | 1)))), la[k1(l1) & 3])
		l1 ^= ((*lfp + int(gn)) * copy(gal, gsl[l0 & 1 : 2+l0&3]))
	} else if gb {
		println(k1(e(65, 4)), g8, -gas[-l0 & 1])
		switch e(66, int(gs[e(67, e(68, 4)) & 3])) {
		case (e(69, e(70, 3)) % ((lap[sum(71, gsl[:2+l2&3]) & 3] * (6 / (e(72, 4) | 1))) | 1)), ((l1 / (int(gs[((e(73, 1) - len(gmp)) * -3) & 3]) | 1)) - k2(*pg)):
			*pg = e(74, -2)
		case ((l0 + l2) % (int(gs[(l2 * l2) & 3]) | 1)), l2:
			*lp %= (e(75, 4) | 1)
		default:
			*pfa ^= len(gs)
		}
	} else if gb {
		fmt.Println(gss[l0 & 1] == gss[(int((N(l1) + gns[(0 - l1) & 3])) + e(76, -2)) & 1], (e(77, -2) + 2))
		l0, gal[(l1 / ((e(78, g0) % (e(79, -2) | 1)) | 1)) & 1] = l2, int(gs[l1 & 3])
	}
	for v15, r := range "\xe2\x82世" + gs[int(gs[(both(l2, e(80, 4)) + (e(81, 0) % (((l0 + 9) - k1(e(82, -1))) | 1))) & 3]) & 3:sum(83, gsl[-0 & 1 : 2+l1&3]) & 3+1] {
		fmt.Println("v15", v15, r)
		if e(84, 1) < v15 {
			pc = &v15
		}
		for v16 := 0; v16 < 3; v16++ {
			println(int(g8), e(85, 4), gs[(e(86, v15) * e(87, 8)) & 3])
			l0 = (4 + (e(88, e(89, -1)) / (l1 | 1)))
		}
		switch {
		case (e(90, -1) % (l2 | 1)) < (v15 >> (1<<(int(sum(91, gsl[:2+v15&3]))&127+gz) - 2)), !(e(92, 2) < *lsp):
			println(uint8(int((e(93, 4) % ((e(94, -1) - (v15 * v15)) | 1)))))
		case (e(95, 1) % ((-e(96, -2) * l1) | 1)) == l2, !(eb(97, e(98, -2) < l1)), gb:
			l2 |= gmp[(v15 % ((e(99, 2) * k2(e(100, 3))) | 1)) & 3]
		}
	}
	lw.arr[(8 - e(101, 4)) & 1], lar[1] = lar[1], lw.arr[(8 - e(101, 4)) & 1]
	println(gsl[both(4, e(102, 3)) & 3])
	fmt.Println(k0(len(gs)))
	_ = -e(103, 2)
	gn = gns[l1 & 3]
	v17 := 0
	for v17 < 3 && (e(104, 2) >> (1<<(int(l0)&127+gz) - 2)) < v17 {
		v17++
		gn = ((0 - gns[e(105, e(106, 2)) & 3]) - (N((*pl + l0)) / (-2 | 1)))
		gns[e(107, 2) & 3]++
	}
	l0 %= (k2((1 + v17)) | 1)
	gsl = append(gsl[:3:3], (-l1 * e(111, 4)), (-*psl * (v17 % (((gsl[(e(112, 0) / (k2(l0) | 1)) & 3] << (uint(int((1 % (e(113, -gst.a) | 1)))) & 7)) * e(115, e(116, 1))) | 1))))
	var v18, v19 = l2, (k0(gmp[(e(117, 0) + e(118, -1)) & 3]) + (l2 - l2))
	fmt.Println(v18, v19)
	fmt.Println(l0, l1, l2, g0, g1, gs, gb, g8, *pg, *lp)
	fmt.Println(gsl[0], gsl[1], gsl[2], gsl[3], gsl[4], gal[0], gal[1], len(gsl), *psl, ls[0], ls[1], ls[2], ls[3], *lsp)
	fmt.Println(len(lb), lb[0], lb[1], lb[2], len(*psb))
	fmt.Println(gar[0], gar[1], gar[2], gar[3], gas[0], gas[1], *gpa, la[0], la[1], la[2], la[3], las[0], las[1], lap[3], gar == la)
	fmt.Println(len(gmp), gmp[0], gmp[1], gmp[2], gmp[3], len(lm), lm[0], lm[1], lm[2], lm[3], mv, mok)
	for k := range 4 {
		fmt.Print(len(gms[k]), ":")
		for _, x := range gms[k] {
			fmt.Print(" ", x)
		}
		fmt.Println()
	}
	fmt.Println(gst, gss[0], gss[1], gsa[0], gsa[1], *pst, lst, *pfa, *lfp, gst == lst, lw, lar[0], lar[1])
	fmt.Println(gn, gns[0], gns[1], gns[2], gns[3], len(gns))
	if pc != nil {
		fmt.Println(*pc)
	}
}
