// phase: 07

// Structs that embed interfaces or pointers satisfy constraints by promotion; the method set of a type argument decides.
package main

import "fmt"

type Namer interface{ Name() string }

type Dog struct{ n string }

func (d Dog) Name() string { return d.n }

type Pet struct {
	Namer
	age int
}

type Owner struct {
	*Pet
	title string
}

type Registry struct{ Namer }

func Names[T Namer](xs ...T) []string {
	var out []string
	for _, x := range xs {
		out = append(out, x.Name())
	}
	return out
}

func Describe[T Namer](x T) string { return fmt.Sprintf("%T:%s", x, x.Name()) }

func main() {
	p := Pet{Dog{"rex"}, 3}
	o := Owner{&p, "Dr"}
	fmt.Println(Names(p, p), Names(o), Names(&o), Names[Namer](Dog{"a"}, p, o))
	fmt.Println(Describe(p), Describe(o), Describe(Registry{o}))
	var empty Pet
	defer func() { fmt.Println("recovered:", recover() != nil) }()
	fmt.Println(Names(empty))
}

// Output:
// [rex rex] [rex] [rex] [a rex rex]
// main.Pet:rex main.Owner:rex main.Registry:rex
// recovered: true
