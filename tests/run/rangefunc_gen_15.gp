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
	defer func() { fmt.Println("deferred acc", acc) }()
if acc%3 == 2 {
	if acc == 1 {
		for x4 := range it1(3) {
			fmt.Println("body", 4, x4)
			fmt.Println("e6", x4)
		}
		fmt.Println("e7")
		fmt.Println("e8")
	} else {
L10:
		for i11 := 0; i11 < 3; i11++ {
			fmt.Println("body", 11, i11)
			if i11%2 == 0 {
				if acc%3 == 2 {
					fmt.Println("e14")
					continue L10
				}
			} else {
				fmt.Println("e16")
			}
		}
		fmt.Println("e17")
		return acc + 7
	}
	fmt.Println("e18")
	fmt.Println("e19")
} else {
	if acc%3 == 2 {
		return acc + 18
	}
}
{
	var fs []func() int
L21:
	for x22 := range it1(2) {
		fmt.Println("body", 22, x22)
		switch x22 % 3 {
case 0:
			if x22 > 1 {
				return acc + 4
			}
			{
				var fs []func() int
				for x26 := range it1(2) {
					fmt.Println("body", 26, x26)
					fs = append(fs, func() int { return x26 })
					defer fmt.Println("defer", 28, acc)
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
case 1:
			fmt.Println("e29", x22)
			acc++
		}
		for x31, s31 := range it2(1) {
			fmt.Println("body", 31, x31)
			fmt.Println("body", 31, s31)
			continue L21
		}
		fmt.Println("e33", x22)
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e34")
if acc%2 == 1 {
	fmt.Println("e35")
}
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc > 2 {
	fmt.Println("e37")
L38:
	switch acc % 3 {
case 0:
		{
			var fs []func() int
			for x40 := range it1(1) {
				fmt.Println("body", 40, x40)
				fs = append(fs, func() int { return x40 })
				fmt.Println("e42", x40)
				acc++
				acc += x40 % 5
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		fmt.Println("e43")
case 1:
		for x45, s45 := range it2(2) {
			fmt.Println("body", 45, x45)
			fmt.Println("body", 45, s45)
			for x48 := range it1(2) {
				fmt.Println("body", 48, x48)
				fmt.Println("e50", x45, x48)
			}
			fmt.Println("e51", x45)
			if x45%2 == 1 {
				fmt.Println("e52", x45)
				goto b36
			}
		}
default:
		if acc == 1 {
			break L38
		} else {
			fmt.Println("e55")
		}
	}
	defer fmt.Println("defer", 56, acc)
b36:
} else {
	return acc + 39
	if acc > 0 {
		fmt.Println("e58")
	}
}
for range it1(1) {
	fmt.Println("body", 60)
	acc++
	if acc%3 == 0 {
		continue
	}
	fmt.Println("e62")
}
if acc > 2 {
	fmt.Println("e63")
	return acc + 84
}
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
defer fmt.Println("defer", 64, acc)
fmt.Println("e65")
for i67 := 0; i67 < 1; i67++ {
	fmt.Println("body", 67, i67)
	fmt.Println("e69", i67)
}
fmt.Println("e70")
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e71")
fmt.Println("e72")
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for x74 := range it1(3) {
	fmt.Println("body", 74, x74)
	fmt.Println("e76", x74)
	if x74 > 0 {
L78:
		switch acc % 3 {
case 0:
L79:
			for x80 := range it1(3) {
				fmt.Println("body", 80, x80)
				if x80%3 == 0 {
					fmt.Println("e82", x80)
					break L79
				}
			}
case 1:
			for acc = range it1(3) {
				fmt.Println("body", 84)
				fmt.Println("e86")
			}
default:
			if acc%2 == 1 {
				if acc%2 == 1 {
					break
				}
			} else {
				if acc%2 == 1 {
					break L78
				}
			}
		}
	} else {
		fmt.Println("e90")
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
// body 22 0
// body 26 0
// body 26 1
// it1 done 2
// kept 0
// kept 1
// body 31 0
// body 31 s0
// it2 stopped at 0
// it2 done 1
// body 22 1
// e29 1
// body 31 0
// body 31 s0
// it2 stopped at 0
// it2 done 1
// it1 done 2
// defer 28 0
// defer 28 0
// deferred acc 1
// f0 = 1
// e34
// f1 = 0
// deferred acc 0
// f2 = 39
// e65
// body 67 0
// e69 0
// e70
// defer 64 0
// deferred acc 0
// f3 = 0
// e71
// e72
// f4 = 0
// body 74 0
// e76 0
// e90
// body 74 1
// e76 1
// body 80 0
// e82 0
// it1 stopped at 0
// it1 done 3
// body 74 2
// e76 2
// body 80 0
// e82 0
// it1 stopped at 0
// it1 done 3
// it1 done 3
// e91
// deferred acc 0
// f5 = 0
