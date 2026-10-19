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
	if (FN(1)[m8(L(2))](v2) < FN(3)[m8(v3)](v1) || (v0 < v2 && v1 < v1)) {
		return Add2(Two(100, FN(4)[m8(L(5))](v2))), nil
	}
	if (append(S(6), v0)[2] == 6) {
		return (A(7).(int) - (*PI(8) + v3)), nil
	}
	if GB(101, FN(9)[m8(L(10))](7)) {
		arr[m8(v2)] = FN(11)[m8(G(102))](PT(12).M((PT(13).M)(9)))
		ch <- int(Num(G(103, G(104, 4, 0, v1), v0, G(105, v0))))
		if !(GB(106, v1)) {
			L(14, int(Num((Cell{A: (map[int]int{m8(0): 2, m8(v1) + 8: v0}[3]), B: 2}.B))), L(15), L(16, (len(S(17)) + (<-CH(18) + 4)), append(S(19), int(int64(3)))[2]))
			w20 := (map[int]int{m8(G(107, Add2(Two(108, v3)), L(21, v1, v2, v3), (Cell{A: 1, B: 9}.B))): (len(S(22)) + (*PI(23) + v0)), m8(FN(24)[m8(PT(25).M(0))](v0)) + 8: (Add2(Two(109, v1))*3 - (func(a, b int) int { return a*2 + b }(1, v2)))}[3])
			_ = w20
			v0, v1 = G(110, (int(int64(v3)) + (PT(26).M)(w20)), GEN[int]((v2 + v3))), ([]int{GG[int](111), v2, G(112, FN(27)[m8(L(28))](w20), ([]int{v0, v3, 9}[1]))}[1])
		} else if ((8*3 - 9) == PT(29).M(4)) {
			ch <- FN(30)[m8(v2)](GG[int](113, (PT(31).M)(v1), PT(32).GM(114, v2)))
			arr[m8(Add2(Two(115, (map[int]int{m8(PT(33).GM(116, 3)): PT(34).M(v0), m8(G(117, 6, v0)) + 8: L(35, 6)}[3]))))] = (A(36).(int) - FN(37)[m8(([]int{v3, v1, v1}[1]))](GEN[int](v2)))
		} else if G(118, v1, Add2(Two(119, v0))) < (map[int]int{m8(append(S(38), 7)[2]): v3, m8(GG[int](120)) + 8: G(121, v3, v1)}[3]) {
			arr[m8(G(122))] += (<-CH(39) + (PT(40).M)(PT(41).M(v3)))
			ch <- (A(42).(int) - 0)
		}
	} else if (FN(43)[m8(v2)](v2) < PT(44).M(v1) && GB(123, 5)) {
		PT(45).X = FN(46)[m8((A(47).(int) - (PT(48).M)(v3)))]((PT(49).M)(([]int{9, v0, v3}[1])))
		if S(50)[m8(L(51))] < Add2(Two(124, int(int64(v3)))) {
			v3 += (Cell{A: (<-CH(52) + v2), B: int(Num(v2))}.B)
			arr[m8(G(125, (v3 + 1), 3))] += (int(int64(v0)) + (map[int]int{m8(v1): 0, m8(v1) + 8: v1}[3]))
		} else if (v3 == int(int64(v2))) {
			arr[m8(v0)] = PT(53).M(FN(54)[m8(L(55))](v1))
			L(56)
			v0 += MP(57)[m8(L(58))]
		}
		L(59, min((Cell{A: v1, B: 4}.B), L(60)), (<-CH(61) + (*PI(62) + 6)), ([]int{G(126), v3, min(v2, 1)}[1]))
	} else {
		v1 = PT(63).GM(127, (Cell{A: 5, B: v2}.B))
		arr[m8(GEN[int]((len(S(64)) + G(128, v0, v1))))]++
	}
	arr[m8(int(int64(MP(65)[m8(L(66))])))]++
	arr[m8(([]int{G(129, 9, v1), FN(67)[m8(v0)](6), PT(68).GM(130, 2)}[1]))]++
	PT(69).X = 9
	switch G(131, FN(70)[m8(v3)](v1), G(132, v3, 1), (Cell{A: v3, B: 9}.B)) {
	case m8(min(MP(71)[m8(L(72))], FN(73)[m8(v2)](v1))):
		for i74 := b1(Add2(Two(136, FN(76)[m8(L(77))](v0)))); i74 < 3 && !(GB(133, v2)); i74 += 1 + b1((map[int]int{m8(Add2(Two(134, 4))): (*PI(75) + i74), m8(G(135, i74)) + 8: min(3, v1)}[3])) {
			v3 += append(S(78), G(137, 7, v2))[2]
			w79 := (Cell{A: (PT(80).M)(v0), B: int(int64(1))}.B)
			_ = w79
		}
		if (L(81) > 6) {
			arr[m8(int(int64(PT(82).M(FN(83)[m8(L(84))](0)))))] = S(85)[m8((Cell{A: int(Num(v3)), B: FN(86)[m8(L(87))](v1)}.B))]
			v1 = (PT(88).M)((Cell{A: v3, B: v1}.B))
		}
		v3 = ((<-CH(89) + 6)*3 - L(90, v2, v3, v0))
	case m8(v1):
		if FN(91)[m8((Cell{A: 3, B: v0}.B))](GEN[int](v1)) < FN(92)[m8(FN(93)[m8(L(94))](v1))](G(138, v3, 3)) {
			v1 = G(139, (Cell{A: v2, B: 2}.B), GEN[int](1))
		} else if ((6 + v3) == FN(95)[m8(L(96))](v1)) {
			v3 = G(140, S(97)[m8(1)], (len(S(98)) + v2))
			arr[m8(append(S(99), FN(100)[m8(3)](v3))[2])] = (func(a, b int) int { return a*2 + b }(int(int64(9)), L(101, 0, v1)))
		} else if (L(102) > GEN[int](v0)) {
			w103 := FN(104)[m8(L(105))](G(141, v0, 5))
			_ = w103
		} else {
			arr[m8(S(106)[m8((<-CH(107) + v2))])] += (map[int]int{m8(FN(108)[m8(L(109))](v3)): G(142, 6), m8((*PI(110) + v0)) + 8: PT(111).M(v1)}[3])
			w112 := (map[int]int{m8(G(143, PT(113).GM(144, v2), int(Num(v0)))): GEN[int](append(S(114), v0)[2]), m8(PT(115).M((PT(116).M)(v3))) + 8: v0}[3])
			_ = w112
		}
		v0 = FN(117)[m8(L(118))]((<-CH(119) + int(Num(v1))))
	default:
		if G(145, PT(120).GM(146, v0), G(147, v3, 2), (<-CH(121) + v0)) < v3 {
			arr[m8(v1)] = (func(a, b int) int { return a*2 + b }(G(148, GEN[int](1)), min(min(3, v1), (A(122).(int) - v3))))
			arr[m8(PT(123).M(MP(124)[m8(L(125))]))] += (<-CH(126) + (A(127).(int) - min(v2, 7)))
		}
		for i128 := b1(v1); i128 < 3 && (GB(149, v3) && GB(150, v2)); i128 += 1 + b1((*PI(129) + int(Num(v2)))) {
			arr[m8(G(151, min(v0, v0), G(152, v3, 4)))] += L(130, MP(131)[m8(v3)])
			v0 = append(S(132), (A(133).(int) - i128))[2]
			L(134, (func(a, b int) int { return a*2 + b }(append(S(135), S(136)[m8(1)])[2], PT(137).M((*PI(138) + 5)))))
		}
		if (L(139) > (len(S(140)) + 0)) {
			v2 += G(153, Add2(Two(154, Add2(Two(155, v1)))), min(2, v1))
		} else if (GEN[int](2) == int(int64(7))) {
			ch <- PT(141).GM(156, v0)
			w142 := min(PT(143).GM(157, (v2*3 - 6)), (A(144).(int) - (Cell{A: 7, B: v1}.B)))
			_ = w142
		} else if GB(158, (*PI(145) + v0)) {
			v1 = PT(146).GM(159, G(160, (<-CH(147) + v1), PT(148).GM(161, 2), int(int64(v1))))
			arr[m8((*PI(149) + (map[int]int{m8((v0 + v2)): S(150)[m8(L(151))], m8((*PI(152) + v0)) + 8: 1}[3])))] = FN(153)[m8(GEN[int](v0))](PT(154).M((PT(155).M)(5)))
			arr[m8(Add2(Two(162, v3)))] = G(163, GEN[int](4), (*PI(156) + 3), G(164, v2))
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136, 137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 149, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
