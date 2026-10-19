// phase: 07

// Conversions T(x) in generic code: between numeric type parameters, constants, to and from
// strings, slices, array and array pointers, between defined types with the same underlying type,
// pointers, and through interfaces; the results (truncation, wrap-around, rune conversion).
package main

import "fmt"

type Int interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint
}
type Float interface{ ~float32 | ~float64 }
type Num interface{ Int | Float }

type Celsius float64
type Fahr float64
type Small int8
type Text string
type Raw []byte

func Conv[T, U Num](x T) U { return U(x) }

func ToString[T ~string | ~[]byte | ~[]rune](x T) string { return string(x) }

func RuneStr[T Int](x T) string { return string(rune(x)) }

func FromString[S ~[]byte | ~[]rune](s string) S { return S(s) }

func Const[T Num]() (T, T, T) { return T(1), T(100), T(127) }

func Float2[T Float](x float64) T { return T(x) }

func Arr[S ~[]E, A ~[3]E, E any](s S) A { return A(s) }

func ArrPtr[S ~[]E, E any](s S) *[2]E { return (*[2]E)(s) }

func Struct[A, B ~struct {
	X int
	Y string
}](a A) B {
	return B(a)
}

func Ptr[P ~*E, Q ~*E, E any](p P) Q { return Q(p) }

func Via[T, U any](x T) (u U, ok bool) {
	u, ok = any(x).(U)
	return
}

type P1 struct {
	X int
	Y string
}
type P2 struct {
	X int
	Y string
}
type P3 struct {
	X int
	Y string `json:"y"`
}

func main() {
	fmt.Println(Conv[float64, int](3.99), Conv[float64, int](-3.99), Conv[float32, int8](100.7), Conv[int, float32](16777217), Conv[int64, float64](1<<53+1))
	fmt.Println(Conv[int, int8](300), Conv[int, uint8](-1), Conv[uint64, int64](1<<63), Conv[int8, uint64](-1), Conv[uint8, int8](200), Conv[int16, Small](257))
	fmt.Println(Conv[Celsius, Fahr](36.6), Conv[Fahr, int](98.6), Conv[int, Celsius](7), Conv[float64, float32](1e40) > 1e38)
	fmt.Println(ToString("abc"), ToString(Text("té")), ToString([]byte{104, 105}), ToString(Raw("raw")), ToString([]rune{0x65e5, 0x672c}))
	fmt.Printf("%q %q %q %q %q %q\n", RuneStr(65), RuneStr(int8(-1)), RuneStr(0x1F600), RuneStr(uint8(255)), RuneStr(0x110000), RuneStr(int64(0xD800)))
	fmt.Println(FromString[[]byte]("hé"), FromString[[]rune]("hé"), FromString[Raw]("ab"))
	fmt.Println(Const[int8]())
	fmt.Println(Const[uint8]())
	fmt.Println(Const[float32]())
	fmt.Println(Float2[float32](0.1), Float2[float64](0.1), Float2[Celsius](1.0/3), Float2[float32](1.0/3))
	fmt.Println(Arr[[]int, [3]int]([]int{1, 2, 3, 4}), Arr[[]string, [3]string]([]string{"a", "b", "c"}))
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		fmt.Println(Arr[[]int, [3]int]([]int{1, 2}))
	}()
	s := []int{1, 2, 3}
	p := ArrPtr(s)
	p[0] = 99
	fmt.Println(s, *p, len(p))
	fmt.Println(Struct[P1, P2](P1{1, "a"}), Struct[P2, P1](P2{2, "b"}), P3(P1{3, "c"}), P1(P3{4, "d"}))
	x := 5
	pp := Ptr[*int, *int](&x)
	*pp = 6
	fmt.Println(x)
	fmt.Println(Via[int, any](1))
	fmt.Println(Via[int, int64](1))
	fmt.Println(Via[Text, string](Text("t")))
	fmt.Println(Via[any, fmt.Stringer](nil))
	fmt.Println(Via[error, error](fmt.Errorf("e")))
}

// Output:
// 3 -3 100 1.6777216e+07 9.007199254740992e+15
// 44 255 -9223372036854775808 18446744073709551615 -56 1
// 36.6 98 7 true
// abc té hi raw 日本
// "A" "�" "😀" "ÿ" "�" "�"
// [104 195 169] [104 233] [97 98]
// 1 100 127
// 1 100 127
// 1 100 127
// 0.1 0.1 0.3333333333333333 0.33333334
// [1 2 3] [a b c]
// recovered: runtime error: cannot convert slice with length 2 to array or pointer to array with length 3
// [99 2 3] [99 2] 2
// {1 a} {2 b} {3 c} {4 d}
// 6
// 1 true
// 0 false
//  false
// <nil> false
// e true
