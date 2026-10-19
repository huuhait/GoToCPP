package main

import (
	"errors"
	"fmt"
)

var failAt = -1

type Pt struct {
	id int
	X  int
}

type Cell struct{ A, B int }

type Num int

func sum(id int, args []int) int {
	s := id * 3
	for _, a := range args {
		s += a
	}
	return s % 1000
}

func m8(x int) int { return (x%8 + 8) % 8 }

func b1(x int) int { return x & 1 }

func L(id int, args ...int) int {
	fmt.Println("L", id)
	return sum(id, args)
}

func S(id int) []int {
	fmt.Println("S", id)
	return []int{10, 11, 12, 13, 14, 15, 16, 17}
}

func FN(id int) []func(int) int {
	fmt.Println("FN", id)
	var fs []func(int) int
	for i := 0; i < 8; i++ {
		fs = append(fs, func(x int) int { return x + i })
	}
	return fs
}

func MP(id int) map[int]int {
	fmt.Println("MP", id)
	m := map[int]int{}
	for i := 0; i < 8; i++ {
		m[i] = i * i
	}
	return m
}

func PT(id int) *Pt {
	fmt.Println("PT", id)
	return &Pt{id: id, X: id + 1}
}

func (p *Pt) M(a int) int {
	fmt.Println("M", p.id)
	return p.id + a
}

func PI(id int) *int {
	fmt.Println("PI", id)
	v := id * 2
	return &v
}

func A(id int) any {
	fmt.Println("A", id)
	return id
}

func CH(id int) chan int {
	fmt.Println("CH", id)
	c := make(chan int, 1)
	c <- id + 100
	return c
}

func GEN[T int | int64](a T) T {
	fmt.Println("GEN", a)
	return a + 1
}

func Add2(a, b int) int { return a*10 + b }

func fail(id int) error {
	return fmt.Errorf("fail %d: %w", id, ErrBad)
}

var ErrBad = errors.New("bad")

func run() (res int, err error) {
	defer func() {
		if r := recover(); r != nil {
			f, ok := r.(mustFail)
			if !ok {
				panic(r)
			}
			res, err = 0, f.err
		}
	}()
	v0, v1, v2, v3 := 1, 2, 3, 4
	var arr [8]int
	ch := make(chan int, 64)
	_ = ch
	v2 = FN(1)[m8(L(2))]((A(3).(int) - 9))
	PT(4).X = (*PI(5) + S(6)[m8(L(7))])
	if ((PT(8).M)(v2) < (len(S(9)) + v0) || (v0 < v1 && v2 < 2)) {
		arr[m8(GEN[int](G(100, 3, v1)))]++
		w10 := int(int64((map[int]int{m8((Cell{A: v0, B: v3}.B)): G(101, v1, 2), m8(FN(11)[m8(L(12))](v1)) + 8: Add2(Two(102, v2))}[3])))
		_ = w10
	} else if (2 == 0) {
		w13 := (*PI(14) + PT(15).M(G(103, v3, v3)))
		_ = w13
		arr[m8((v2 + MP(16)[m8(L(17))]))] += G(104, append(S(18), (A(19).(int) - 6))[2])
		if (L(20) > GEN[int](w13)) {
			v3 += S(21)[m8(L(22))]
		} else if (L(23) > v3) {
			ch <- FN(24)[m8(FN(25)[m8(v3)](v0))]((*PI(26) + w13))
			L(27, append(S(28), G(105))[2])
		}
	} else {
		arr[m8(FN(29)[m8(L(30))](G(106, v2, v0)))] += ([]int{GG[int](107), v0, (A(31).(int) - 9)}[1])
	}
	switch int(int64(L(32))) {
	case m8(PT(33).M(([]int{v0, 3, 9}[1]))):
		PT(34).X = (Cell{A: MP(35)[m8(L(36))], B: GEN[int](G(108, 0, v2))}.B)
		switch v3 {
		case m8(L(37)):
			L(38, (map[int]int{m8(Add2(Two(109, G(110, v3)))): FN(39)[m8(PT(40).M(1))](L(41, v3, v3)), m8((func(a, b int) int { return a*2 + b }(int(int64(v1)), v3))) + 8: (map[int]int{m8(([]int{v1, v0, v2}[1])): (len(S(42)) + 8), m8((v0*3 - 8)) + 8: 3}[3])}[3]), ([]int{(*PI(43) + Add2(Two(111, 6))), L(44, FN(45)[m8(v1)](v0), (map[int]int{m8(2): v0, m8(v2) + 8: v3}[3]), min(v2, v3)), int(int64(L(46)))}[1]), GG[int](112, int(Num(PT(47).M(v2))), G(113, (v3 + 7), int(Num(0)))))
			v3 = FN(48)[m8((PT(49).M)(v1))]((PT(50).M)(v0))
			w51 := int(int64(FN(52)[m8(L(53))](v2)))
			_ = w51
		case m8(FN(54)[m8(L(55))](FN(56)[m8(L(57))](v2))):
			w58 := (len(S(59)) + (PT(60).M)(v0))
			_ = w58
			arr[m8((map[int]int{m8(MP(61)[m8(L(62))]): (map[int]int{m8(G(114, v0, v0)): (*PI(63) + v0), m8(min(3, v2)) + 8: append(S(64), v0)[2]}[3]), m8((<-CH(65) + (map[int]int{m8(w58): 4, m8(6) + 8: w58}[3]))) + 8: (G(115, v1, 7)*3 - v3)}[3]))] += G(116, G(117, ([]int{v2, 1, w58}[1]), G(118, 7, v1), append(S(66), v0)[2]), 6)
		default:
			v0 = MP(67)[m8(S(68)[m8(v0)])]
		}
	case m8(G(119, (A(69).(int) - v2), min(v2, v3))):
		switch PT(70).M(([]int{4, 5, v2}[1])) {
		case m8(int(int64(GEN[int](1)))):
			v3 = G(120)
			ch <- (len(S(71)) + GEN[int](v0))
			arr[m8(5)] += int(int64((*PI(72) + v0)))
		case m8((PT(73).M)(v2)):
			v2, v3 = FN(74)[m8(L(75))]((<-CH(76) + FN(77)[m8(L(78))](v2))), (func(a, b int) int { return a*2 + b }(v2, PT(79).GM(121, (A(80).(int) - v3))))
			v2 += GG[int](122, (A(81).(int) - (8*3 - v2)), G(123, append(S(82), 2)[2], ([]int{v0, v0, v3}[1])))
			arr[m8(MP(83)[m8((v0 + v3))])] += S(84)[m8(G(124, v0))]
		default:
			v0 = G(125, PT(85).GM(126, v0), GEN[int](8))
			v1 = (map[int]int{m8(([]int{L(86), min(v1, 2), GEN[int](v1)}[1])): PT(87).GM(127, PT(88).M(0)), m8(G(128)) + 8: G(129, min(v2, v3), int(Num(v0)))}[3])
			v3 = GEN[int](G(130, 8, v1, v1))
		}
		arr[m8(((Cell{A: v3, B: v2}.B)*3 - append(S(89), 2)[2]))] += G(131, 0, (len(S(90)) + v3))
	default:
		L(91, v0, MP(92)[m8(L(93))], ((v2*3 - v0)*3 - (len(S(94)) + 6)))
	}
	PT(95).X = GEN[int](int(Num((map[int]int{m8(1): v3, m8(v3) + 8: 9}[3]))))
	v3 += GEN[int]((append(S(96), v0)[2] + GG[int](132, v3)))
	v0 = v3
	for i97 := b1(int(int64(G(133, v0, v0)))); i97 < 3 && (GEN[int](i97)*3 - i97) < ((2*3 - v2)*3 - S(98)[m8(L(99))]); i97 += 1 + b1(int(Num(v3))) {
		v3, v0 = G(134, v1), 0
		v3 = (GG[int](135, i97)*3 - (v1 + v0))
		arr[m8(PT(100).GM(136, S(101)[m8(L(102))]))]++
	}
	if (G(137, 9, v0) == G(138, v2, v2)) {
		v1, v2 = FN(103)[m8(L(104))](MP(105)[m8(L(106))]), append(S(107), FN(108)[m8(L(109))](8))[2]
		if (L(110) > v3) {
			return (map[int]int{m8(GEN[int](v2)): (len(S(111)) + v2), m8((<-CH(112) + v0)) + 8: (Cell{A: 1, B: v3}.B)}[3]), nil
		}
	} else if (L(113) > G(139, 7, 0)) {
		w114 := v3
		_ = w114
	}
	return (v0*7+v1*5+v2*3+v3+arr[0]+arr[3]+len(ch)) % 100003, nil
}

type mustFail struct{ err error }

func G(id int, args ...int) int {
	fmt.Println("G", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return sum(id, args)
}

func GB(id int, x int) bool {
	fmt.Println("GB", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return (sum(id, nil)+x)%3 == 0
}

func (p *Pt) GM(id int, a int) int {
	fmt.Println("GM", id, p.id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return p.id + a + id
}

func GG[T int | int64](id int, args ...T) T {
	fmt.Println("GG", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	var s T
	for _, a := range args {
		s += a
	}
	return s + T(id)
}

func Two(id int, a int) (int, int) {
	fmt.Println("Two", id)
	if id == failAt {
		panic(mustFail{fail(id)})
	}
	return a, id
}

func main() {
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
