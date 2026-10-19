package model

// Plain shows a slice in an exported field and has no methods.
type Plain struct {
	Tags []string
	N    int
}

// Opaque holds slices and pointers in private fields only: the generators take it as a value.
type Opaque struct {
	tags []string
	p    *int
}

// Cloner has a Clone method, so it is cloned with it.
type Cloner struct{ Tags []string }

func (c Cloner) Clone() Cloner { return Cloner{append([]string(nil), c.Tags...)} }

func (c Cloner) Equal(o Cloner) bool { return len(c.Tags) == len(o.Tags) }

type List []int

func (l List) Equal(o List) bool { return len(l) == len(o) }
