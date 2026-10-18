package base

import "fmt"

const Version = 3

const Label = "base"

type Shape interface {
	Area() int
}

type Rect struct{ W, H int }

func (r Rect) Area() int { return r.W * r.H }

var Count int

func Next() int {
	Count++
	return Count
}

func Pair() struct{ A, B int } { return struct{ A, B int }{1, 2} }

func init() { fmt.Println("init base", Label) }
