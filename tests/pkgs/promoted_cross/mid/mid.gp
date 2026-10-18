package mid

import "example.com/prom/base"

// Cat promotes Hello, Rename, Sound and base's name, and declares its own name: a different method.
type Cat struct {
	base.Base
	Lives int
}

func (c Cat) name() string  { return "mid-cat" }
func (c Cat) Sound() string { return "meow" }

type Wrapper struct {
	*Cat
	Tag string
}

type Both struct {
	base.Greeter
	base.Animal
}
