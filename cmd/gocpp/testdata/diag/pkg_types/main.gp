package main

import (
	"<cstdlib>"

	"github.com/huuhait/game/engine"
)

func main() {
	b := engine.New()
	_ = cstdlib.abs(b)
	_ = cstdlib.abs(*b)
	_ = cstdlib.abs([]engine.Body{})
	_ = cstdlib.abs(map[engine.ID]*engine.Body{})
}
