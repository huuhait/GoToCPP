// phase: 07

package main

import "fmt"

func Cnt(name string, n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println(name, "done")
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

// Each function has one kind of jump from the deepest of four loops to one of the loops around.

func breakTop() {
A:
	for a := range Cnt("A", 3) {
		for b := range Cnt("B", 2) {
			for c := range Cnt("C", 2) {
				for d := range Cnt("D", 2) {
					fmt.Println("breakTop", a, b, c, d)
					if a == 1 && d == 1 {
						break A
					}
				}
			}
		}
	}
	fmt.Println("breakTop end")
}

func continueTop() {
A:
	for a := range Cnt("A", 3) {
		for b := range Cnt("B", 2) {
			for c := range Cnt("C", 2) {
				for d := range Cnt("D", 2) {
					fmt.Println("continueTop", a, b, c, d)
					if d == 1 {
						continue A
					}
				}
			}
		}
		fmt.Println("not reached")
	}
	fmt.Println("continueTop end")
}

func breakMiddle() {
	for a := range Cnt("A", 2) {
	B:
		for b := range Cnt("B", 3) {
			for c := range Cnt("C", 2) {
				for d := range Cnt("D", 2) {
					fmt.Println("breakMiddle", a, b, c, d)
					if b == 1 {
						break B
					}
				}
			}
		}
		fmt.Println("after B", a)
	}
}

func continueMiddle() {
	for a := range Cnt("A", 2) {
	B:
		for b := range Cnt("B", 2) {
			for c := range Cnt("C", 2) {
				for d := range Cnt("D", 2) {
					fmt.Println("continueMiddle", a, b, c, d)
					if c == 0 && d == 1 {
						continue B
					}
				}
			}
			fmt.Println("not reached")
		}
	}
}

func gotoOut() int {
	n := 0
	for a := range Cnt("A", 2) {
		for b := range Cnt("B", 2) {
			for c := range Cnt("C", 2) {
				n += a + b + c
				if a == 1 && b == 1 && c == 0 {
					goto out
				}
			}
		}
	}
out:
	return n
}

func returnDeep() (r int) {
	for a := range Cnt("A", 2) {
		for b := range Cnt("B", 2) {
			for c := range Cnt("C", 2) {
				for d := range Cnt("D", 2) {
					if a+b+c+d == 3 {
						return a*1000 + b*100 + c*10 + d
					}
				}
			}
		}
	}
	return -1
}

func breakPlainOuter() {
L:
	for i := 0; i < 3; i++ {
		for a := range Cnt("A", 2) {
			for b := range Cnt("B", 2) {
				for c := range Cnt("C", 2) {
					fmt.Println("plain", i, a, b, c)
					if i == 1 && c == 1 {
						break L
					}
					if c == 1 {
						continue L
					}
				}
			}
		}
	}
}

func main() {
	breakTop()
	continueTop()
	breakMiddle()
	continueMiddle()
	fmt.Println(gotoOut())
	fmt.Println(returnDeep())
	breakPlainOuter()
}

// Output:
// breakTop 0 0 0 0
// breakTop 0 0 0 1
// D done
// breakTop 0 0 1 0
// breakTop 0 0 1 1
// D done
// C done
// breakTop 0 1 0 0
// breakTop 0 1 0 1
// D done
// breakTop 0 1 1 0
// breakTop 0 1 1 1
// D done
// C done
// B done
// breakTop 1 0 0 0
// breakTop 1 0 0 1
// D done
// C done
// B done
// A done
// breakTop end
// continueTop 0 0 0 0
// continueTop 0 0 0 1
// D done
// C done
// B done
// continueTop 1 0 0 0
// continueTop 1 0 0 1
// D done
// C done
// B done
// continueTop 2 0 0 0
// continueTop 2 0 0 1
// D done
// C done
// B done
// A done
// continueTop end
// breakMiddle 0 0 0 0
// breakMiddle 0 0 0 1
// D done
// breakMiddle 0 0 1 0
// breakMiddle 0 0 1 1
// D done
// C done
// breakMiddle 0 1 0 0
// D done
// C done
// B done
// after B 0
// breakMiddle 1 0 0 0
// breakMiddle 1 0 0 1
// D done
// breakMiddle 1 0 1 0
// breakMiddle 1 0 1 1
// D done
// C done
// breakMiddle 1 1 0 0
// D done
// C done
// B done
// after B 1
// A done
// continueMiddle 0 0 0 0
// continueMiddle 0 0 0 1
// D done
// C done
// continueMiddle 0 1 0 0
// continueMiddle 0 1 0 1
// D done
// C done
// B done
// continueMiddle 1 0 0 0
// continueMiddle 1 0 0 1
// D done
// C done
// continueMiddle 1 1 0 0
// continueMiddle 1 1 0 1
// D done
// C done
// B done
// A done
// C done
// C done
// B done
// C done
// C done
// B done
// A done
// 9
// D done
// D done
// C done
// D done
// D done
// C done
// B done
// A done
// 111
// plain 0 0 0 0
// plain 0 0 0 1
// C done
// B done
// A done
// plain 1 0 0 0
// plain 1 0 0 1
// C done
// B done
// A done
