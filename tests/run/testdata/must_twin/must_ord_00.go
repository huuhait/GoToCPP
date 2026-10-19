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
	if !((v2 < 6 || 4 < v0)) {
		if !(FN(1)[m8(v3)](v0) < (v3*3 - v3)) {
			return G(100, PT(2).GM(101, v3), (A(3).(int) - 1), MP(4)[m8(7)]), nil
		}
		arr[m8(append(S(5), (map[int]int{m8(v3): v0, m8(v3) + 8: v3}[3]))[2])] += FN(6)[m8((map[int]int{m8(v0): v0, m8(v0) + 8: 1}[3]))](S(7)[m8(L(8))])
	} else if GG[int](102, int(Num(v3)), G(103, 7, 2)) < Add2(Two(104, G(105, v3))) {
		for i9 := b1((map[int]int{m8((len(S(13)) + v1)): (Cell{A: v0, B: v2}.B), m8(FN(14)[m8(1)](9)) + 8: (PT(15).M)(v0)}[3])); i9 < 3 && !(G(106, 2, v0) < (PT(10).M)(8)); i9 += 1 + b1(GEN[int](FN(11)[m8(L(12))](v1))) {
			v3, v0 = FN(16)[m8(L(17))](int(Num(v2))), (PT(18).GM(107, v2) + G(108, 0, 8, 9))
		}
	} else {
		arr[m8(Add2(Two(109, int(int64(v3)))))]++
		v2 = Add2(Two(110, G(111, v1, v1)))
	}
	for i19 := b1((Cell{A: L(20, v3, v2), B: FN(21)[m8(L(22))](v3)}.B)); i19 < 3 && ((7 < i19 && v0 < v1) && GB(112, v2)); i19 += 1 + b1(GEN[int]((v3*3 - v1))) {
		if (L(23) > G(113, v0, v2)) {
			v1 = (PT(24).M)((A(25).(int) - FN(26)[m8(L(27))](i19)))
			ch <- (*PI(28) + (<-CH(29) + FN(30)[m8(L(31))](v2)))
		} else if ((1 < v2 && 5 < v1) || (0 < v2 || v0 < v1)) {
			v0, v1 = L(32, (v2 + v2), i19), int(Num((2 + i19)))
			arr[m8((func(a, b int) int { return a*2 + b }(S(33)[m8(L(34))], (PT(35).M)((A(36).(int) - v3)))))] += GEN[int](Add2(Two(114, v1)))
			w37 := ([]int{(*PI(38) + PT(39).M(1)), (map[int]int{m8(G(115, 9)): FN(40)[m8(L(41))](v2), m8(S(42)[m8(L(43))]) + 8: PT(44).M(v1)}[3]), (len(S(45)) + G(116, v1, v2))}[1])
			_ = w37
		} else {
			v2, v3 = (*PI(46) + (func(a, b int) int { return a*2 + b }(G(117, 8), S(47)[m8(v1)]))), (<-CH(48) + v1)
		}
		v0 = G(118, GG[int](119))
	}
	L(49)
	arr[m8(v0)] = (v3 + G(120, v3, v1, 3))
	v0 += v3
	PT(50).X = L(51)
	v0 = (v0 + min(7, v2))
	switch int(int64((<-CH(52) + 2))) {
	case m8((map[int]int{m8(v3): FN(53)[m8(9)](v3), m8((map[int]int{m8(6): v2, m8(v1) + 8: v1}[3])) + 8: int(Num(v3))}[3])):
		ch <- (A(54).(int) - (map[int]int{m8(5): v3, m8(v2) + 8: v1}[3]))
	case m8(FN(55)[m8(L(56))](PT(57).M(v2))):
		ch <- (FN(58)[m8(L(59))](3)*3 - L(60, 7))
		ch <- v1
		switch Add2(Two(121, GG[int](122, v3))) {
		case m8(int(int64(9))):
			ch <- int(Num(Add2(Two(123, v2))))
		case m8((PT(61).M)(G(124, v0, v1))):
			v3 = append(S(62), v0)[2]
		default:
			v1 = (PT(63).M)(v3)
		}
	default:
		if (L(64) > (PT(65).M)(v0)) {
			v2 = S(66)[m8(L(67))]
			ch <- 7
			arr[m8(FN(68)[m8(L(69))]((1 + v0)))] += FN(70)[m8(L(71))](append(S(72), 9)[2])
		} else if (G(125, v3, v0) < FN(73)[m8(v2)](v2) && (v0 < v0 && 0 < v3)) {
			arr[m8(FN(74)[m8(Add2(Two(126, v1)))]((<-CH(75) + v2)))] = Add2(Two(127, (v3*3 - 7)))
			L(76, append(S(77), v3)[2], int(int64((<-CH(78) + v0))))
			arr[m8(MP(79)[m8((map[int]int{m8(6): v1, m8(4) + 8: 4}[3]))])] += G(128, 3, (func(a, b int) int { return a*2 + b }(v0, v3)))
		} else if ((9 < v2 || v1 < 8) && (v2 == 4)) {
			v3 += GEN[int](PT(80).M((map[int]int{m8(2): 2, m8(6) + 8: 3}[3])))
		}
		if ((4*3 - v3) == FN(81)[m8(L(82))](v2)) {
			ch <- ([]int{([]int{S(83)[m8(L(84))], FN(85)[m8(L(86))](7), (*PI(87) + v0)}[1]), (map[int]int{m8(int(int64(4))): (Cell{A: v1, B: v0}.B), m8(int(Num(v2))) + 8: (Cell{A: 0, B: v1}.B)}[3]), ([]int{append(S(88), v0)[2], G(129, v0, v2, 3), Add2(Two(130, v3))}[1])}[1])
			arr[m8(2)] = FN(89)[m8(L(90))](int(int64(v1)))
		} else {
			w91 := FN(92)[m8(L(93))](Add2(Two(131, GG[int](132, v0))))
			_ = w91
			v2 = FN(94)[m8(L(95))](L(96))
		}
		switch FN(97)[m8(FN(98)[m8(v2)](3))](PT(99).GM(133, 5)) {
		case m8(S(100)[m8((v2 + 5))]):
			L(101, G(134, int(Num(7)), (func(a, b int) int { return a*2 + b }(0, FN(102)[m8(v3)](1)))), S(103)[m8(L(104))])
		case m8((*PI(105) + FN(106)[m8(v2)](v0))):
			w107 := (PT(108).M)((*PI(109) + v1))
			_ = w107
		default:
			arr[m8(G(135, GG[int](136, FN(110)[m8(L(111))](2), FN(112)[m8(L(113))](v1)), v0))] = int(int64(FN(114)[m8(int(int64(v2)))](G(137, v2, v3))))
			w115 := G(138, GG[int](139, 1, 8), G(140), S(116)[m8(v2)])
			_ = w115
		}
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
