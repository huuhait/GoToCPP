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
	v2, v3 = G(100, GEN[int](8), (len(S(1)) + v3)), (v2 + (func(a, b int) int { return a*2 + b }(v1, 9)))
	L(2, v2)
	w3 := PT(4).GM(101, (3 + int(int64(8))))
	_ = w3
	switch FN(5)[m8(L(6))](append(S(7), v3)[2]) {
	case m8(0):
		L(8, (map[int]int{m8(G(102, v2, v2)): FN(9)[m8(L(10))](v0), m8(v1) + 8: G(103, v2, v1)}[3]), (len(S(11)) + (map[int]int{m8(5): v3, m8(4) + 8: 3}[3])))
		arr[m8(FN(12)[m8(Add2(Two(104, v3)))]((9*3 - v2)))]++
		ch <- L(13)
	case m8(G(105, FN(14)[m8(L(15))](3))):
		for i16 := b1(S(20)[m8(int(Num(v2)))]); i16 < 3 && MP(17)[m8((Cell{A: v2, B: v3}.B))] < w3; i16 += 1 + b1((len(S(18)) + (A(19).(int) - 5))) {
			w21 := G(106)
			_ = w21
			v1 = (PT(22).M)(2)
			L(23, GG[int](107, (G(108, w21, v3)*3 - (*PI(24) + i16))), w21)
		}
		switch FN(25)[m8(G(109, 5))](FN(26)[m8(v1)](6)) {
		case m8(PT(27).GM(110, (Cell{A: v3, B: v3}.B))):
			ch <- 8
		case m8(GEN[int](PT(28).GM(111, 8))):
			v1 = PT(29).M((map[int]int{m8(v2): 1, m8(3) + 8: w3}[3]))
			ch <- G(112, min(G(113, v3, v0), (w3*3 - w3)), PT(30).M(S(31)[m8(3)]), L(32, Add2(Two(114, 3))))
		default:
			L(33, (map[int]int{m8((*PI(34) + FN(35)[m8(L(36))](v3))): GEN[int](FN(37)[m8(L(38))](w3)), m8(int(Num(v1))) + 8: PT(39).M(L(40, 1, 4, v1))}[3]), L(41, (len(S(42)) + PT(43).M(v0))), PT(44).M(Add2(Two(115, (len(S(45)) + 3)))))
		}
	default:
		for i46 := b1(PT(47).GM(120, v0)); i46 < 3 && GB(116, Add2(Two(117, v3))); i46 += 1 + b1(G(118, G(119, v2, 9), GEN[int](7))) {
			arr[m8(S(48)[m8(L(49))])] = FN(50)[m8((A(51).(int) - v1))](5)
			w52 := w3
			_ = w52
			v1 += w52
		}
	}
	v3 += PT(53).GM(121, w3)
	for i54 := b1(v3); i54 < 3 && ((len(S(55)) + w3) == FN(56)[m8(L(57))](v1)); i54 += 1 + b1(append(S(58), FN(59)[m8(v0)](v1))[2]) {
		if ((*PI(60) + 7) == (A(61).(int) - v1)) {
			v2 = v1
		} else if (PT(62).GM(122, v2) < int(Num(1)) && min(0, i54) < append(S(63), v2)[2]) {
			arr[m8((<-CH(64) + int(int64(i54))))] += w3
			w65 := w3
			_ = w65
			ch <- S(66)[m8(L(67))]
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
