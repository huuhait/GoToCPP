package main

import "./errmacro.h"

func main() {
	n := 3
	_ = errmacro.POINT_X(C.int(n))
}
