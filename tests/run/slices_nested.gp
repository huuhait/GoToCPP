// phase: 03

// Slices of slices: rows are separate arrays (or parts of one), appending to a row, replacing
// a row by a part of itself (ss[0] = ss[0][1:]), and assignments whose value is reached
// through the target (the old value is read before the target changes).
package main

import "fmt"

func show(s []int) {
	fmt.Print(" [")
	for i, v := range s {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(v)
	}
	fmt.Print("]")
}

func main() {
	ss := [][]int{{1, 2, 3}, {4}, nil}
	ss[0] = ss[0][1:]
	ss[1] = append(ss[1], ss[0]...)
	ss[2] = ss[1][:1]
	ss[2][0] = 40
	for _, row := range ss {
		show(row)
	}
	fmt.Println()
	ss = ss[1:]
	ss[0] = ss[0][:0]
	ss = append(ss, ss...)
	for _, row := range ss {
		show(row)
	}
	fmt.Println(len(ss))

	tri := make([][]int, 4)
	for i := range tri {
		tri[i] = make([]int, i+1)
		tri[i][0], tri[i][i] = 1, 1
		for j := 1; j < i; j++ {
			tri[i][j] = tri[i-1][j-1] + tri[i-1][j]
		}
	}
	for _, row := range tri {
		show(row)
	}
	fmt.Println()

	deep := [][][]string{{{"a"}}}
	deep[0] = append(deep[0], []string{"b", "c"})
	deep[0][1] = deep[0][1][1:]
	deep = append(deep, deep[0])
	deep[1][0][0] = "z"
	fmt.Println(len(deep), len(deep[0]), deep[0][1][0], deep[0][0][0])

	only := [][]int{{1, 2}}
	only = only[0:1]
	inner := only[0]
	only = nil
	show(inner)
	fmt.Println()
}

// Output:
//  [2 3] [40 2 3] [40]
//  [] [40] [] [40]4
//  [1] [1 1] [1 2 1] [1 3 3 1]
// 2 2 c z
//  [1 2]
