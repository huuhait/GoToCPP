package second

import (
	"example.com/outline/first"
	"example.com/outline/lib"
)

func Last(w lib.Wrapped[first.Feet]) first.Feet {
	var value interface{ Last() first.Feet } = w
	return value.Last()
}

func Add(w *lib.Wrapped[first.Feet], v first.Feet) {
	var pointer interface{ Add(first.Feet) } = w
	pointer.Add(v)
}
