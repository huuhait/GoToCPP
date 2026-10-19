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
fmt.Println("e1")
L2:
for range it0(2) {
	fmt.Println("body", 3)
L5:
	for i6 := 0; i6 < 1; i6++ {
		fmt.Println("body", 6, i6)
		if acc%2 == 1 {
			continue L5
		}
		fmt.Println("e8", i6)
	}
	if acc%2 == 1 {
		break L2
	}
}
	res = acc
	return
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
for x10 := range it1(3) {
	fmt.Println("body", 10, x10)
	defer fmt.Println("defer", 12, acc)
}
L13:
for n14 := range 2 {
	fmt.Println("body", 14, n14)
	fmt.Println("e16", n14)
	fmt.Println("e17", n14)
	if n14%2 == 1 {
		fmt.Println("e19")
		continue L13
		fmt.Println("e20")
	} else {
		fmt.Println("e22")
		for x24 := range it1(1) {
			fmt.Println("body", 24, x24)
			for x27 := range it1(2) {
				fmt.Println("body", 27, x27)
				acc++
			}
			for x30 := range it1(3) {
				fmt.Println("body", 30, x30)
				if x30 > 1 {
					continue L13
				}
			}
			for x33 := range it1(1) {
				fmt.Println("body", 33, x33)
				fmt.Println("e35", x24, x33)
			}
		}
L36:
		for x37 := range it1(3) {
			fmt.Println("body", 37, x37)
			if x37%2 == 0 {
				break L36
			}
			fmt.Println("e39", x37)
		}
	}
}
fmt.Println("e40")
if acc == 0 {
	for n43 := range 2 {
		fmt.Println("body", 43, n43)
		defer fmt.Println("defer", 45, acc)
	}
	if acc%3 == 1 {
		fmt.Println("e46")
	}
} else {
	fmt.Println("e48")
	if acc == 0 {
		return acc + 32
		if acc == 3 {
			return acc + 75
		}
	} else {
		if acc == 1 {
			fmt.Println("e52")
		} else {
L54:
			for range it1(2) {
				fmt.Println("body", 55)
				if acc%2 == 0 {
					continue L54
				}
			}
		}
		goto top
	}
	acc++
}
end:
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
for x58 := range it1(1) {
	fmt.Println("body", 58, x58)
	fmt.Println("e60", x58)
}
L61:
for range it0(1) {
	fmt.Println("body", 62)
	if acc%3 == 0 {
		break L61
	}
	if acc%2 == 1 {
		goto end
	}
	for n65 := range 2 {
		fmt.Println("body", 65, n65)
		fmt.Println("e67", n65)
	}
}
end:
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e68")
fmt.Println("e69")
fmt.Println("e70")
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
for x72 := range it1(2) {
	fmt.Println("body", 72, x72)
	for x75 := range it1(1) {
		fmt.Println("body", 75, x75)
		fmt.Println("e77", x72, x75)
		for n79 := range 2 {
			fmt.Println("body", 79, n79)
			if x72%3 == 0 {
				if acc%2 == 0 {
					continue
				}
			} else {
				if acc == 1 {
					goto end
				}
			}
		}
		if x72 == 3 {
			fmt.Println("e83", x72, x75)
			continue
		}
	}
}
fmt.Println("e84")
end:
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
for x86 := range it1(2) {
	fmt.Println("body", 86, x86)
L88:
	for x89 := range it1(2) {
		fmt.Println("body", 89, x89)
		if x89 == 2 {
			continue
		}
		continue L88
	}
}
fmt.Println("e91")
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
// e1
// body 3
// body 6 0
// e8 0
// body 3
// body 6 0
// e8 0
// it0 done 2
// f0 = 0
// body 10 0
// body 10 1
// body 10 2
// it1 done 3
// body 14 0
// e16 0
// e17 0
// e22
// body 24 0
// body 27 0
// body 27 1
// it1 done 2
// body 30 0
// body 30 1
// body 30 2
// it1 stopped at 2
// it1 done 3
// it1 stopped at 0
// it1 done 1
// body 14 1
// e16 1
// e17 1
// e19
// e40
// e48
// body 55
// body 55
// it1 done 2
// body 10 0
// body 10 1
// body 10 2
// it1 done 3
// body 14 0
// e16 0
// e17 0
// e22
// body 24 0
// body 27 0
// body 27 1
// it1 done 2
// body 30 0
// body 30 1
// body 30 2
// it1 stopped at 2
// it1 done 3
// it1 stopped at 0
// it1 done 1
// body 14 1
// e16 1
// e17 1
// e19
// e40
// e48
// body 55
// body 55
// it1 done 2
// body 10 0
// body 10 1
// body 10 2
// it1 done 3
// body 14 0
// e16 0
// e17 0
// e22
// body 24 0
// body 27 0
// body 27 1
// it1 done 2
// body 30 0
// body 30 1
// body 30 2
// it1 stopped at 2
// it1 done 3
// it1 stopped at 0
// it1 done 1
// body 14 1
// e16 1
// e17 1
// e19
// e40
// e48
// body 55
// body 55
// it1 done 2
// defer 12 4
// defer 12 4
// defer 12 4
// defer 12 2
// defer 12 2
// defer 12 2
// defer 12 0
// defer 12 0
// defer 12 0
// deferred acc 6
// f1 = 6
// body 58 0
// e60 0
// it1 done 1
// body 62
// it0 stopped at 0
// it0 done 1
// f2 = 0
// e68
// e69
// e70
// deferred acc 0
// f3 = 0
// body 72 0
// body 75 0
// e77 0 0
// body 79 0
// body 79 1
// it1 done 1
// body 72 1
// body 75 0
// e77 1 0
// body 79 0
// body 79 1
// it1 done 1
// it1 done 2
// e84
// f4 = 0
// body 86 0
// body 89 0
// body 89 1
// it1 done 2
// body 86 1
// body 89 0
// body 89 1
// it1 done 2
// it1 done 2
// e91
// f5 = 0
