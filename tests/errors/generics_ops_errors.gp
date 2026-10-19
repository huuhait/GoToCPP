// phase: 07

// Operations that a constraint's type set does not allow, and constraint methods that a type argument lacks.
package main

type Counter struct{ n int }

func (c *Counter) Inc()    { c.n++ }
func (c Counter) Get() int { return c.n }

type Incer interface{ Inc() }
type IntOrStr interface{ ~int | ~string }
type Both interface {
	~int
	Get() int
}

func CallInc[T Incer](x T)            { x.Inc() }
func Add[T any](a, b T) T             { return a + b } // ERROR "operator \+ not defined on a"
func Less[T comparable](a, b T) bool  { return a < b } // ERROR "type parameter T cannot use operator <"
func Neg[T IntOrStr](a T) T           { return -a } // ERROR "operator - not defined on a \(variable of type T constrained by IntOrStr\)"
func Mul[T IntOrStr](a, b T) T        { return a * b } // ERROR "operator \* not defined on a"
func Len[T ~int | ~[]int](x T) int    { return len(x) } // ERROR "invalid argument: x \(variable of type T constrained by ~int \| ~\[\]int\) for built-in len"
func Shift[T ~float64 | ~int](x T) T  { return x << 1 } // ERROR "shifted operand x \(variable of type T constrained by ~float64 \| ~int\) must be integer"
func Mod[T ~float64 | ~int](x T) T    { return x % 2 } // ERROR "operator % not defined on x"
func Idx[T ~int | ~string](x T) byte  { return x[0] } // ERROR "cannot index x \(variable of type T constrained by ~int \| ~string\)"
func Call[T any](x T)                 { x() } // ERROR "cannot call x \(variable of type T constrained by any\): no specific type"
func Field[T any](x T) int            { return x.n } // ERROR "x.n undefined \(type T has no field or method n\)"
func Meth[T Both](x T) int            { return x.Get() + int(x) }
func Eq[T any](a, b T) bool           { return a == b } // ERROR "incomparable types in type set"
func Conv[T ~int | ~string](x T) []byte { return []byte(x) } // ERROR "cannot convert x \(variable of type T constrained by ~int \| ~string\) to type \[\]byte"

type MyInt int

func main() {
	CallInc(Counter{})            // ERROR "Counter\) does not satisfy Incer \(method Inc has pointer receiver\)"
	CallInc(&Counter{})
	Less([]int{1}, []int{2})      // ERROR "\[\]int does not satisfy comparable"
	Neg(3)
	Meth(MyInt(1))                // ERROR "MyInt\) does not satisfy Both \(missing method Get\)"
	Meth(Counter{})               // ERROR "Counter\) does not satisfy Both"
	Add(1, 2)
	Mul("a", "b")
}
