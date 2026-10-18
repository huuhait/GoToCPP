package outer

import "example.com/fwdowner/inner"

// Wrapper promotes the methods of a type of package inner.
type Wrapper struct {
	inner.Base
	Name string
}

func New(n int) *Wrapper { return &Wrapper{Base: inner.Base{N: n}, Name: "w"} }
