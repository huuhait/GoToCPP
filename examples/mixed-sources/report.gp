package main

import "fmt"

// show prints a label and numbers on one line. It lives in a second gocpp
// file: a package is a folder, so report.gp is built with main.gp and the
// C and C++ files.
func show(label string, values ...int) {
	fmt.Print(label, ":")
	for _, v := range values {
		fmt.Print(" ", v)
	}
	fmt.Println()
}
