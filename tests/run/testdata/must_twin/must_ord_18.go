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
	v2 += ([]int{(map[int]int{m8(v0): v0, m8(v3) + 8: 4}[3]), ([]int{v3, v1, 8}[1]), L(1, v2)}[1])
	if (v1 == (v0*3 - v1)) {
		arr[m8((PT(2).M)(PT(3).GM(100, (v2*3 - 9))))] = G(101, G(102, (map[int]int{m8(v2): v3, m8(4) + 8: v3}[3])), append(S(4), v2)[2])
	}
	v1 += (len(S(5)) + G(103, v3, v3))
	v0 = 9
	ch <- int(Num(v1))
	L(6, GG[int](104))
	switch (*PI(7) + S(8)[m8(v2)]) {
	case m8(min((<-CH(9) + v2), G(105))):
		v0, v1 = (PT(10).M)(L(11)), (<-CH(12) + PT(13).GM(106, 0))
		arr[m8(int(int64(PT(14).GM(107, 2))))] = Add2(Two(108, PT(15).M(4)))
	case m8(G(109, (6*3 - v3), 5)):
		arr[m8((v3 + (S(16)[m8(v0)]*3 - G(110, v2, 9))))] += L(17, 5, (*PI(18) + (PT(19).M)(v0)), GG[int](111, min(1, 6), PT(20).GM(112, 1)))
		L(21)
	default:
		if (*PI(22) + int(int64(v2))) < G(113) {
			v1 += min(v3, GEN[int](v3))
		} else if ((*PI(23) + 9) == (PT(24).M)(8)) {
			v2 += int(Num(int(Num(v0))))
			ch <- FN(25)[m8(L(26))](v3)
			w27 := (A(28).(int) - v1)
			_ = w27
		} else if ((PT(29).M)(v2) == GG[int](114, v0)) {
			v2, v3 = MP(30)[m8(L(31))], G(115)
			arr[m8(FN(32)[m8(G(116, (map[int]int{m8(0): 2, m8(v2) + 8: v0}[3]), MP(33)[m8(9)]))]((*PI(34) + GG[int](117, 0, 1))))] = G(118, (<-CH(35) + L(36)), G(119, Add2(Two(120, 5))))
			arr[m8(G(121, G(122, 6, 3), MP(37)[m8(L(38))]))] = MP(39)[m8(int(int64(v2)))]
		} else {
			ch <- (G(123, G(124, v1, v2), FN(40)[m8(L(41))](v1)) + v0)
			v3 = int(Num((PT(42).M)(2)))
		}
		if !(!(v1 < v0)) {
			arr[m8(v1)] += v1
		} else if (GEN[int](v1) < (Cell{A: 1, B: v0}.B) || (v2 < 7 && v1 < v2)) {
			arr[m8((PT(43).M)((<-CH(44) + v3)))] += PT(45).M(v3)
			v2, v3 = v1, (len(S(46)) + v1)
		} else {
			arr[m8(G(125, append(S(47), (A(48).(int) - 4))[2], GEN[int](G(126, v3, v1)), (PT(49).M)(S(50)[m8(v2)])))] = min(append(S(51), v2)[2], ([]int{int(Num(v0)), L(52, 2, v2), (5*3 - v1)}[1]))
		}
	}
	v1 = G(127, 3, (len(S(53)) + v3))
	switch GG[int](128, 1) {
	case m8(FN(54)[m8(L(55))]((PT(56).M)(v2))):
		if !((8 < v0 || v1 < 0)) {
			return append(S(57), G(129, v0))[2], nil
		}
	case m8(append(S(58), G(130, v0, v2))[2]):
		v3 += PT(59).GM(131, PT(60).GM(132, v0))
		for i61 := b1(G(134, PT(63).GM(135, v0), int(int64(v1)))); i61 < 3 && !(!(v2 < 9)); i61 += 1 + b1((*PI(62) + G(133))) {
			arr[m8((Cell{A: v3, B: GEN[int]((PT(64).M)(v0))}.B))] = (PT(65).M)(([]int{FN(66)[m8(L(67))](v0), int(int64(v3)), (A(68).(int) - v2)}[1]))
			v3 += i61
			v0 = GG[int](136)
		}
	default:
		switch (A(69).(int) - (len(S(70)) + v2)) {
		case m8((func(a, b int) int { return a*2 + b }(S(71)[m8(v3)], (A(72).(int) - v2)))):
			v2, v3 = (<-CH(73) + min(v2, 4)), (G(137, v0, 8)*3 - FN(74)[m8(L(75))](v1))
			L(76, (*PI(77) + G(138, v1, v3)), FN(78)[m8(L(79))](([]int{v0, v2, v3}[1])))
			ch <- S(80)[m8(7)]
		case m8((*PI(81) + ([]int{7, v1, v1}[1]))):
			ch <- (*PI(82) + FN(83)[m8(v1)](2))
			v1 += (Cell{A: (map[int]int{m8((<-CH(84) + v3)): (A(85).(int) - v0), m8(G(139, 6)) + 8: (PT(86).M)(7)}[3]), B: MP(87)[m8(L(88))]}.B)
		default:
			w89 := (<-CH(90) + v1)
			_ = w89
		}
		arr[m8((Cell{A: GEN[int](7), B: 8}.B))]++
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
