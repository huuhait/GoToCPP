// phase: 07

// Methods promoted from an embedded generic instance satisfy interfaces, through values and pointers.
package main

import "fmt"

type Namer interface{ Name() string }
type Tagger interface{ Tag(string) }

type Entity[T any] struct {
	id   T
	name string
	tags []string
}

func (e Entity[T]) Name() string   { return e.name }
func (e *Entity[T]) Tag(s string)  { e.tags = append(e.tags, s) }

type Player struct {
	Entity[int]
	score int
}

type Pet struct {
	*Entity[string]
}

func Describe(n Namer) string { return "name:" + n.Name() }

func TagAll(ts ...Tagger) {
	for _, t := range ts {
		t.Tag("all")
	}
}

func main() {
	p := &Player{Entity[int]{1, "bob", nil}, 5}
	pet := Pet{&Entity[string]{"p", "rex", nil}}
	fmt.Println(Describe(*p), Describe(p), Describe(pet))
	TagAll(p, pet)
	fmt.Println(p.tags, pet.tags)
	var n Namer = *p
	_, isTagger := n.(Tagger)
	var pn Namer = p
	_, ptrTagger := pn.(Tagger)
	fmt.Println(isTagger, ptrTagger)
	fmt.Printf("%T %T\n", n, pn)
}

// Output:
// name:bob name:bob name:rex
// [all] [all]
// false true
// main.Player *main.Player
