// phase: 05

// A field named like its type (legal Go, and what an embedded field is): the C++ field type is
// written qualified, so the name of the field does not change the meaning of the type.
package main

import "fmt"

type Config struct{ Debug bool }

type Base struct{ n int }

func (b Base) Get() int { return b.n }

type App struct {
	Config Config
	Base   Base
}

type Server struct {
	Base
	Config
	Log Logger
}

type Logger struct{ Prefix string }

func (l Logger) Say(s string) string { return l.Prefix + s }

// The type is also used by a method's parameter, after a member named like it.
func (a *App) Reset(c Config, b Base) { a.Config, a.Base = c, b }

func main() {
	a := App{Config{true}, Base{1}}
	fmt.Println(a.Config.Debug, a.Base.Get(), a)
	a.Reset(Config{}, Base{5})
	fmt.Println(a)
	s := Server{Base{2}, Config{true}, Logger{"> "}}
	fmt.Println(s.Get(), s.Debug, s.Base.n, s.Config.Debug, s.Log.Say("hi"))
	var Base Base
	Base.n = 3
	fmt.Println(Base.Get(), Base)
}

// Output:
// true 1 {{true} {1}}
// {{false} {5}}
// 2 true 2 true > hi
// 3 {3}
