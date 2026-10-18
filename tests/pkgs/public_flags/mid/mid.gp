package mid

import (
	"fmt"

	"example.com/hdr/geo"
)

type Grid struct {
	Cells [2]geo.Cell
	Name  string
}

func (g *Grid) Sum() int { return g.Cells[0].Total() + g.Cells[1].Total() }

func Show(g *Grid) { fmt.Println(g.Name, g.Cells[0].N, g.Cells[1].N, g.Sum()) }
