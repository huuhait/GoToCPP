// phase: 07

// Constants, iota and typed constants used with generic code.
package main

import "fmt"

type Weekday int

const (
	Sunday Weekday = iota
	Monday
	Tuesday
)

func (d Weekday) String() string { return [...]string{"Sun", "Mon", "Tue"}[d] }

type Flags uint8

const (
	FA Flags = 1 << iota
	FB
	FC
)

const Big = 1 << 40
const Pi = 3.14159

func Next[T ~int | ~uint8](v T) T { return v + 1 }

func Has[T ~uint8](set, flag T) bool { return set&flag != 0 }

func Const[T ~int | ~int64 | ~float64]() T { return Big / 1024 }

func Scale[T ~float32 | ~float64](x T) T { return x * Pi }

func Names[T fmt.Stringer](xs ...T) []string {
	var out []string
	for _, x := range xs {
		out = append(out, x.String())
	}
	return out
}

func Add[T ~int](a T) T { return a + Monday2 }

const Monday2 = 7

func main() {
	fmt.Println(Next(Monday), Next(Weekday(1)), Next(FA), Next(FB|FC))
	fmt.Println(Has(FA|FC, FC), Has(FA|FC, FB), Has[uint8](255, 128))
	fmt.Println(Const[int](), Const[int64](), Const[float64]())
	fmt.Println(Scale(2.0), Scale[float32](2))
	fmt.Println(Names(Sunday, Tuesday), Names(Next(Sunday)))
	fmt.Println(Add(1), Add(Weekday(0)) == 7)
	fmt.Printf("%v %d %s %T\n", Next(Sunday), Next(Sunday), Next(Monday), Next(Tuesday))
}

// Output:
// Tue Tue 2 7
// true false true
// 1073741824 1073741824 1.073741824e+09
// 6.28318 6.28318
// [Sun Tue] [Mon]
// 8 true
// Mon 1 Tue main.Weekday
