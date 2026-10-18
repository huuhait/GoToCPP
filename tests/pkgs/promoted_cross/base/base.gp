package base

import "fmt"

type Animal interface {
	Sound() string
	name() string
}

type Base struct{ N string }

func (b Base) Hello() string     { return "hello " + b.N }
func (b *Base) Rename(n string)  { b.N = n }
func (b Base) name() string      { return "base-" + b.N }
func (b Base) Sound() string     { return "..." }

func Describe(a Animal) string { return a.name() + ":" + a.Sound() }

type Greeter interface{ Hello() string }

func Greet(g Greeter) { fmt.Println(g.Hello()) }
