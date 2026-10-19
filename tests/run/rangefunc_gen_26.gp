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
acc++
acc++
if acc == 2 {
	fmt.Println("e1")
	return acc + 19
}
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
if acc == 1 {
	fmt.Println("e2")
}
fmt.Println("e3")
fmt.Println("e4")
	return acc
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
if acc%2 == 1 {
	fmt.Println("e5")
	return acc + 4
}
for range it0(1) {
	fmt.Println("body", 7)
	for n10 := range 1 {
		fmt.Println("body", 10, n10)
		fmt.Println("e12", n10)
		fmt.Println("e13", n10)
		acc++
	}
}
L14:
for i15 := 0; i15 < 1; i15++ {
	fmt.Println("body", 15, i15)
	for x18 := range it1(2) {
		fmt.Println("body", 18, x18)
		for range it0(1) {
			fmt.Println("body", 21)
			if i15 == 0 {
				fmt.Println("e23", i15, x18)
				break L14
			}
			if x18%3 == 2 {
				break
			}
		}
		for x25 := range it1(2) {
			fmt.Println("body", 25, x25)
			fmt.Println("e27", i15, x18, x25)
			acc += x25 % 5
		}
	}
	if acc == 3 {
		continue L14
	}
}
for range it1(1) {
	fmt.Println("body", 29)
	if acc%2 == 0 {
		fmt.Println("e31")
		return acc + 84
	}
	fmt.Println("e32")
}
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for x34 := range it1(1) {
	fmt.Println("body", 34, x34)
L36:
	for x37 := range it1(2) {
		fmt.Println("body", 37, x37)
		continue L36
	}
	if x34 > 1 {
		fmt.Println("e39", x34)
	}
	fmt.Println("e40", x34)
}
if acc%2 == 1 {
	if acc%2 == 0 {
		fmt.Println("e42")
	}
	fmt.Println("e43")
	fmt.Println("e44")
} else {
	if acc%2 == 1 {
		fmt.Println("e46")
		fmt.Println("e47")
	}
	for x49 := range it1(3) {
		fmt.Println("body", 49, x49)
		for acc = range it1(1) {
			fmt.Println("body", 52)
			acc++
			for range it1(1) {
				fmt.Println("body", 55)
				if x49 > 3 {
					if acc > 2 {
						if acc > 2 {
							fmt.Println("e59")
						}
					} else {
						if acc%3 == 2 {
							fmt.Println("e61")
							continue
						}
					}
				} else {
					fmt.Println("e63")
				}
			}
		}
		if acc > 1 {
			goto end
		} else {
			fmt.Println("e66")
		}
	}
	fmt.Println("e67")
}
end:
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
L68:
switch acc % 3 {
case 0:
	fmt.Println("e69")
	if acc%2 == 1 {
		break
	}
case 1:
	for x71 := range it1(3) {
		fmt.Println("body", 71, x71)
		fmt.Println("e73", x71)
		defer fmt.Println("defer", 74, acc)
	}
default:
	acc++
	for i76 := 0; i76 < 3; i76++ {
		fmt.Println("body", 76, i76)
		for x79 := range it1(2) {
			fmt.Println("body", 79, x79)
			if i76 == 1 {
				fmt.Println("e81", i76, x79)
				goto b80
			}
			break L68
			if x79%3 == 0 {
				break L68
			}
b80:
		}
	}
}
{
	var fs []func() int
L82:
	for x83 := range it1(3) {
		fmt.Println("body", 83, x83)
		fmt.Println("e85", x83)
		if x83%2 == 0 {
			fmt.Println("e87")
		} else {
			res = acc + 12
			return
			{
				var fs []func() int
				for x90 := range it1(3) {
					fmt.Println("body", 90, x90)
					fs = append(fs, func() int { return x90 })
					if x90%2 == 0 {
						acc++
					} else {
						if acc%2 == 1 {
							break L82
						}
					}
					acc += x90 % 5
					for x95 := range it1(3) {
						fmt.Println("body", 95, x95)
						if x95 == 3 {
							fmt.Println("e97", x90, x95)
							goto end
						}
					}
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
			if acc%3 == 1 {
				fmt.Println("e98")
				fmt.Println("e99")
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
for i101 := 0; i101 < 3; i101++ {
	fmt.Println("body", 101, i101)
	for x104 := range it1(1) {
		fmt.Println("body", 104, x104)
		if x104%3 == 2 {
			fmt.Println("e106", i101, x104)
			continue
		}
	}
}
for i108 := 0; i108 < 1; i108++ {
	fmt.Println("body", 108, i108)
	for x111 := range it1(3) {
		fmt.Println("body", 111, x111)
		for x114 := range it1(3) {
			fmt.Println("body", 114, x114)
			defer fmt.Println("defer", 116, acc)
			if x114 > 1 {
				res = acc + 75
				return
			}
		}
	}
	fmt.Println("e117", i108)
}
end:
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
defer fmt.Println("defer", 118, acc)
{
	var fs []func() int
	for x120 := range it1(1) {
		fmt.Println("body", 120, x120)
		fs = append(fs, func() int { return x120 })
		fmt.Println("e122", x120)
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
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
// f0 = 21
// e3
// e4
// f1 = 0
// body 7
// body 10 0
// e12 0
// e13 0
// it0 done 1
// body 15 0
// body 18 0
// body 21
// e23 0 0
// it0 stopped at 0
// it0 done 1
// it1 stopped at 0
// it1 done 2
// body 29
// e32
// it1 done 1
// f2 = 1
// body 34 0
// body 37 0
// body 37 1
// it1 done 2
// e40 0
// it1 done 1
// body 49 0
// body 52
// body 55
// e63
// it1 done 1
// it1 done 1
// e66
// body 49 1
// body 52
// body 55
// e63
// it1 done 1
// it1 done 1
// e66
// body 49 2
// body 52
// body 55
// e63
// it1 done 1
// it1 done 1
// e66
// it1 done 3
// e67
// deferred acc 1
// f3 = 1
// e69
// body 83 0
// e85 0
// e87
// body 83 1
// e85 1
// it1 stopped at 1
// it1 done 3
// deferred acc 0
// f4 = 12
// body 120 0
// e122 0
// it1 done 1
// kept 0
// defer 118 0
// f5 = 0
