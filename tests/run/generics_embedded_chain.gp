// phase: 07

// Embedded generic types: promoted fields and methods through one and two levels of embedded
// instances (by value and by pointer), shadowing, an embedded type parameter-typed interface,
// interfaces satisfied through promotion, and %v / %+v of the nest.
package main

import "fmt"

type Base[T any] struct {
	ID   T
	Tags []string
}

func (b *Base[T]) Tag(s string)    { b.Tags = append(b.Tags, s) }
func (b Base[T]) Describe() string { return fmt.Sprintf("Base(%v,%v)", b.ID, b.Tags) }
func (b Base[T]) Key() T           { return b.ID }

type Named[T any] struct {
	Base[T]
	Name string
}

func (n Named[T]) Describe() string { return "Named:" + n.Base.Describe() + ":" + n.Name }

type Deep[K comparable, V any] struct {
	*Named[K]
	Val V
}

type Describer interface{ Describe() string }
type Keyer[T any] interface{ Key() T }
type Tagger interface{ Tag(string) }

type Wrapper[T Describer] struct {
	Inner T
	Count int
}

func (w Wrapper[T]) Describe() string {
	return fmt.Sprintf("Wrapper(%d):%s", w.Count, w.Inner.Describe())
}

func describeAll(ds ...Describer) {
	for _, d := range ds {
		fmt.Println(d.Describe())
	}
}

func keyOf[T any](k Keyer[T]) T { return k.Key() }

func main() {
	n := Named[int]{Base[int]{ID: 7}, "seven"}
	n.Tag("a")
	n.Tag("b")
	fmt.Println(n.Describe(), n.Base.Describe(), n.Key(), n.ID, n.Tags)
	d := Deep[string, float64]{&Named[string]{Base[string]{ID: "k"}, "deep"}, 2.5}
	d.Tag("x")
	fmt.Println(d.Describe(), d.Key(), d.Name, d.ID, d.Val)
	fmt.Printf("%v\n%+v\n", n, n)
	fmt.Printf("%T %T %T\n", n, d, d.Named)
	describeAll(n, &n, d, Base[bool]{ID: true}, &Base[bool]{})
	fmt.Println(keyOf[int](n), keyOf[string](d), keyOf[int](&n.Base), keyOf[bool](Base[bool]{ID: true}))
	var t Tagger = &n
	t.Tag("via iface")
	t = d
	t.Tag("via iface 2")
	fmt.Println(n.Tags, d.Tags)
	w := Wrapper[Named[int]]{n, 3}
	fmt.Println(w.Describe(), w.Inner.Name, w.Inner.ID, w.Count)
	w2 := Wrapper[Describer]{Base[int]{ID: 1}, 1}
	fmt.Println(w2.Describe())
	fmt.Printf("%T %T\n", w, w2)
	m := map[string]Describer{"n": n, "d": d, "w": w}
	for _, k := range []string{"d", "n", "w"} {
		fmt.Println(k, m[k].Describe())
	}
}

// Output:
// Named:Base(7,[a b]):seven Base(7,[a b]) 7 7 [a b]
// Named:Base(k,[x]):deep k deep k 2.5
// {{7 [a b]} seven}
// {Base:{ID:7 Tags:[a b]} Name:seven}
// main.Named[int] main.Deep[string,float64] *main.Named[string]
// Named:Base(7,[a b]):seven
// Named:Base(7,[a b]):seven
// Named:Base(k,[x]):deep
// Base(true,[])
// Base(false,[])
// 7 k 7 true
// [a b via iface] [x via iface 2]
// Wrapper(3):Named:Base(7,[a b via iface]):seven seven 7 3
// Wrapper(1):Base(1,[])
// main.Wrapper[main.Named[int]] main.Wrapper[main.Describer]
// d Named:Base(k,[x via iface 2]):deep
// n Named:Base(7,[a b via iface]):seven
// w Wrapper(3):Named:Base(7,[a b via iface]):seven
