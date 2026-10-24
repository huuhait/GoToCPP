// phase: 09

// A method named like a macro of its package (step 10.13 review): the macro Show and the method Rec.Show, whose
// parameters have the types of the macro's hidden arguments (string, int, string, then the string of #text). Only
// the macro is a macro: gen.macroSig takes no function with a receiver, so the method's parameters are plain
// strings, which the calls give values that are not constants (temporaries, a variable), also through a method value
// and a method expression. The twin (testdata/macro_twin/umacro_method_name.go) is the macro written out.

package main

import (
	"fmt"
	"strings"
)

macro Show(v int) string {
	return fmt.Sprint(#file(), ":", #line(), " ", #func(), " ", #text(v), "=", v)
}

type Rec struct{ name string }

func (r Rec) Show(file string, line int, fn string, rest string) string {
	file += "/" + r.name
	return fmt.Sprint(file, ":", line, " ", fn, " ", rest)
}

func main() {
	r := Rec{name: "rec"}
	s := "file"
	for i := range 2 {
		s += "!"
		fmt.Println(r.Show(s+".gp", i, strings.ToUpper(s), fmt.Sprint(i*10)))
	}
	f := r.Show
	fmt.Println(f(s, 1, s+"?", strings.Repeat("ab", 3)))
	fmt.Println(Rec.Show(Rec{name: "expr"}, "a"+s, 2, "b", "c"))
	fmt.Println(#Show(4 + 1))
}

// Output:
// file!.gp/rec:0 FILE! 0
// file!!.gp/rec:1 FILE!! 10
// file!!/rec:1 file!!? ababab
// afile!!/expr:2 b c
// umacro_method_name.gp:37 main.main 4 + 1=5
