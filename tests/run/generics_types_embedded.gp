// phase: 07

// Embedding an instance of a generic type: promoted fields and methods, by value and by pointer.
package main

import "fmt"

type Base[T any] struct {
	id   T
	tags []string
}

func (b Base[T]) ID() T            { return b.id }
func (b *Base[T]) Tag(s string)    { b.tags = append(b.tags, s) }
func (b Base[T]) Tags() []string   { return b.tags }

type User struct {
	Base[int]
	Name string
}

type Doc struct {
	*Base[string]
	Title string
}

type Wrapper[T any] struct {
	Base[T]
	extra T
}

func main() {
	u := User{Base[int]{id: 7}, "ann"}
	u.Tag("admin")
	u.Tag("dev")
	fmt.Println(u.ID(), u.Tags(), u.id, u.Base.id)
	d := Doc{&Base[string]{id: "d1"}, "t"}
	d.Tag("x")
	fmt.Println(d.ID(), d.Tags(), len(d.Base.tags))
	w := Wrapper[float64]{Base[float64]{id: 1.5}, 2.5}
	w.Tag("w")
	fmt.Println(w.ID(), w.extra, w.Tags())
	fmt.Printf("%v %+v\n", u, u)
	fmt.Printf("%T %T %T\n", u, d, w)
}

// Output:
// 7 [admin dev] 7 7
// d1 [x] 1
// 1.5 2.5 [w]
// {{7 [admin dev]} ann} {Base:{id:7 tags:[admin dev]} Name:ann}
// main.User main.Doc main.Wrapper[float64]
