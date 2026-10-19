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
	switch append(S(1), G(100, 1, 3))[2] {
	case m8((<-CH(2) + Add2(Two(101, v0)))):
		v0, v1 = G(102, (func(a, b int) int { return a*2 + b }((v0 + 4), (v1*3 - v2))), (func(a, b int) int { return a*2 + b }(L(3, v1), G(103, v0, v1))), (len(S(4)) + Add2(Two(104, v1)))), v1
		L(5, FN(6)[m8(v3)](L(7)), PT(8).GM(105, min(7, v3)), v0)
	case m8(G(106, 2, ([]int{v2, v1, v3}[1]))):
		L(9, PT(10).GM(107, G(108, v1, 5)), (func(a, b int) int { return a*2 + b }((v1*3 - v3), (map[int]int{m8(2): 5, m8(v0) + 8: 2}[3]))), FN(11)[m8(L(12))](int(Num(v1))))
		L(13, G(109, (PT(14).M)(v2), MP(15)[m8(L(16))], (*PI(17) + v1)), (Cell{A: FN(18)[m8(L(19))](1), B: int(int64(8))}.B))
	default:
		PT(20).X = G(110, GG[int](111, PT(21).GM(112, 3), (4*3 - v3)))
		switch append(S(22), v2)[2] {
		case m8(3):
			arr[m8(append(S(23), 5)[2])] = G(113, G(114, int(int64(v0))), (*PI(24) + (PT(25).M)(v1)))
			v3 += FN(26)[m8(L(27))]((v0 + ([]int{v0, v1, v3}[1])))
			v3 = v2
		case m8(PT(28).M(v2)):
			arr[m8((len(S(29)) + (v1 + v1)))] += int(Num(FN(30)[m8(7)](v1)))
			v1, v2 = G(115, (func(a, b int) int { return a*2 + b }((map[int]int{m8(v3): v1, m8(6) + 8: 8}[3]), ([]int{v1, 6, v0}[1]))), v2), int(Num(int(Num(v0))))
			v1 += (len(S(31)) + ([]int{v2, v3, 0}[1]))
		default:
			w32 := G(116)
			_ = w32
		}
		arr[m8(FN(33)[m8(PT(34).M(v1))](FN(35)[m8(v0)](v1)))] = (v3 + S(36)[m8(v1)])
	}
	switch int(Num(v1)) {
	case m8(S(37)[m8(min(4, 5))]):
		PT(38).X = (FN(39)[m8(L(40))](v0)*3 - G(117, v1))
	case m8(FN(41)[m8(L(42))](G(118, 3, 8))):
		v0, v1 = (v2*3 - FN(43)[m8(L(44))](append(S(45), v0)[2])), GG[int](119, ((len(S(46)) + v1) + PT(47).M(1)), PT(48).M(L(49)))
		for i50 := b1(G(121, ([]int{v0, v0, v2}[1]))); i50 < 3 && !(!(9 < v1)); i50 += 1 + b1(G(120, FN(51)[m8(1)](i50), v2)) {
			arr[m8((<-CH(52) + (func(a, b int) int { return a*2 + b }(MP(53)[m8(7)], append(S(54), v3)[2]))))] += (len(S(55)) + append(S(56), (*PI(57) + i50))[2])
		}
		switch G(122, (Cell{A: v1, B: v2}.B), G(123)) {
		case m8(int(int64(G(124, v2, v3)))):
			arr[m8(append(S(58), G(125, (PT(59).M)(7), (len(S(60)) + 1)))[2])] = PT(61).M(min((*PI(62) + v2), G(126, 2, v3)))
		case m8(GEN[int]((A(63).(int) - v2))):
			arr[m8(v2)] = PT(64).GM(127, int(int64(v2)))
			L(65, G(128, FN(66)[m8(L(67))](FN(68)[m8(L(69))](v3))), (*PI(70) + v3))
		default:
			v1 = (Cell{A: (v1 + v2), B: 5}.B)
			L(71, Add2(Two(129, PT(72).GM(130, 8))), (func(a, b int) int { return a*2 + b }(GG[int](131), (8 + v0))))
			w73 := G(132, v1, PT(74).M((PT(75).M)(5)), PT(76).M(G(133, v0, v1)))
			_ = w73
		}
	default:
		if G(134, (v0 + 3), G(135, 7, v3), v0) < v0 {
			arr[m8(G(136))] += v1
			w77 := L(78, (v2*3 - v1), int(Num(9)), G(137, 1))
			_ = w77
			arr[m8(1)] += ([]int{v1, 0, (*PI(79) + MP(80)[m8(v0)])}[1])
		} else if !(GB(138, 3)) {
			v1 = FN(81)[m8(L(82))](S(83)[m8(L(84))])
			L(85, (func(a, b int) int { return a*2 + b }(PT(86).M(GEN[int](4)), ((*PI(87) + 6) + (<-CH(88) + 0)))))
		} else {
			v2 += G(139, (len(S(89)) + (map[int]int{m8(8): v0, m8(v2) + 8: v1}[3])), min((*PI(90) + v3), G(140)))
		}
		v0 += L(91, (len(S(92)) + 1), (len(S(93)) + v1), GEN[int](v2))
		if (A(94).(int) - 3) < int(int64((len(S(95)) + v3))) {
			v0 += (*PI(96) + int(Num(8)))
			ch <- (PT(97).M)(L(98))
		} else if (func(a, b int) int { return a*2 + b }((Cell{A: v3, B: v1}.B), (5 + 8))) < FN(99)[m8(L(100))](G(141, v1, 0)) {
			v0 += Add2(Two(142, FN(101)[m8(L(102))](S(103)[m8(v3)])))
			w104 := (A(105).(int) - (<-CH(106) + GG[int](143, v3, v0)))
			_ = w104
			arr[m8(PT(107).M(FN(108)[m8(L(109))](int(Num(8)))))] += (PT(110).M)((A(111).(int) - int(int64(6))))
		} else {
			arr[m8(([]int{L(112, v2, v0), min(v3, v1), L(113, v3)}[1]))] += 1
		}
	}
	L(114, L(115, 1), min(GG[int](144), (G(145)*3 - PT(116).GM(146, v2))), FN(117)[m8(L(118))](FN(119)[m8(L(120))](v3)))
	arr[m8(int(int64(min(v3, v2))))]++
	arr[m8(GEN[int](FN(121)[m8(6)](5)))] += (len(S(122)) + S(123)[m8(L(124))])
	arr[m8((A(125).(int) - ([]int{5, v2, v2}[1])))] = (Cell{A: int(Num(v3)), B: (<-CH(126) + 0)}.B)
	v0 += Add2(Two(147, ((<-CH(127) + 3)*3 - min(v1, 3))))
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
