// phase: 07

// Defined types with methods as type arguments: methods are available through constraints, %T names the type.
package main

import "fmt"

type Celsius float64
type Fahrenheit float64

func (c Celsius) ToF() Fahrenheit    { return Fahrenheit(c*9/5 + 32) }
func (f Fahrenheit) ToC() Celsius    { return Celsius((f - 32) * 5 / 9) }
func (c Celsius) String() string     { return fmt.Sprintf("%.1f°C", float64(c)) }
func (f Fahrenheit) String() string  { return fmt.Sprintf("%.1f°F", float64(f)) }

type Temp interface {
	~float64
	fmt.Stringer
}

type Stack []int

func (s *Stack) Push(v int) { *s = append(*s, v) }
func (s Stack) Len() int    { return len(s) }

type Lener interface{ Len() int }

type Pusher interface {
	Push(int)
	Lener
}

func Hottest[T Temp](ts ...T) T {
	h := ts[0]
	for _, t := range ts[1:] {
		if t > h {
			h = t
		}
	}
	return h
}

func Describe[T Temp](t T) string { return fmt.Sprintf("%s (%.2f)", t, float64(t)) }

func Fill[P Pusher](p P, n int) int {
	for i := range n {
		p.Push(i)
	}
	return p.Len()
}

func TotalLen[L Lener](ls ...L) int {
	n := 0
	for _, l := range ls {
		n += l.Len()
	}
	return n
}

func main() {
	fmt.Println(Hottest(Celsius(20), Celsius(36.6), Celsius(-5)), Hottest(Fahrenheit(70), Fahrenheit(50)))
	fmt.Println(Describe(Celsius(100)), Describe(Fahrenheit(212)))
	fmt.Println(Celsius(100).ToF(), Fahrenheit(32).ToC())
	s := &Stack{}
	fmt.Println(Fill(s, 4), *s, TotalLen(*s, Stack{1}), TotalLen[Lener](Stack{1, 2}, s))
	fmt.Printf("%T %T %T\n", Hottest(Celsius(1)), s, TotalLen[Stack])
}

// Output:
// 36.6°C 70.0°F
// 100.0°C (100.00) 212.0°F (212.00)
// 212.0°F 0.0°C
// 4 [0 1 2 3] 5 6
// main.Celsius *main.Stack func(...main.Stack) int
