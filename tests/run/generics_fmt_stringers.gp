// phase: 07

// fmt of generic instances that implement String, Error, GoString and Format: value and pointer
// receivers (the method set rules for what Println finds), nested in slices, maps, structs and
// pointers, with the verbs %v %+v %#v %s %q %d %x and width.
package main

import "fmt"

type Opt[T any] struct {
	v  T
	ok bool
}

func Some[T any](v T) Opt[T] { return Opt[T]{v, true} }

func (o Opt[T]) String() string {
	if !o.ok {
		return "None"
	}
	return fmt.Sprintf("Some(%v)", o.v)
}

type PtrStr[T any] struct{ v T }

func (p *PtrStr[T]) String() string { return fmt.Sprint("PtrStr<", p.v, ">") }

type E[T any] struct{ code T }

func (e E[T]) Error() string { return fmt.Sprint("E", e.code) }

type G[T any] struct{ v T }

func (g G[T]) GoString() string { return fmt.Sprintf("G[%T](%v)", g.v, g.v) }

type F[T any] struct{ v T }

func (f F[T]) Format(s fmt.State, verb rune) {
	w, hasW := s.Width()
	fmt.Fprintf(s, "F(%c,%v,%d,%v,%v)", verb, f.v, w, hasW, s.Flag('+'))
}

type Both[T any] struct{ v T }

func (b Both[T]) String() string { return "str" }
func (b Both[T]) Error() string  { return "err" }

type Wrapper[T any] struct {
	Inner T
	Ptr   *T
	List  []T
	Map   map[string]T
}

func main() {
	fmt.Println(Some(3), Opt[string]{}, Some("x"), []Opt[int]{Some(1), {}}, map[string]Opt[bool]{"k": Some(true)})
	fmt.Printf("%v %s %+v %#v %q %d\n", Some(7), Some(7), Some(7), Some(7), Some("q"), Some(7))
	fmt.Printf("%8v|%-8v|%8s\n", Some(1), Some(2), Opt[int]{})
	ps := PtrStr[int]{5}
	fmt.Println(ps, &ps, []PtrStr[int]{ps}, []*PtrStr[int]{&ps})
	fmt.Printf("%v %v %s\n", ps, &ps, &ps)
	fmt.Println(E[int]{404}, E[string]{"nf"}, []error{E[int]{1}, E[bool]{true}})
	fmt.Printf("%v %s %q %x\n", E[int]{2}, E[int]{3}, E[int]{4}, E[int]{5})
	var err error = E[float64]{1.5}
	fmt.Printf("%v %T\n", err, err)
	fmt.Printf("%v %+v %#v\n", G[int]{1}, G[int]{2}, G[string]{"s"})
	fmt.Printf("%#v %#v\n", []G[int]{{1}}, struct{ X G[bool] }{G[bool]{true}})
	fmt.Printf("%v|%+v|%10d|%s\n", F[int]{1}, F[int]{2}, F[string]{"s"}, F[bool]{true})
	fmt.Println(F[int]{9}, []F[int]{{1}, {2}})
	fmt.Println(Both[int]{}, fmt.Sprint(Both[int]{}), fmt.Sprintf("%v %s", Both[string]{}, Both[bool]{}))
	w := Wrapper[Opt[int]]{Some(1), nil, []Opt[int]{Some(2), {}}, map[string]Opt[int]{"a": Some(3)}}
	fmt.Printf("%v\n%+v\n", w, w)
	x := Some(4)
	w.Ptr = &x
	fmt.Println(*w.Ptr, w.Ptr.String())
	fmt.Printf("%#v\n", Wrapper[int]{Inner: 1, List: []int{2}})
	var np *PtrStr[int]
	var no *Opt[int]
	fmt.Printf("%v %v\n", np == nil, no == nil)
	fmt.Println(fmt.Sprint(Some(1), Some(2)), fmt.Sprint("a", Some(1), 2, Some(3)), fmt.Sprintln(Some(0), 1))
}

// Output:
// Some(3) None Some(x) [Some(1) None] map[k:Some(true)]
// Some(7) Some(7) Some(7) main.Opt[int]{v:7, ok:true} "Some(q)" {7 %!d(bool=true)}
//  Some(1)|Some(2) |    None
// {5} PtrStr<5> [{5}] [PtrStr<5>]
// {5} PtrStr<5> PtrStr<5>
// E404 Enf [E1 Etrue]
// E2 E3 "E4" 4535
// E1.5 main.E[float64]
// {1} {v:2} G[string](s)
// []main.G[int]{G[int](1)} struct { X main.G[bool] }{X:G[bool](true)}
// F(v,1,0,false,false)|F(v,2,0,false,true)|F(d,s,10,true,false)|F(s,true,0,false,false)
// F(v,9,0,false,false) [F(v,1,0,false,false) F(v,2,0,false,false)]
// err err err err
// {Some(1) <nil> [Some(2) None] map[a:Some(3)]}
// {Inner:Some(1) Ptr:<nil> List:[Some(2) None] Map:map[a:Some(3)]}
// Some(4) Some(4)
// main.Wrapper[int]{Inner:1, Ptr:(*int)(nil), List:[]int{2}, Map:map[string]int(nil)}
// true true
// Some(1) Some(2) aSome(1) 2 Some(3) Some(0) 1
