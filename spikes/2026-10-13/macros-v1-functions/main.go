package main

import (
	"fmt"

	"spike/gocpp/macro"
	"spike/util"
)

var evaluated = 0

func expensive() string { evaluated++; return "computed" }

// Call sites AFTER the front-end rewrite. The user wrote:
//
//	x := #util.Dbg(a * 2)
//	#util.Assert(x > 0, "x must be positive: "+expensive())
var gp_site_1 = macro.Site{File: "main.gp", Line: 9, Func: "main.main", Args: []string{"a * 2"}}
var gp_site_2 = macro.Site{File: "main.gp", Line: 10, Func: "main.main", Args: []string{"x > 0", `"x must be positive: "+expensive()`}}
var gp_site_3 = macro.Site{File: "main.gp", Line: 13, Func: "main.main", Args: []string{"x < 0", `"x must be negative: "+expensive()`}}

func main() {
	a := 21
	x := util.Dbg(&gp_site_1, a*2)
	util.Assert(&gp_site_2, x > 0, macro.Lazy[string](func() string { return "x must be positive: " + expensive() }))
	fmt.Printf("x=%d (type %T), lazy message evaluated %d times\n", x, x, evaluated)

	defer func() { fmt.Println("recovered:", recover()) }()
	util.Assert(&gp_site_3, x < 0, macro.Lazy[string](func() string { return "x must be negative: " + expensive() }))
}
