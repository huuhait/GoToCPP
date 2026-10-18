package dot

import "fmt"

const K = 7

type Pair struct{ A, B int }

func (p Pair) Sum() int { return p.A + p.B }

var Counter = 100

func Make(a, b int) Pair { return Pair{a, b} }

func Variadic(xs ...int) int { return len(xs) }

func init() { fmt.Println("init dot") }
