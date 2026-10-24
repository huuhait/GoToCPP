package main

import (
	"fmt"
	"strings"
)

func Show(gp_file string, gp_line int, gp_func string, gp_text_v string, v int) string {
	return fmt.Sprint(gp_file, ":", gp_line, " ", gp_func, " ", gp_text_v, "=", v)
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
	fmt.Println(Show("umacro_method_name.gp", 37, "main.main", "4 + 1", 4+1))
}
