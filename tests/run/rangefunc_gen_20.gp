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

func f0() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
if acc > 0 {
	goto end
}
for acc = range it1(2) {
	fmt.Println("body", 2)
	if acc%2 == 1 {
		goto top
	}
}
end:
	res = acc
	return
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
L4:
switch acc % 3 {
case 0:
	if acc%3 == 2 {
		goto end
	}
	for i6 := 0; i6 < 1; i6++ {
		fmt.Println("body", 6, i6)
		if acc%3 == 0 {
			fmt.Println("e8", i6)
			goto end
		}
		for x10, s10 := range it2(3) {
			fmt.Println("body", 10, x10)
			fmt.Println("body", 10, s10)
			if i6 == 1 {
				fmt.Println("e13")
			} else {
				fmt.Println("e15")
			}
			for x17 := range it1(2) {
				fmt.Println("body", 17, x17)
				fmt.Println("e19", i6, x10, x17)
			}
			fmt.Println("e20", i6, x10)
		}
		break L4
	}
case 1:
	if acc%2 == 0 {
		if acc > 1 {
			break L4
		}
		for x23, s23 := range it2(3) {
			fmt.Println("body", 23, x23)
			fmt.Println("body", 23, s23)
			acc++
			fmt.Println("e25", x23)
			if x23 > 1 {
				if acc%3 == 1 {
					break L4
				}
			} else {
				if acc%3 == 0 {
					fmt.Println("e28")
					break L4
				}
			}
		}
	} else {
		if acc == 0 {
			break
		}
		fmt.Println("e30")
		fmt.Println("e31")
	}
default:
	fmt.Println("e32")
}
if acc%3 == 1 {
L34:
	for range it1(2) {
		fmt.Println("body", 35)
L37:
		for n38 := range 2 {
			fmt.Println("body", 38, n38)
			for x41 := range it1(3) {
				fmt.Println("body", 41, x41)
				if x41%3 == 2 {
					fmt.Println("e43", n38, x41)
					break L37
				}
			}
			for x45 := range it1(1) {
				fmt.Println("body", 45, x45)
				fmt.Println("e47", n38, x45)
			}
			for x49 := range it1(3) {
				fmt.Println("body", 49, x49)
				if x49%3 == 0 {
					fmt.Println("e51", n38, x49)
					break
				}
			}
		}
		for n53 := range 3 {
			fmt.Println("body", 53, n53)
			for x56 := range it1(1) {
				fmt.Println("body", 56, x56)
				fmt.Println("e58", n53, x56)
			}
		}
		for x60 := range it1(1) {
			fmt.Println("body", 60, x60)
			for x63 := range it1(3) {
				fmt.Println("body", 63, x63)
				if x63 > 3 {
					if acc%3 == 1 {
						res = acc + 43
						return
					}
				} else {
					if acc == 0 {
						break L34
					}
				}
			}
L67:
			switch x60 % 3 {
case 0:
				if x60 > 2 {
					break L67
				}
				acc += x60 % 5
case 1:
				fmt.Println("e68", x60)
			}
		}
	}
} else {
	if acc > 1 {
		res = acc + 77
		return
	}
}
end:
	res = acc
	return
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
L70:
for x71 := range it1(3) {
	fmt.Println("body", 71, x71)
	for x74 := range it1(3) {
		fmt.Println("body", 74, x74)
		acc++
		if x74 > 3 {
			fmt.Println("e76", x71, x74)
			continue L70
		}
	}
}
if acc%3 == 2 {
	fmt.Println("e78")
	fmt.Println("e79")
} else {
	if acc%2 == 1 {
L82:
		for i83 := 0; i83 < 1; i83++ {
			fmt.Println("body", 83, i83)
			continue L82
			fmt.Println("e85", i83)
		}
L86:
		for acc = range it1(2) {
			fmt.Println("body", 87)
			for x90 := range it1(1) {
				fmt.Println("body", 90, x90)
				acc += x90 % 5
			}
			for range it0(1) {
				fmt.Println("body", 93)
				if acc == 0 {
					break L86
				}
			}
		}
	} else {
		for x97 := range it1(1) {
			fmt.Println("body", 97, x97)
			fmt.Println("e99", x97)
		}
L100:
		for x101 := range it1(1) {
			fmt.Println("body", 101, x101)
			fmt.Println("e103", x101)
			fmt.Println("e104", x101)
			break L100
		}
	}
	goto end
}
for x106 := range it1(3) {
	fmt.Println("body", 106, x106)
	fmt.Println("e108", x106)
}
if acc%2 == 1 {
	fmt.Println("e109")
}
end:
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
for x111, s111 := range it2(3) {
	fmt.Println("body", 111, x111)
	fmt.Println("body", 111, s111)
	defer fmt.Println("defer", 113, acc)
}
fmt.Println("e114")
fmt.Println("e115")
for x117, s117 := range it2(3) {
	fmt.Println("body", 117, x117)
	fmt.Println("body", 117, s117)
	for n120 := range 1 {
		fmt.Println("body", 120, n120)
		for n123 := range 2 {
			fmt.Println("body", 123, n123)
			defer fmt.Println("defer", 125, acc)
		}
	}
}
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e126")
L127:
switch acc % 3 {
case 0:
	for x129 := range it1(3) {
		fmt.Println("body", 129, x129)
		break L127
	}
case 1:
	fmt.Println("e131")
}
fmt.Println("e132")
L133:
for x134 := range it1(3) {
	fmt.Println("body", 134, x134)
	continue L133
}
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e136")
fmt.Println("e137")
if acc%3 == 2 {
	fmt.Println("e138")
}
if acc == 3 {
	goto end
}
end:
	return acc
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
// body 2
// body 2
// it1 stopped at 1
// it1 done 2
// deferred acc 1
// f0 = 1
// body 6 0
// e8 0
// f1 = 0
// body 71 0
// body 74 0
// body 74 1
// body 74 2
// it1 done 3
// body 71 1
// body 74 0
// body 74 1
// body 74 2
// it1 done 3
// body 71 2
// body 74 0
// body 74 1
// body 74 2
// it1 done 3
// it1 done 3
// body 83 0
// body 87
// body 90 0
// it1 done 1
// body 93
// it0 stopped at 0
// it0 done 1
// it1 stopped at 0
// it1 done 2
// deferred acc 0
// f2 = 0
// body 111 0
// body 111 s0
// body 111 10
// body 111 s1
// body 111 20
// body 111 s2
// it2 done 3
// e114
// e115
// body 117 0
// body 117 s0
// body 120 0
// body 123 0
// body 123 1
// body 117 10
// body 117 s1
// body 120 0
// body 123 0
// body 123 1
// body 117 20
// body 117 s2
// body 120 0
// body 123 0
// body 123 1
// it2 done 3
// defer 125 0
// defer 125 0
// defer 125 0
// defer 125 0
// defer 125 0
// defer 125 0
// defer 113 0
// defer 113 0
// defer 113 0
// f3 = 0
// e126
// body 129 0
// it1 stopped at 0
// it1 done 3
// e132
// body 134 0
// body 134 1
// body 134 2
// it1 done 3
// deferred acc 0
// f4 = 0
// e136
// e137
// f5 = 0
