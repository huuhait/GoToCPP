// phase: 09

// %T, %v, %+v and %#v of generic type instances: nested instances, pointers, slices, maps, arrays,
// func and channel types, anonymous structs and interfaces as type arguments. The type names are
// Go's: full import paths inside the brackets and no space after the commas.
package main

import (
	"fmt"
	"os"
)

type Pair[K comparable, V any] struct {
	Key K
	Val V
}

type Box[T any] struct{ V T }

type List[T any] struct {
	Head T
	Tail *List[T]
}

type Fn[A, R any] func(A) R

type Named int

type Shape interface{ Area() float64 }

type Sq struct{ S float64 }

func (s Sq) Area() float64 { return s.S * s.S }

func show[T any](v T) {
	fmt.Printf("%T | %v | %+v | %#v\n", v, v, v, v)
}

func main() {
	show(Pair[string, int]{"a", 1})
	show(Pair[string, []int]{"b", []int{1, 2}})
	show(Pair[int, Pair[string, bool]]{1, Pair[string, bool]{"x", true}})
	show(Box[Box[Box[int]]]{Box[Box[int]]{Box[int]{7}}})
	show(&Box[string]{"ptr"})
	show([]Box[int]{{1}, {2}})
	show(map[string]Box[float64]{"k": {1.5}})
	show([2]Pair[Named, int8]{{1, 2}, {3, 4}})
	show(Box[[]Pair[string, int]]{[]Pair[string, int]{{"z", 26}}})
	show(Box[map[Named][]string]{map[Named][]string{5: {"five"}}})
	show(Box[*int]{nil})
	show(Box[func(int) string]{nil})
	show(Box[chan int]{nil})
	show(Box[<-chan Box[int]]{nil})
	show(Box[chan<- string]{nil})
	show(Box[struct{ X, Y int }]{struct{ X, Y int }{1, 2}})
	show(Box[struct {
		Name string `json:"name"`
		Tags []string
	}]{})
	show(Box[interface{}]{42})
	show(Box[any]{"any"})
	show(Box[error]{nil})
	show(Box[Shape]{Sq{3}})
	show(Box[fmt.Stringer]{nil})
	show(Box[*os.File]{nil})
	show(Box[[]any]{[]any{1, "a", nil}})
	show(Box[map[string]any]{map[string]any{"a": 1}})
	show(Box[Fn[int, string]]{nil})
	show(Box[[3]Box[[]byte]]{})
	show(Box[uint8]{255})
	show(Box[rune]{'x'})
	show(Box[byte]{'y'})
	show(Box[complex64]{1 + 2i})
	show(Box[uintptr]{9})
	show(Pair[[2]int, Pair[string, *Box[int]]]{})
	l := &List[string]{"a", &List[string]{"b", nil}}
	fmt.Printf("%T %+v\n", l, *l.Tail)
	var s Shape = Sq{2}
	show(Pair[string, Shape]{"sq", s})
	show(Pair[Shape, Shape]{})
}

// Output:
// main.Pair[string,int] | {a 1} | {Key:a Val:1} | main.Pair[string,int]{Key:"a", Val:1}
// main.Pair[string,[]int] | {b [1 2]} | {Key:b Val:[1 2]} | main.Pair[string,[]int]{Key:"b", Val:[]int{1, 2}}
// main.Pair[int,main.Pair[string,bool]] | {1 {x true}} | {Key:1 Val:{Key:x Val:true}} | main.Pair[int,main.Pair[string,bool]]{Key:1, Val:main.Pair[string,bool]{Key:"x", Val:true}}
// main.Box[main.Box[main.Box[int]]] | {{{7}}} | {V:{V:{V:7}}} | main.Box[main.Box[main.Box[int]]]{V:main.Box[main.Box[int]]{V:main.Box[int]{V:7}}}
// *main.Box[string] | &{ptr} | &{V:ptr} | &main.Box[string]{V:"ptr"}
// []main.Box[int] | [{1} {2}] | [{V:1} {V:2}] | []main.Box[int]{main.Box[int]{V:1}, main.Box[int]{V:2}}
// map[string]main.Box[float64] | map[k:{1.5}] | map[k:{V:1.5}] | map[string]main.Box[float64]{"k":main.Box[float64]{V:1.5}}
// [2]main.Pair[main.Named,int8] | [{1 2} {3 4}] | [{Key:1 Val:2} {Key:3 Val:4}] | [2]main.Pair[main.Named,int8]{main.Pair[main.Named,int8]{Key:1, Val:2}, main.Pair[main.Named,int8]{Key:3, Val:4}}
// main.Box[[]main.Pair[string,int]] | {[{z 26}]} | {V:[{Key:z Val:26}]} | main.Box[[]main.Pair[string,int]]{V:[]main.Pair[string,int]{main.Pair[string,int]{Key:"z", Val:26}}}
// main.Box[map[main.Named][]string] | {map[5:[five]]} | {V:map[5:[five]]} | main.Box[map[main.Named][]string]{V:map[main.Named][]string{5:[]string{"five"}}}
// main.Box[*int] | {<nil>} | {V:<nil>} | main.Box[*int]{V:(*int)(nil)}
// main.Box[func(int) string] | {<nil>} | {V:<nil>} | main.Box[func(int) string]{V:(func(int) string)(nil)}
// main.Box[chan int] | {<nil>} | {V:<nil>} | main.Box[chan int]{V:(chan int)(nil)}
// main.Box[<-chan main.Box[int]] | {<nil>} | {V:<nil>} | main.Box[<-chan main.Box[int]]{V:(<-chan main.Box[int])(nil)}
// main.Box[chan<- string] | {<nil>} | {V:<nil>} | main.Box[chan<- string]{V:(chan<- string)(nil)}
// main.Box[struct { X int; Y int }] | {{1 2}} | {V:{X:1 Y:2}} | main.Box[struct { X int; Y int }]{V:struct { X int; Y int }{X:1, Y:2}}
// main.Box[struct { Name string "json:\"name\""; Tags []string }] | {{ []}} | {V:{Name: Tags:[]}} | main.Box[struct { Name string "json:\"name\""; Tags []string }]{V:struct { Name string "json:\"name\""; Tags []string }{Name:"", Tags:[]string(nil)}}
// main.Box[interface {}] | {42} | {V:42} | main.Box[interface {}]{V:42}
// main.Box[interface {}] | {any} | {V:any} | main.Box[interface {}]{V:"any"}
// main.Box[error] | {<nil>} | {V:<nil>} | main.Box[error]{V:error(nil)}
// main.Box[main.Shape] | {{3}} | {V:{S:3}} | main.Box[main.Shape]{V:main.Sq{S:3}}
// main.Box[fmt.Stringer] | {<nil>} | {V:<nil>} | main.Box[fmt.Stringer]{V:fmt.Stringer(nil)}
// main.Box[*os.File] | {<nil>} | {V:<nil>} | main.Box[*os.File]{V:(*os.File)(nil)}
// main.Box[[]interface {}] | {[1 a <nil>]} | {V:[1 a <nil>]} | main.Box[[]interface {}]{V:[]interface {}{1, "a", interface {}(nil)}}
// main.Box[map[string]interface {}] | {map[a:1]} | {V:map[a:1]} | main.Box[map[string]interface {}]{V:map[string]interface {}{"a":1}}
// main.Box[main.Fn[int,string]] | {<nil>} | {V:<nil>} | main.Box[main.Fn[int,string]]{V:(main.Fn[int,string])(nil)}
// main.Box[[3]main.Box[[]uint8]] | {[{[]} {[]} {[]}]} | {V:[{V:[]} {V:[]} {V:[]}]} | main.Box[[3]main.Box[[]uint8]]{V:[3]main.Box[[]uint8]{main.Box[[]uint8]{V:[]uint8(nil)}, main.Box[[]uint8]{V:[]uint8(nil)}, main.Box[[]uint8]{V:[]uint8(nil)}}}
// main.Box[uint8] | {255} | {V:255} | main.Box[uint8]{V:0xff}
// main.Box[int32] | {120} | {V:120} | main.Box[int32]{V:120}
// main.Box[uint8] | {121} | {V:121} | main.Box[uint8]{V:0x79}
// main.Box[complex64] | {(1+2i)} | {V:(1+2i)} | main.Box[complex64]{V:(1+2i)}
// main.Box[uintptr] | {9} | {V:9} | main.Box[uintptr]{V:0x9}
// main.Pair[[2]int,main.Pair[string,*main.Box[int]]] | {[0 0] { <nil>}} | {Key:[0 0] Val:{Key: Val:<nil>}} | main.Pair[[2]int,main.Pair[string,*main.Box[int]]]{Key:[2]int{0, 0}, Val:main.Pair[string,*main.Box[int]]{Key:"", Val:(*main.Box[int])(nil)}}
// *main.List[string] {Head:b Tail:<nil>}
// main.Pair[string,main.Shape] | {sq {2}} | {Key:sq Val:{S:2}} | main.Pair[string,main.Shape]{Key:"sq", Val:main.Sq{S:2}}
// main.Pair[main.Shape,main.Shape] | {<nil> <nil>} | {Key:<nil> Val:<nil>} | main.Pair[main.Shape,main.Shape]{Key:main.Shape(nil), Val:main.Shape(nil)}
