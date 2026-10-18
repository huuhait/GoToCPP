// phase: 07

// Printf with operands of type parameter type: the verbs see the Go type of the type argument.
package main

import "fmt"

type Color int

func (c Color) String() string { return [...]string{"red", "green", "blue"}[c] }

type Temp float64

type Num interface{ ~int | ~int8 | ~uint8 | ~float64 }

type Point struct{ X, Y int }

func Show[T any](v T) string {
	return fmt.Sprintf("%v|%+v|%#v|%T", v, v, v, v)
}

func Fmt[T any](format string, v T) string { return fmt.Sprintf(format, v) }

func Hex[T ~int | ~uint8 | ~int8](v T) string { return fmt.Sprintf("%x %X %o %b %c %q %U", v, v, v, v, v, v, v) }

func Table[T Num](xs []T) string {
	out := ""
	for _, x := range xs {
		out += fmt.Sprintf("[%5v|%-5v|%05v]", x, x, x)
	}
	return out
}

func main() {
	fmt.Println(Show(1), Show("s"), Show(2.5), Show(true), Show(Color(1)), Show(Temp(36.6)))
	fmt.Println(Show(Point{1, 2}), Show(&Point{3, 4}), Show([]Color{0, 2}), Show(map[string]int{"a": 1}))
	fmt.Println(Fmt("%d", 42), Fmt("%5.2f", 3.14159), Fmt("%s", "str"), Fmt("%q", "q"), Fmt("%t", false), Fmt("%v", Color(2)))
	fmt.Println(Fmt("%d", Color(1)), Fmt("%s", Color(0)), Fmt("%x", "hi"), Fmt("%08.3f", Temp(2.5)), Fmt("%e", 1234.5))
	fmt.Println(Hex(65), Hex(uint8(97)), Hex(int8(-3)))
	fmt.Println(Table([]int{1, 22}), Table([]float64{1.5}), Table([]uint8{7}))
	fmt.Println(Fmt("%v", []int(nil)), Fmt("%d", []int{1, 2}), Fmt("%v", struct{}{}), Fmt("%v", [0]int{}), Fmt("%p", (*int)(nil)))
}

// Output:
// 1|1|1|int s|s|"s"|string 2.5|2.5|2.5|float64 true|true|true|bool green|green|1|main.Color 36.6|36.6|36.6|main.Temp
// {1 2}|{X:1 Y:2}|main.Point{X:1, Y:2}|main.Point &{3 4}|&{X:3 Y:4}|&main.Point{X:3, Y:4}|*main.Point [red blue]|[red blue]|[]main.Color{0, 2}|[]main.Color map[a:1]|map[a:1]|map[string]int{"a":1}|map[string]int
// 42  3.14 str "q" false blue
// 1 red 6869 0002.500 1.234500e+03
// 41 41 101 1000001 A 'A' U+0041 61 61 141 1100001 a 'a' U+0061 -3 -3 -3 -11 � '�' U+FFFFFFFFFFFFFFFD
// [    1|1    |00001][   22|22   |00022] [  1.5|1.5  |001.5] [    7|7    |00007]
// [] [1 2] {} [] 0x0
