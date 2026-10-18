package mid

import (
	"example.com/app/base"
)

type Cat struct {
	base.Base
	Lives int
}

func (c Cat) Sound() string { return "meow" }

// mid's own unexported method with the same name as base's: a different method.
func (c Cat) name() string { return "mid-cat" }

type Pet struct {
	*base.Puppy
	Tag string
}

var B = base.A + 10

func init() { base.Log("mid.init") }
