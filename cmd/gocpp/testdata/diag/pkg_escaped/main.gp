package main

import (
	"<cstdlib>"

	"example.com/my-game/v2/util"
)

func main() {
	p := util.Pair{1, 2}
	_ = cstdlib.abs(p)
	_ = cstdlib.abs(&p)
}
