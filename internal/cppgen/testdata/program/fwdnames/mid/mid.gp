package mid

import "example.com/fwdnames/base"

// Cat declares its own name and promotes base's: two different methods in Go.
type Cat struct {
	base.Base
	Lives int
}

func (c Cat) name() string { return "mid-cat" }

func (c Cat) Both() string { return c.name() + "/" + c.Hello() }
