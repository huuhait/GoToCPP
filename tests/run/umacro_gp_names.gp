// phase: 05

// A macro that calls ordinary functions whose parameters are named like the hidden arguments of a macro (step 10.13
// review): only a macro's own hidden arguments are gp::static_view in C++, so these calls take plain strings, constants or
// not. A function whose first three parameters are (gp_file string, gp_line int, gp_func string) is a function, not
// a macro: the loader's list of macros (Package.Macros) says which is which, not the names. The twin
// (testdata/macro_twin/umacro_gp_names.go) is the macro written out.

package main

import "fmt"

func where(gp_file string, gp_line int, gp_func string, rest string) string {
	return fmt.Sprint(gp_file, ":", gp_line, " ", gp_func, " ", rest)
}

func tag(gp_text_x string) string { return "<" + gp_text_x + ">" }

macro Report(v int) string {
	label := "v=" + fmt.Sprint(v)
	// the place of the use, as constants and as a variable
	a := where(#file(), #line(), #func(), label)
	f := #file()
	y := tag(#text(v))
	y = y + tag(f)
	return a + " " + y
}

func main() {
	n := 3
	fmt.Println(#Report(n * 2))
	fmt.Println(#Report(7))
}

// Output:
// umacro_gp_names.gp:31 main.main v=6 <n * 2><umacro_gp_names.gp>
// umacro_gp_names.gp:32 main.main v=7 <7><umacro_gp_names.gp>
