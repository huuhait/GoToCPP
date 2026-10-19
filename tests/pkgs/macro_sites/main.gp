// phase: 10

// The built-in macros across the packages of a module (Phase 10 step 10.8): #file() is the path
// from the module root, #func() has the import path of its package, #embed and //go:embed read
// the files next to the source file that has them.

package main

import (
	"fmt"

	"example.com/sites/util"
	"example.com/sites/util/deep"
)

func main() {
	fmt.Println(#file(), #func())
	fmt.Println(util.Where())
	fmt.Println((&util.Widget{}).Where())
	fmt.Println(util.Closure())
	fmt.Println(util.Gen(1), util.Gen("s"))
	fmt.Println(util.Init)
	fmt.Println(util.Text(6, 7))
	fmt.Print(util.Note, util.EmbedHere())
	fmt.Println(deep.Where())
	fmt.Print(#embed("data/root.txt"))
}

// Output:
// main.gp main.main
// util/util.gp:13 example.com/sites/util.Where
// util/util.gp example.com/sites/util.(*Widget).Where
// example.com/sites/util.Closure.func1
// example.com/sites/util.Gen[...] example.com/sites/util.Gen[...]
// example.com/sites/util.init
// a * b
// util data
// util data
// util/deep/deep.gp example.com/sites/util/deep.Where
// root data
