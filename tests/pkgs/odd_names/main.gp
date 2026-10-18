// phase: 06

// Import paths and package names that differ: a folder util with package utils, a folder with upper case letters, a dash and a dot, a module path with a dot and a dash, packages called new and class-like names (C++ keywords), and package variables and constants named like C++ keywords.

package main

import (
	"fmt"

	utils "example.com/my-mod.v2/internal/util"
	"example.com/my-mod.v2/pkg/Mixed-Case.v2"
	nw "example.com/my-mod.v2/pkg/new"
)

func main() {
	utils.Hello()
	fmt.Println(utils.Wrap("w").Get(), mixed.Name())
	nw.Class().Delete()
	fmt.Println(nw.Template())
}

// Output:
// hello from utils
// w mixed
// delete 3
// tmpl
