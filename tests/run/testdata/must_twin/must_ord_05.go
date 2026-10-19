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
	v1, v2 = MP(1)[m8((func(a, b int) int { return a*2 + b }((v2*3 - v2), FN(2)[m8(v3)](v1))))], (A(3).(int) - v0)
	v0 = FN(4)[m8(L(5))](9)
	L(6, (FN(7)[m8((len(S(8)) + v2))](S(9)[m8(L(10))])*3 - (MP(11)[m8(L(12))] + 1)))
	arr[m8(min(([]int{2, v3, v1}[1]), GEN[int](3)))]++
	if (Add2(Two(100, v3)) < PT(13).M(1) || min(v2, v0) < (Cell{A: 4, B: v0}.B)) {
		if (S(14)[m8(L(15))] < G(101, v3, 2) || (v3 < v0 || 2 < v3)) {
			arr[m8(FN(16)[m8(G(102, v1, v1))]((v2 + 2)))] += PT(17).M((<-CH(18) + v0))
		} else {
			L(19, append(S(20), PT(21).M(v2))[2], PT(22).M(G(103, v0)), PT(23).M(FN(24)[m8(L(25))](v3)))
			v1 = v3
		}
		v1 = v0
	} else if v0 < v3 {
		PT(26).X = G(104)
		arr[m8((func(a, b int) int { return a*2 + b }(G(105, v2, v0), (PT(27).M)(v2))))]++
	}
	v3 += FN(28)[m8(int(int64(v2)))](G(106, 7))
	L(29, int(Num((8 + v3))), PT(30).M(MP(31)[m8(L(32))]), int(Num(FN(33)[m8(L(34))](v0))))
	if G(107, PT(35).M(4), (A(36).(int) - v2)) < GEN[int](int(int64(5))) {
		return (func(a, b int) int { return a*2 + b }(v1, int(int64(1)))), nil
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106, 107} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
