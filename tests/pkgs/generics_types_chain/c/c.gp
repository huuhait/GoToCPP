package c

import (
	"example.com/gtchain/a"
	"example.com/gtchain/b"
)

func Double(x a.Box[b.Feet]) a.Box[b.Feet] { return a.Box[b.Feet]{x.Get() * 2} }

func Any(f b.Feet) any { return b.Boxed(f) }
