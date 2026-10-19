// phase: 07

package main

import "fmt"

func Cnt(name string, n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println(name, "done")
		for i := range n {
			if !yield(i) {
				fmt.Println(name, "stopped", i)
				return
			}
		}
	}
}

func main() {
outer:
	for a := range Cnt("A", 3) {
	middle:
		for b := range Cnt("B", 3) {
			for c := range Cnt("C", 3) {
				fmt.Println(a, b, c)
				switch {
				case a == 0 && b == 0 && c == 1:
					continue middle
				case a == 0 && b == 1 && c == 0:
					continue outer
				case a == 1 && b == 0 && c == 0:
					continue
				case a == 1 && b == 1 && c == 1:
					break middle
				case a == 2 && b == 0 && c == 2:
					break outer
				}
			}
			fmt.Println("after c", a, b)
		}
		fmt.Println("after b", a)
	}
	fmt.Println("end")
}

// Output:
// 0 0 0
// 0 0 1
// C stopped 1
// C done
// 0 1 0
// C stopped 0
// C done
// B stopped 1
// B done
// 1 0 0
// 1 0 1
// 1 0 2
// C done
// after c 1 0
// 1 1 0
// 1 1 1
// C stopped 1
// C done
// B stopped 1
// B done
// after b 1
// 2 0 0
// 2 0 1
// 2 0 2
// C stopped 2
// C done
// B stopped 0
// B done
// A stopped 2
// A done
// end
