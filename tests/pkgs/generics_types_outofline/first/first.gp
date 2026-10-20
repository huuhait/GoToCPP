package first

import "example.com/outline/lib"

type Feet int

func New() lib.Wrapped[Feet] {
	var w lib.Wrapped[Feet]
	w.Add(3)
	w.Add(5)
	return w
}
