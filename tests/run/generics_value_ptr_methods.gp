// phase: 07

// Value-receiver and pointer-receiver types as type arguments of one constraint.
package main

import "fmt"

type Animal interface {
	Sound() string
	Name() string
}

type Dog struct{ name string }

func (d Dog) Sound() string { return "woof" }
func (d Dog) Name() string  { return d.name }

type Cat struct{ name string }

func (c *Cat) Sound() string { return "meow" }
func (c *Cat) Name() string  { return c.name }

func Speak[A Animal](a A) string { return a.Name() + " says " + a.Sound() }

func Chorus[A Animal](as []A) []string {
	var out []string
	for _, a := range as {
		out = append(out, Speak(a))
	}
	return out
}

func Method[A Animal](a A) func() string { return a.Sound }

func main() {
	fmt.Println(Speak(Dog{"rex"}), Speak(&Cat{"tom"}), Speak(&Dog{"ptr"}))
	fmt.Println(Speak[Animal](Dog{"i"}), Speak[Animal](&Cat{"j"}))
	fmt.Println(Chorus([]Dog{{"a"}, {"b"}}), Chorus([]*Cat{{"c"}}), Chorus([]Animal{Dog{"d"}, &Cat{"e"}}))
	fmt.Println(Method(Dog{"m"})(), Method(&Cat{"n"})())
	var nilCat *Cat
	defer func() { fmt.Println("recovered:", recover() != nil) }()
	fmt.Println(Speak(nilCat))
}

// Output:
// rex says woof tom says meow ptr says woof
// i says woof j says meow
// [a says woof b says woof] [c says meow] [d says woof e says meow]
// woof meow
// recovered: true
