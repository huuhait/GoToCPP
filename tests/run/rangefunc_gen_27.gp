// phase: 07

package main

import "fmt"

// it1 yields 0..n-1; the body may stop it.
func it1(n int) func(yield func(int) bool) {
	return func(yield func(int) bool) {
		defer fmt.Println("it1 done", n)
		for i := 0; i < n; i++ {
			if !yield(i) {
				fmt.Println("it1 stopped at", i)
				return
			}
		}
	}
}

// it2 yields an int and a string.
func it2(n int) func(yield func(int, string) bool) {
	return func(yield func(int, string) bool) {
		defer fmt.Println("it2 done", n)
		for i := 0; i < n; i++ {
			if !yield(i*10, fmt.Sprint("s", i)) {
				fmt.Println("it2 stopped at", i)
				return
			}
		}
	}
}

// it0 yields nothing but calls the body n times.
func it0(n int) func(yield func() bool) {
	return func(yield func() bool) {
		defer fmt.Println("it0 done", n)
		for i := 0; i < n; i++ {
			if !yield() {
				fmt.Println("it0 stopped at", i)
				return
			}
		}
	}
}

func f0() int {
	acc, steps := 0, 0
	_ = steps
if acc%3 == 0 {
	for i3 := 0; i3 < 1; i3++ {
		fmt.Println("body", 3, i3)
		if i3 == 1 {
			return acc + 55
		}
		if acc > 0 {
			fmt.Println("e6")
		} else {
			for x9 := range it1(1) {
				fmt.Println("body", 9, x9)
				if x9 > 1 {
					fmt.Println("e11", x9)
					continue
				}
			}
		}
		for x13 := range it1(2) {
			fmt.Println("body", 13, x13)
			return acc + 6
		}
	}
	if acc%3 == 1 {
		goto b1
	}
L15:
	for x16, s16 := range it2(2) {
		fmt.Println("body", 16, x16)
		fmt.Println("body", 16, s16)
		for range it0(1) {
			fmt.Println("body", 19)
			if x16%3 == 1 {
				if acc == 1 {
					return acc + 58
				}
			} else {
				if acc > 0 {
					return acc + 89
				}
			}
			if x16 > 0 {
				continue
			}
			defer fmt.Println("defer", 23, acc)
		}
L24:
		for x25 := range it1(3) {
			fmt.Println("body", 25, x25)
			break L24
			for i28 := 0; i28 < 3; i28++ {
				fmt.Println("body", 28, i28)
				if x25 == 1 {
					break L15
				}
			}
		}
L30:
		for x31 := range it1(3) {
			fmt.Println("body", 31, x31)
L33:
			switch x31 % 3 {
case 0:
				if x16%2 == 1 {
					fmt.Println("e34", x16, x31)
					break L15
				}
case 1:
				break L33
default:
				fmt.Println("e35", x16, x31)
				if acc%2 == 1 {
					continue L30
				}
			}
L36:
			switch x31 % 3 {
case 0:
				if x31 == 1 {
					break L36
				}
case 1:
				fmt.Println("e37", x16, x31)
				if x31 > 1 {
					continue L30
				}
			}
		}
	}
b1:
} else {
L39:
	for n40 := range 2 {
		fmt.Println("body", 40, n40)
		acc += n40 % 5
		for range it0(1) {
			fmt.Println("body", 43)
			if n40 == 2 {
				break L39
			}
		}
		fmt.Println("e45", n40)
	}
}
goto end
fmt.Println("e46")
end:
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
L47:
for range it1(3) {
	fmt.Println("body", 48)
	if acc == 1 {
		fmt.Println("e50")
		break L47
	}
	acc++
}
for n52 := range 3 {
	fmt.Println("body", 52, n52)
	fmt.Println("e54", n52)
}
L55:
for acc = range it1(3) {
	fmt.Println("body", 56)
L58:
	for acc = range it1(1) {
		fmt.Println("body", 59)
		for x62 := range it1(2) {
			fmt.Println("body", 62, x62)
			fmt.Println("e64", x62)
			if acc > 3 {
				continue L58
			}
		}
		for x66, s66 := range it2(2) {
			fmt.Println("body", 66, x66)
			fmt.Println("body", 66, s66)
			goto end
			for acc = range it1(1) {
				fmt.Println("body", 69)
				if x66 > 0 {
					if acc == 1 {
						fmt.Println("e72")
						goto top
					}
				} else {
					if acc%2 == 0 {
						break
					}
				}
			}
			acc += x66 % 5
		}
	}
	if acc%3 == 1 {
		fmt.Println("e75")
		continue L55
		if acc%3 == 0 {
			return acc + 22
		}
	} else {
		fmt.Println("e77")
		for acc = range it1(1) {
			fmt.Println("body", 79)
L81:
			for i82 := 0; i82 < 3; i82++ {
				fmt.Println("body", 82, i82)
				if i82 == 1 {
					continue L81
				}
			}
			for x85, s85 := range it2(1) {
				fmt.Println("body", 85, x85)
				fmt.Println("body", 85, s85)
				if acc > 0 {
					fmt.Println("e87", x85)
					goto b57
				}
			}
		}
		if acc%2 == 0 {
			continue
		}
	}
	continue
b57:
}
if acc%2 == 0 {
	fmt.Println("e88")
}
end:
	return acc
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for range it1(2) {
	fmt.Println("body", 90)
	if acc > 3 {
		fmt.Println("e92")
		return acc + 63
	}
	defer fmt.Println("defer", 93, acc)
}
switch acc % 3 {
case 0:
	for x96 := range it1(2) {
		fmt.Println("body", 96, x96)
		fmt.Println("e98", x96)
	}
case 1:
	defer fmt.Println("defer", 99, acc)
default:
	fmt.Println("e100")
	fmt.Println("e101")
}
defer fmt.Println("defer", 102, acc)
	res = acc
	return
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e103")
for x105 := range it1(3) {
	fmt.Println("body", 105, x105)
	if x105%3 == 0 {
		fmt.Println("e107", x105)
		goto top
	}
}
end:
	res = acc
	return
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc%2 == 1 {
	fmt.Println("e108")
}
fmt.Println("e109")
	res = acc
	return
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
if acc > 1 {
	fmt.Println("e110")
}
fmt.Println("e111")
fmt.Println("e112")
goto end
end:
	res = acc
	return
}


func run(name string, f func() int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "recovered", r)
		}
	}()
	fmt.Println(name, "=", f())
}

func main() {
	run("f0", f0)
	run("f1", f1)
	run("f2", f2)
	run("f3", f3)
	run("f4", f4)
	run("f5", f5)
}

// Output:
// body 3 0
// body 9 0
// it1 done 1
// body 13 0
// it1 stopped at 0
// it1 done 2
// f0 = 6
// body 48
// body 48
// e50
// it1 stopped at 1
// it1 done 3
// body 52 0
// e54 0
// body 52 1
// e54 1
// body 52 2
// e54 2
// body 56
// body 59
// body 62 0
// e64 0
// body 62 1
// e64 1
// it1 done 2
// body 66 0
// body 66 s0
// it2 stopped at 0
// it2 done 2
// it1 stopped at 0
// it1 done 1
// it1 stopped at 0
// it1 done 3
// deferred acc 0
// f1 = 0
// body 90
// body 90
// it1 done 2
// body 96 0
// e98 0
// body 96 1
// e98 1
// it1 done 2
// defer 102 0
// defer 93 0
// defer 93 0
// deferred acc 0
// f2 = 0
// e103
// body 105 0
// e107 0
// it1 stopped at 0
// it1 done 3
// e103
// body 105 0
// e107 0
// it1 stopped at 0
// it1 done 3
// e103
// body 105 0
// e107 0
// it1 stopped at 0
// it1 done 3
// deferred acc 0
// f3 = 0
// e109
// f4 = 0
// e111
// e112
// f5 = 0
