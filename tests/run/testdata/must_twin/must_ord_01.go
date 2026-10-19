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
	if (int(Num(v3)) == (Cell{A: v2, B: v2}.B)) {
		return (*PI(1) + (len(S(2)) + v0)), nil
	}
	for i3 := b1(G(100, v1, G(101))); i3 < 3 && (*PI(4) + FN(5)[m8(L(6))](v0)) < v0; i3 += 1 + b1(([]int{(PT(7).M)(1), int(Num(v3)), v3}[1])) {
		v0, v1 = ([]int{append(S(8), FN(9)[m8(3)](v2))[2], min((map[int]int{m8(v1): v0, m8(v0) + 8: v2}[3]), L(10, 7, v2)), MP(11)[m8(G(102, v1, v2))]}[1]), Add2(Two(103, G(104)))
	}
	L(12, (A(13).(int) - (len(S(14)) + v2)))
	if (L(15) > (A(16).(int) - 9)) {
		return min((*PI(17) + 8), G(105, v1)), nil
	}
	w18 := (*PI(19) + G(106, v1))
	_ = w18
	v3 = (map[int]int{m8(S(20)[m8(v1)]): ([]int{v2, v1, 3}[1]), m8((len(S(21)) + v1)) + 8: (Cell{A: 1, B: w18}.B)}[3])
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
	for _, fa := range []int{-1, 100, 101, 102, 103, 104, 105, 106} {
		failAt = fa
		res, err := run()
		fmt.Println("run", fa, res, err, errors.Is(err, ErrBad))
	}
}
