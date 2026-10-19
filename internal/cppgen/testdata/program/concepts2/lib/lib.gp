package lib

type Visitor interface {
	Visit(n int, rest ...string) int
}

// Visit is named like its method: the C++ member is Visit_.
type Visit struct{}

func (Visit) Visit(n int, rest ...string) int { return n + len(rest) }

type S struct{ N int }

func (s S) Hello() string { return "s" }

// NPS is a defined pointer type: its method set is empty.
type NPS *S

type Número int

func (n Número) Hello() string { return "número" }

type Greeter interface{ Hello() string }

type Number interface{ ~int | ~float64 }

type Count int

func (c *Count) Add(n int)        { *c += Count(n) }
func (c Count) Get() int          { return int(c) }
func (c Count) Sum(xs ...int) int { return int(c) + len(xs) }

type Adder interface{ Add(int) }

func unexp[T Number](x T) T { return x }

func Twice[T Number](x T) T { return unexp(x) + x }
