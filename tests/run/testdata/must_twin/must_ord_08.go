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
	v1 = G(100, GG[int](101, v0))
	v3 = v2
	v2 = FN(1)[m8(G(102, GG[int](103, v1)))](int(Num(v0)))
	if !(!(v1 < 5)) {
		return FN(2)[m8(G(104, 0, 1))](GG[int](105, v2)), nil
	}
	arr[m8((func(a, b int) int { return a*2 + b }(GEN[int](v0), v3)))] = GEN[int](GG[int](106, v3, v1))
	v3 = 4
	PT(3).X = v3
	arr[m8((len(S(4)) + v3))]++
	switch (Cell{A: (Cell{A: v3, B: v2}.B), B: GEN[int](v1)}.B) {
	case m8((<-CH(5) + (Cell{A: v1, B: 2}.B))):
		L(6, Add2(Two(107, GEN[int](v2))), S(7)[m8(min(6, 3))], min((*PI(8) + 9), GEN[int](v0)))
		w9 := v1
		_ = w9
	case m8((*PI(10) + v2)):
		if GB(108, (PT(11).M)(v2)) {
			v2 = (<-CH(12) + (0 + v3))
			v1 = GEN[int](GEN[int](append(S(13), v1)[2]))
			arr[m8((<-CH(14) + GEN[int](v3)))] = v1
		} else if int(Num(MP(15)[m8(L(16))])) < 8 {
			v2, v3 = FN(17)[m8((v2*3 - 2))](MP(18)[m8(v1)]), (func(a, b int) int { return a*2 + b }(GG[int](109), int(int64(v1))))
			arr[m8((Cell{A: G(110, int(int64(v2)), (func(a, b int) int { return a*2 + b }(1, v2))), B: (<-CH(19) + GG[int](111, v2))}.B))] += G(112, L(20, MP(21)[m8(L(22))], (v3 + v3)), (<-CH(23) + FN(24)[m8(L(25))](0)))
			v2 = G(113, append(S(26), (map[int]int{m8(v2): 3, m8(8) + 8: v2}[3]))[2], (*PI(27) + v1), FN(28)[m8(L(29))](FN(30)[m8(L(31))](v0)))
		} else {
			arr[m8(G(114, (v3 + v2)))] = PT(32).GM(115, (v0*3 - v2))
		}
		for i33 := b1((FN(37)[m8(L(38))](v3)*3 - (len(S(39)) + v3))); i33 < 3 && !((L(34) > v2)); i33 += 1 + b1(int(int64(S(35)[m8(L(36))]))) {
			ch <- FN(40)[m8((A(41).(int) - append(S(42), v2)[2]))]((PT(43).M)((<-CH(44) + i33)))
			ch <- FN(45)[m8(v1)]((len(S(46)) + (PT(47).M)(2)))
		}
	default:
		v1 += ((PT(48).M)(8)*3 - G(116))
		arr[m8((*PI(49) + (len(S(50)) + G(117, 5, v2))))] += (v3*3 - G(118, G(119, 8), GEN[int](8), 0))
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
