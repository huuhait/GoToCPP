package main

import (
	"strconv"

	"example.com/app/util"
)

// parse is a must function.
func parse!(s string) int {
	return strconv.Atoi(s)
}

// plainB has nothing to expand.
func plainB() util.Point { return util.Point{X: 1} }

func main() {
	hello(parse!("1"))
	_ = plainB().Equal(util.Point{})
}
