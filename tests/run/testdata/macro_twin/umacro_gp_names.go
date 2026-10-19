package main

import "fmt"

func where(gp_file string, gp_line int, gp_func string, rest string) string {
	return fmt.Sprint(gp_file, ":", gp_line, " ", gp_func, " ", rest)
}

func tag(gp_text_x string) string { return "<" + gp_text_x + ">" }

func Report(gp_file string, gp_line int, gp_func string, gp_text_v string, v int) string {
	label := "v=" + fmt.Sprint(v)
	a := where(gp_file, gp_line, gp_func, label)
	f := gp_file
	y := tag(gp_text_v)
	y = y + tag(f)
	return a + " " + y
}

func main() {
	n := 3
	fmt.Println(Report("umacro_gp_names.gp", 31, "main.main", "n * 2", n*2))
	fmt.Println(Report("umacro_gp_names.gp", 32, "main.main", "7", 7))
}
