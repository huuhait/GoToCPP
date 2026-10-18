package shapes

import (
	"fmt"

	"example.com/shapes/inner"
)

type Square struct{ Side int }

func (s Square) Area() int { return inner.Mul(s.Side, s.Side) }

func init() { fmt.Println("init shapes") }
