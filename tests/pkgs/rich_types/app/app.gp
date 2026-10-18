package app

import (
	"example.com/rich/colors"
	"example.com/rich/geo"
)

type Item struct {
	geo.Point
	Color colors.Color
	*geo.Stack
	geo.Named
}

type Scene struct {
	Items map[colors.Color][]Item
	Tf    geo.Transform
}

var Default = Scene{
	Items: map[colors.Color][]Item{colors.Red: {{Point: geo.Point{1, 1}}}},
	Tf:    func(p geo.Point) geo.Point { return p.Add(geo.Unit) },
}

var Sum = geo.Points{geo.Origin, geo.Unit}.Sum()

var _ = geo.Pair
