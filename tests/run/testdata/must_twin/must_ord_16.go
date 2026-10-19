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
	arr[m8((<-CH(1) + v2))] += 5
	w2 := FN(3)[m8(GEN[int](v2))](5)
	_ = w2
	v2, v3 = (func(a, b int) int { return a*2 + b }((map[int]int{m8((<-CH(4) + v0)): (v2 + w2), m8(G(100, v0, v3)) + 8: S(5)[m8(v3)]}[3]), S(6)[m8(append(S(7), v0)[2])])), FN(8)[m8(([]int{PT(9).M(v1), int(int64(v1)), G(101, v3, v2)}[1]))](int(int64(9)))
	switch (G(102, 5, v2) + (PT(10).M)(v2)) {
	case m8(([]int{(9*3 - 4), G(103), G(104, v2, v3)}[1])):
		L(11)
	case m8(int(Num(append(S(12), 0)[2]))):
		for i13 := b1((*PI(14) + G(105, 5, v3))); i13 < 3; i13 += 1 + b1(7) {
			v1, v2 = PT(15).GM(106, (Cell{A: 1, B: v1}.B)), ([]int{(*PI(16) + 4), v3, G(107, 8, v2, 0)}[1])
			v2, v3 = (A(17).(int) - (*PI(18) + v0)), ([]int{GEN[int](w2), G(108, v3, 5, v2), (*PI(19) + v2)}[1])
		}
	default:
		PT(20).X = v1
		arr[m8(append(S(21), (len(S(22)) + FN(23)[m8(v1)](v2)))[2])] += G(109, GG[int](110, (PT(24).M)(v3), MP(25)[m8(L(26))]), (A(27).(int) - GEN[int](v1)))
		arr[m8(G(111, L(28), G(112, (*PI(29) + 1), append(S(30), v3)[2])))] += (<-CH(31) + (*PI(32) + (<-CH(33) + 4)))
	}
	for i34 := b1(FN(37)[m8(MP(38)[m8(v0)])](int(Num(5)))); i34 < 3; i34 += 1 + b1(min(S(35)[m8(L(36))], (i34 + v3))) {
		v3 = G(113, GEN[int](8), int(int64(v3)))
	}
	w39 := G(114, G(115, w2, (len(S(40)) + 9)), v0)
	_ = w39
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
