package b

import "example.com/gtchain/a"

type Feet int

// Both packages use Box[Feet]: it is one C++ type.
func Boxed(f Feet) a.Box[Feet] { return a.Wrap(f) }

func Pair(f Feet, m a.Meters) (a.Box[Feet], a.Box[a.Meters]) { return Boxed(f), a.Wrap(m) }

func Nest(f Feet) a.Box[a.Box[Feet]] { return a.Box[a.Box[Feet]]{Boxed(f)} }
