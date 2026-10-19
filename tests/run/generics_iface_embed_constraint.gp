// phase: 07

// Constraint interfaces that embed other constraints, methods and type sets.
package main

import "fmt"

type Number interface {
	~int | ~int8 | ~int64 | ~float64
}

type Stringish interface {
	~string
	Len() int
}

type Shower interface{ Show() string }

type NumShow interface {
	Number
	Shower
}

type ShowCmp interface {
	NumShow
	comparable
}

type Meters int
type Pct float64
type Tag string

func (m Meters) Show() string { return fmt.Sprint(int(m), "m") }
func (p Pct) Show() string    { return fmt.Sprint(float64(p), "%") }
func (t Tag) Len() int        { return len(t) }
func (t Tag) Show() string    { return "#" + string(t) }
func (m Meters) Cmp() int     { return int(m) }

func Describe[T NumShow](x T) string { return x.Show() }
func Total[T NumShow](xs ...T) (T, string) {
	var t T
	var parts string
	for _, x := range xs {
		t += x
		if parts != "" {
			parts += " "
		}
		parts += x.Show()
	}
	return t, parts
}
func Find[T ShowCmp](xs []T, x T) string {
	for i, v := range xs {
		if v == x {
			return fmt.Sprint(i, ":", v.Show())
		}
	}
	return "none"
}
func Measure[T Stringish](t T) int { return t.Len() + len(t) }
func Both[T interface {
	Stringish
	Shower
}](t T) string {
	return t.Show() + string(t[:1])
}
func Double[T interface {
	Number
	Shower
}](x T) string {
	return (x + x).Show()
}

func main() {
	fmt.Println(Describe(Meters(5)), Describe(Pct(12.5)))
	fmt.Println(Total(Meters(1), 2, 3))
	fmt.Println(Total(Pct(1.5), Pct(2)))
	fmt.Println(Find([]Meters{3, 4, 5}, 4), Find([]Pct{1}, 2))
	fmt.Println(Measure(Tag("héllo")), Both(Tag("go")), Double(Meters(21)), Double(Pct(0.25)))
	fmt.Printf("%T %v\n", Describe[Meters], Measure(Tag("")))
}

// Output:
// 5m 12.5%
// 6 1m 2m 3m
// 3.5 1.5% 2%
// 1:4m none
// 12 #gog 42m 0.5%
// func(main.Meters) string 0
