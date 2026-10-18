package mid

import "example.com/three/base"

type Box struct {
	base.Rect
	Name string
}

func (b Box) Label() string { return b.Name + ":" + base.Label }

type Sq struct{ S int }

func (s *Sq) Area() int { return s.S * s.S }

func Sum(ss ...base.Shape) int {
	t := 0
	for _, s := range ss {
		t += s.Area()
	}
	return t
}

var Total = Sum(base.Rect{2, 3}, &Sq{4}) + base.Version
