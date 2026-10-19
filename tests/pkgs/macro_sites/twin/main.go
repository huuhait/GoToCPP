package main

import (
	_ "embed"
	"fmt"

	"example.com/sites/util"
	"example.com/sites/util/deep"
)

//go:embed data/root.txt
var rootData string

func main() {
	fmt.Println("main.gp", "main.main")
	fmt.Println(util.Where())
	fmt.Println((&util.Widget{}).Where())
	fmt.Println(util.Closure())
	fmt.Println(util.Gen(1), util.Gen("s"))
	fmt.Println(util.Init)
	fmt.Println(util.Text(6, 7))
	fmt.Print(util.Note, util.EmbedHere())
	fmt.Println(deep.Where())
	fmt.Print(rootData)
}
