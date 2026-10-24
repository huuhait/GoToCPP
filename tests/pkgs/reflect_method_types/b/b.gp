package b

import "example.com/reflmt/a"

type W struct {
	a.T
	Extra a.Box[string]
	Ptr   *a.Box[float64]
}

func (w W) Wrap(x a.T, ys ...a.U) a.Box[a.T]             { return a.Box[a.T]{} }
func (w *W) Iface(i interface{ Make(a.T) (a.U, error) }) {}

type Mine interface{ Do(a.U) W }
