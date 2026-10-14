// phase: 02

// switch with a tag, without a tag, with an init statement, with several
// values in one case, with default first, in the middle and last, on
// strings and on bools.
package main

import "fmt"

func kind(n int) string {
	switch n {
	case 0:
		return "zero"
	case 1, 3, 5, 7, 9:
		return "odd digit"
	default:
		return "other"
	case 2, 4, 6, 8:
		return "even digit"
	}
}

func grade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	}
	return "F"
}

func lang(s string) int {
	switch s {
	default:
		return -1
	case "go":
		return 1
	case "c++", "cpp":
		return 2
	}
}

func main() {
	for i := range 11 {
		fmt.Print(kind(i), "|")
	}
	fmt.Println("|")
	fmt.Println(grade(95), grade(85), grade(75), grade(10))
	fmt.Println(lang("go"), lang("cpp"), lang("c++"), lang("rust"))

	switch x := 5 * 3; {
	case x > 10:
		fmt.Println("big", x)
	case x > 5:
		fmt.Println("medium", x)
	}

	switch x := 7; x % 3 {
	case 0:
		fmt.Println("div by 3")
	case 1:
		fmt.Println("rem 1")
	}

	ok := true
	switch ok {
	case false:
		fmt.Println("false")
	case true:
		fmt.Println("true")
	}

	switch {
	}
	switch 3 {
	default:
		fmt.Println("only default")
	}

	for i := range 4 {
		switch i {
		case 1:
			continue
		case 2:
			break
		}
		fmt.Print(i, " ")
	}
	fmt.Println("|")
}

// Output:
// zero|odd digit|even digit|odd digit|even digit|odd digit|even digit|odd digit|even digit|odd digit|other||
// A B C F
// 1 2 2 -1
// big 15
// rem 1
// true
// only default
// 0 2 3 |
