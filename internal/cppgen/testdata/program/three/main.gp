package main

import (
	"fmt"

	"example.com/three/base"
	"example.com/three/mid"
)

func main() {
	b := mid.Box{base.Rect{3, 4}, "box"}
	fmt.Println(b.Label(), b.Area(), mid.Total, base.Next(), base.Next())
	p := base.Pair()
	fmt.Println(p.A+p.B, base.Describe(base.Rect{1, 1}), base.Describe(&mid.Sq{2}))
}
