// phase: 05

package main

import "fmt"

func risky(i int) int {
	switch i % 4 {
	case 1:
		panic(fmt.Sprint("panic at ", i))
	case 2:
		var m map[int]int
		m[i] = i
	case 3:
		a := []int{1}
		return a[i]
	}
	return i * 10
}

func try(i int) (v int, msg string) {
	defer func() {
		if r := recover(); r != nil {
			v = -i
			msg = fmt.Sprint(r)
		}
	}()
	return risky(i), "ok"
}

func main() {
	for i := 0; i < 8; i++ {
		v, msg := try(i)
		fmt.Println(i, v, msg)
	}
}
