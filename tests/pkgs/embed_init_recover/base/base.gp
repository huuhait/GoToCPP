package base

import "fmt"

var Trace []string

func Log(s string) { Trace = append(Trace, s) }

type Animal interface {
	Sound() string
	name() string // unexported: only this package's types satisfy it
}

type Base struct {
	ID   int
	Name string
}

func (b Base) Hello() string    { return fmt.Sprint("hello ", b.Name) }
func (b *Base) SetName(n string) { b.Name = n }
func (b Base) name() string      { return b.Name }

type Dog struct{ ID int; Name string }

type Puppy struct{ Base }

func (d Dog) Sound() string { return "woof" }
func (d *Dog) SetName(n string) { d.Name = n }
func (d Dog) name() string  { return d.Name }

func Describe(a Animal) string { return a.name() + ":" + a.Sound() }

var A = initA()

func initA() int { Log("base.A"); return 1 }

func init() { Log("base.init") }
