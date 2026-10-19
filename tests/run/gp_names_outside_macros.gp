// phase: 05

// Names that start with gp_ are reserved only inside macros (docs/spec/language.md, "Rules (v1)"); anywhere else they are
// ordinary names, also the ones the loader gives to the hidden arguments of a macro (gp_file, gp_line, gp_func,
// gp_text_x). The lowering must treat them as plain strings (step 10.13 review): no gp::static_view for them.
// The program is valid Go: real Go runs this file.

package main

import "fmt"

// Parameters named like the hidden ones, called with values that are not constants.
func describe(gp_file string, n int) string {
	return gp_file + fmt.Sprint(n)
}

// The first three parameters have the types of a macro's hidden ones, but this is a function.
func where(gp_file string, gp_line int, gp_func string, rest string) string {
	return fmt.Sprint(gp_file, ":", gp_line, " ", gp_func, " ", rest)
}

type Rec struct {
	gp_file string
	gp_func string
}

func (gp_text_x Rec) Show(gp_text_y string) string {
	return gp_text_x.gp_file + "/" + gp_text_x.gp_func + "/" + gp_text_y
}

func main() {
	s := "name"
	for i := range 2 {
		s += "!"
		fmt.Println(describe(s, i))
	}

	gp_text_x := "a"
	gp_text_x = "t"
	gp_text_x += "u"
	fmt.Println(gp_text_x)

	var gp_func string
	gp_func = "f" + s
	fmt.Println(gp_func)

	var gp_file = "x"
	fn := func() string {
		gp_file = gp_file + "y"
		var gp_text_z string
		gp_text_z = gp_file
		return gp_text_z
	}
	fmt.Println(fn(), gp_file)

	r := Rec{gp_file: "file", gp_func: s}
	fmt.Println(r.Show(gp_text_x))
	r.gp_file = gp_text_x
	fmt.Println(r)

	line := len(s)
	fmt.Println(where(s, line, gp_func, "end"))
	fmt.Println(where("lit", 3, "lit", s))
}

// Output:
// name!0
// name!!1
// tu
// fname!!
// xy xy
// file/name!!/tu
// {tu name!!}
// name!!:6 fname!! end
// lit:3 lit name!!
