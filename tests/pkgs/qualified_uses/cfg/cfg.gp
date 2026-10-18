package cfg

import "fmt"

var Names = []string{"alpha", "beta", "gamma"}

var Size = len(Names) * 10

var Table = [4]int{1, 2, 3, 4}

var Lookup = map[string]int{"x": 1, "y": 2}

var Greeting = fmt.Sprint("hello ", Names[0])

type Handler func(string) string

var Upper Handler = func(s string) string { return "<" + s + ">" }

var Hooks []func() int

const Max = 3

var Buf [Max * 2]byte

func Register(f func() int) { Hooks = append(Hooks, f) }

func init() {
	fmt.Println("cfg.init", Size, Greeting)
	Register(func() int { return Size })
}

type Opt struct {
	Name  string
	Level int
}

var Default = Opt{"def", 2}

func (o Opt) With(l int) Opt { o.Level = l; return o }
func (o *Opt) Bump()         { o.Level++ }
