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
fmt.Println("e1")
for range it0(1) {
	fmt.Println("body", 3)
	for n6 := range 2 {
		fmt.Println("body", 6, n6)
		fmt.Println("e8", n6)
	}
	acc++
}
	res = acc
	return
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc%3 == 2 {
	fmt.Println("e9")
}
fmt.Println("e10")
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
L11:
for x12 := range it1(2) {
	fmt.Println("body", 12, x12)
	fmt.Println("e14", x12)
	continue L11
	for x16 := range it1(3) {
		fmt.Println("body", 16, x16)
		for x19 := range it1(3) {
			fmt.Println("body", 19, x19)
			fmt.Println("e21", x12, x16, x19)
			fmt.Println("e22", x12, x16, x19)
			if x12 == 0 {
				return acc + 0
			}
		}
		fmt.Println("e23", x12, x16)
		if x12 == 0 {
			continue L11
		}
	}
}
if acc%2 == 1 {
	fmt.Println("e25")
	if acc == 1 {
		return acc + 57
	}
	fmt.Println("e26")
} else {
L28:
	for acc = range it1(2) {
		fmt.Println("body", 29)
		for x32, s32 := range it2(3) {
			fmt.Println("body", 32, x32)
			fmt.Println("body", 32, s32)
			fmt.Println("e34", x32)
			if x32 == 2 {
				goto end
			}
		}
		if acc%3 == 2 {
			if acc > 3 {
				continue L28
			}
		} else {
			defer fmt.Println("defer", 37, acc)
		}
	}
}
for range it1(1) {
	fmt.Println("body", 39)
	fmt.Println("e41")
	for range it1(3) {
		fmt.Println("body", 43)
		fmt.Println("e45")
	}
}
end:
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e46")
acc++
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
for range it1(1) {
	fmt.Println("body", 48)
	acc++
L50:
	for range it0(3) {
		fmt.Println("body", 51)
		if acc%3 == 0 {
			fmt.Println("e53")
			continue L50
		}
		for x55 := range it1(2) {
			fmt.Println("body", 55, x55)
			fmt.Println("e57", x55)
		}
		for n59 := range 1 {
			fmt.Println("body", 59, n59)
			if acc%3 == 2 {
				fmt.Println("e61", n59)
				continue
			}
		}
	}
}
fmt.Println("e62")
if acc == 3 {
	goto end
}
L63:
switch acc % 3 {
case 0:
	for acc = range it1(2) {
		fmt.Println("body", 65)
		if acc == 0 {
			fmt.Println("e67")
			fmt.Println("e68")
		}
		if acc%2 == 1 {
			fmt.Println("e69")
			break L63
		}
		fmt.Println("e70")
	}
	fmt.Println("e71")
case 1:
	for acc = range it1(2) {
		fmt.Println("body", 73)
		for range it0(2) {
			fmt.Println("body", 76)
			for x79 := range it1(1) {
				fmt.Println("body", 79, x79)
				fmt.Println("e81", x79)
			}
			for range it1(3) {
				fmt.Println("body", 83)
				if acc == 3 {
					break
				}
			}
			for x86 := range it1(1) {
				fmt.Println("body", 86, x86)
				if x86%3 == 1 {
					continue
				}
			}
		}
		for x89, s89 := range it2(1) {
			fmt.Println("body", 89, x89)
			fmt.Println("body", 89, s89)
			fmt.Println("e91", x89)
		}
	}
	fmt.Println("e92")
default:
	if acc%2 == 0 {
		res = acc + 81
		return
	}
}
end:
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
if acc > 0 {
	fmt.Println("e93")
}
if acc%3 == 0 {
	fmt.Println("e94")
}
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
// body 6 1
// e8 1
// it0 done 1
// deferred acc 1
// f0 = 1
// e10
// deferred acc 0
// f1 = 0
// body 12 0
// e14 0
// body 12 1
// e14 1
// it1 done 2
// body 29
// body 32 0
// body 32 s0
// e34 0
// body 32 10
// body 32 s1
// e34 10
// body 32 20
// body 32 s2
// e34 20
// it2 done 3
// body 29
// body 32 0
// body 32 s0
// e34 0
// body 32 10
// body 32 s1
// e34 10
// body 32 20
// body 32 s2
// e34 20
// it2 done 3
// it1 done 2
// body 39
// e41
// body 43
// e45
// body 43
// e45
// body 43
// e45
// it1 done 3
// it1 done 1
// defer 37 1
// defer 37 0
// f2 = 1
// e46
// deferred acc 1
// f3 = 1
// body 48
// body 51
// body 55 0
// e57 0
// body 55 1
// e57 1
// it1 done 2
// body 59 0
// body 51
// body 55 0
// e57 0
// body 55 1
// e57 1
// it1 done 2
// body 59 0
// body 51
// body 55 0
// e57 0
// body 55 1
// e57 1
// it1 done 2
// body 59 0
// it0 done 3
// it1 done 1
// e62
// body 73
// body 76
// body 79 0
// e81 0
// it1 done 1
// body 83
// body 83
// body 83
// it1 done 3
// body 86 0
// it1 done 1
// body 76
// body 79 0
// e81 0
// it1 done 1
// body 83
// body 83
// body 83
// it1 done 3
// body 86 0
// it1 done 1
// it0 done 2
// body 89 0
// body 89 s0
// e91 0
// it2 done 1
// body 73
// body 76
// body 79 0
// e81 0
// it1 done 1
// body 83
// body 83
// body 83
// it1 done 3
// body 86 0
// it1 done 1
// body 76
// body 79 0
// e81 0
// it1 done 1
// body 83
// body 83
// body 83
// it1 done 3
// body 86 0
// it1 done 1
// it0 done 2
// body 89 0
// body 89 s0
// e91 0
// it2 done 1
// it1 done 2
// e92
// f4 = 1
// e94
// f5 = 0
