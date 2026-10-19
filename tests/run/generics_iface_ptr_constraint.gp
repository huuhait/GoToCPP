// phase: 07

// The pointer-receiver constraint pattern: PT interface{ *T; Set(string) } builds values of T.
package main

import "fmt"

type Config struct{ name string }

func (c *Config) Set(s string) { c.name = "cfg:" + s }
func (c Config) Name() string  { return c.name }

type Counter struct{ n int }

func (c *Counter) Set(s string) { c.n = len(s) }

type Named interface{ Name() string }

func FromStrings[T any, PT interface {
	*T
	Set(string)
}](ss []string) []T {
	out := make([]T, len(ss))
	for i, s := range ss {
		p := PT(&out[i])
		p.Set(s)
	}
	return out
}

func New[T any, PT interface {
	*T
	Set(string)
}](s string) T {
	var v T
	PT(&v).Set(s)
	return v
}

func NewPtr[T any, PT interface {
	*T
	Set(string)
}](s string) *T {
	p := PT(new(T))
	p.Set(s)
	return p
}

func Apply[T any, PT interface {
	*T
	Named
	Set(string)
}](v T, s string) string {
	p := PT(&v)
	p.Set(s)
	return p.Name()
}

func main() {
	cs := FromStrings[Config]([]string{"a", "bb"})
	fmt.Println(cs, cs[1].Name())
	ns := FromStrings[Counter]([]string{"abc", "", "hello"})
	fmt.Println(ns)
	fmt.Println(New[Config]("x"), New[Counter]("four"), *NewPtr[Counter]("xy"), NewPtr[Config]("q").Name())
	fmt.Println(Apply(Config{}, "z"), Apply(Config{name: "old"}, "new"))
	fmt.Printf("%T %T\n", FromStrings[Config], New[Counter])
}

// Output:
// [{cfg:a} {cfg:bb}] cfg:bb
// [{3} {0} {5}]
// {cfg:x} {4} {2} cfg:q
// cfg:z cfg:new
// func([]string) []main.Config func(string) main.Counter
