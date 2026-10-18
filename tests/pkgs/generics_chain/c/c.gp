package c

import (
	"example.com/gchain/a"
	"example.com/gchain/b"
)

func Both(f b.Feet, m a.Meters) (b.Feet, a.Meters) { return a.Double(f), b.Quad(m) }
