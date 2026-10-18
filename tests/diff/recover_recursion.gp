// phase: 05

package main

import "fmt"

func descend(n int) int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("level", n, "recovered", r)
			panic(fmt.Sprint("level ", n))
		}
	}()
	if n == 0 {
		panic("bottom")
	}
	return descend(n-1) + 1
}

func count(n int) (total int) {
	defer func() { total += n }()
	if n == 0 {
		return 0
	}
	return count(n - 1)
}

func safeDescend(n int) (res string) {
	defer func() {
		res = fmt.Sprint("top recovered: ", recover())
	}()
	descend(n)
	return "no panic"
}

func main() {
	fmt.Println(safeDescend(3))
	fmt.Println(count(10))
}
