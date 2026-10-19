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
	if (v1 == 3) {
		L(1, G(100, G(101, 0, (PT(2).M)(v1))), v2, append(S(3), int(int64((*PI(4) + 5))))[2])
		w5 := append(S(6), (v0 + 1))[2]
		_ = w5
	} else if GB(102, v2) {
		for i7 := b1(MP(11)[m8((func(a, b int) int { return a*2 + b }(3, v3)))]); i7 < 3 && (L(8) > FN(9)[m8(1)](v1)); i7 += 1 + b1((Cell{A: 5, B: PT(10).GM(103, 3)}.B)) {
			w12 := S(13)[m8(FN(14)[m8(v2)](v3))]
			_ = w12
		}
		if int(int64(PT(15).GM(104, v3))) < (G(105, v1, v0)*3 - (map[int]int{m8(v3): v3, m8(v1) + 8: 0}[3])) {
			arr[m8(FN(16)[m8(FN(17)[m8((len(S(18)) + v3))](MP(19)[m8(L(20))]))]((G(106, 7, v3)*3 - G(107, v2, v2))))] += FN(21)[m8(L(22))](GG[int](108, GG[int](109, v2), int(int64(v2))))
		} else if (PT(23).M(v3) == PT(24).M(9)) {
			ch <- append(S(25), (PT(26).M)(G(110, v1, 0)))[2]
			ch <- min((PT(27).M)(1), FN(28)[m8(L(29))](2))
			arr[m8(Add2(Two(111, G(112, (v3*3 - 0), G(113, v2, v0)))))] = append(S(30), FN(31)[m8(G(114, 8, v2))](PT(32).GM(115, v0)))[2]
		}
	} else if S(33)[m8(([]int{v1, 5, v2}[1]))] < MP(34)[m8(Add2(Two(116, v2)))] {
		if ((map[int]int{m8(v0): 2, m8(v2) + 8: v0}[3]) < FN(35)[m8(L(36))](v1) || (8 < v1 || v0 < v2)) {
			v3 = FN(37)[m8(GEN[int]((PT(38).M)(v3)))](FN(39)[m8((v3*3 - v0))](v1))
			arr[m8(PT(40).GM(117, GEN[int](min(v1, v1))))] = (MP(41)[m8(L(42))] + int(Num((map[int]int{m8(4): v3, m8(8) + 8: 3}[3]))))
			L(43, S(44)[m8(L(45))], int(Num(append(S(46), L(47))[2])))
		} else if (*PI(48) + FN(49)[m8(v0)](v2)) < ([]int{append(S(50), v3)[2], (A(51).(int) - v3), L(52, 7)}[1]) {
			v0 = (Cell{A: (func(a, b int) int { return a*2 + b }(v0, v1)), B: min(PT(53).M(v3), (0 + 5))}.B)
		} else if FN(54)[m8(min(v3, v0))](min(v2, v2)) < (map[int]int{m8((map[int]int{m8(v3): v1, m8(v1) + 8: 4}[3])): (len(S(55)) + 1), m8(FN(56)[m8(v1)](v0)) + 8: v1}[3]) {
			v0 = v3
		} else {
			v1 = append(S(57), (<-CH(58) + v3))[2]
			L(59, int(Num(min(v2, v0))), (A(60).(int) - v2), L(61))
			v3, v0 = int(Num((v0*3 - 7))), v2
		}
	}
	arr[m8((A(62).(int) - G(118)))] = v2
	if (!(v2 < v2) || (L(63) > v0)) {
		PT(64).X = (PT(65).M)(GG[int](119, (*PI(66) + v3)))
		L(67)
	} else if ((L(68) > v0) || GB(120, v1)) {
		PT(69).X = 0
		if GB(121, (<-CH(70) + 6)) {
			w71 := (len(S(72)) + Add2(Two(122, v0)))
			_ = w71
			L(73, PT(74).M(PT(75).M((<-CH(76) + v1))))
			v2 = (<-CH(77) + int(Num(FN(78)[m8(6)](v0))))
		}
		v1 = v3
	}
	v3, v0 = (map[int]int{m8(GEN[int](int(int64(v0)))): G(123, FN(79)[m8(L(80))](v0), (Cell{A: v2, B: 9}.B)), m8(int(Num(Add2(Two(124, v1))))) + 8: ((A(81).(int) - 4) + (A(82).(int) - 0))}[3]), ((map[int]int{m8((map[int]int{m8(v0): v1, m8(v0) + 8: 5}[3])): min(v2, v0), m8(GEN[int](5)) + 8: PT(83).M(v0)}[3])*3 - (len(S(84)) + S(85)[m8(L(86))]))
	for i87 := b1((A(89).(int) - ([]int{v0, v3, v0}[1]))); i87 < 3; i87 += 1 + b1(((<-CH(88) + v1) + GEN[int](v1))) {
		if v0 < MP(90)[m8(FN(91)[m8(L(92))](v2))] {
			v1 = (G(125, v1)*3 - ([]int{v0, v1, v0}[1]))
		} else if GB(126, min(5, 2)) {
			v3 = int(int64(min(v0, i87)))
			v0 = (func(a, b int) int { return a*2 + b }(append(S(93), i87)[2], (len(S(94)) + v2)))
		} else if 6 < v2 {
			v0 = G(127, (*PI(95) + (Cell{A: i87, B: v1}.B)), (<-CH(96) + append(S(97), v2)[2]))
			arr[m8((A(98).(int) - (Cell{A: 2, B: v2}.B)))] += (Cell{A: (Cell{A: v3, B: v3}.B), B: L(99, i87)}.B)
			w100 := v1
			_ = w100
		} else {
			L(101, Add2(Two(128, (Cell{A: L(102, v2, v0, v1), B: GEN[int](9)}.B))))
			arr[m8(int(Num(MP(103)[m8(i87)])))] += Add2(Two(129, append(S(104), v1)[2]))
			ch <- FN(105)[m8(L(106))](G(130, 3))
		}
	}
	arr[m8(FN(107)[m8(L(108))](FN(109)[m8(PT(110).M(v2))]((map[int]int{m8(v2): v2, m8(2) + 8: 9}[3]))))] += int(int64((v0*3 - PT(111).GM(131, v2))))
	v2 = PT(112).GM(132, FN(113)[m8(v0)](v2))
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129, 130, 131, 132} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
