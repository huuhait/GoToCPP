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
top:
	steps++
	if steps > 3 {
		goto end
	}
if acc%3 == 1 {
	return acc + 69
}
fmt.Println("e1")
if acc == 1 {
	fmt.Println("e2")
}
for x4 := range it1(2) {
	fmt.Println("body", 4, x4)
	for i7 := 0; i7 < 3; i7++ {
		fmt.Println("body", 7, i7)
		fmt.Println("e9", x4, i7)
		if acc > 3 {
			fmt.Println("e10", x4, i7)
		}
		fmt.Println("e11", x4, i7)
	}
	goto top
	for x13 := range it1(1) {
		fmt.Println("body", 13, x13)
		fmt.Println("e15", x4, x13)
	}
}
end:
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for n17 := range 1 {
	fmt.Println("body", 17, n17)
	fmt.Println("e19", n17)
}
switch acc % 3 {
case 0:
	fmt.Println("e21")
	{
		var fs []func() int
		for x23 := range it1(2) {
			fmt.Println("body", 23, x23)
			fs = append(fs, func() int { return x23 })
			fmt.Println("e25", x23)
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
case 1:
	for i27 := 0; i27 < 1; i27++ {
		fmt.Println("body", 27, i27)
		for range it0(2) {
			fmt.Println("body", 30)
			if i27 > 1 {
				fmt.Println("e32", i27)
				res = acc + 95
				return
			}
		}
	}
}
fmt.Println("e33")
fmt.Println("e34")
	res = acc
	return
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
defer fmt.Println("defer", 35, acc)
fmt.Println("e36")
goto end
end:
	res = acc
	return
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e37")
{
	var fs []func() int
L38:
	for x39 := range it1(1) {
		fmt.Println("body", 39, x39)
		fs = append(fs, func() int { return x39 })
		if x39%3 == 1 {
			if acc > 3 {
				break L38
			}
L42:
			for acc = range it1(1) {
				fmt.Println("body", 43)
				fmt.Println("e45")
				fmt.Println("e46")
				if acc > 3 {
					fmt.Println("e47")
					break L42
				}
			}
			fmt.Println("e48")
		} else {
			{
				var fs []func() int
				for x51, s51 := range it2(3) {
					fmt.Println("body", 51, x51)
					fmt.Println("body", 51, s51)
					fs = append(fs, func() int { return x51 })
					defer fmt.Println("defer", 53, acc)
					fmt.Println("e54", x51)
					if x51%2 == 1 {
						goto end
					}
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
			if acc == 1 {
				fmt.Println("e55")
				return acc + 92
			}
		}
		for n57 := range 3 {
			fmt.Println("body", 57, n57)
			if n57 == 0 {
				defer fmt.Println("defer", 60, acc)
			} else {
				break L38
			}
			for range it0(2) {
				fmt.Println("body", 63)
				if n57%3 == 0 {
					if acc%2 == 0 {
						goto top
					}
				} else {
					if acc%3 == 1 {
						goto end
					}
				}
				fmt.Println("e67", x39, n57)
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc%2 == 0 {
	fmt.Println("e68")
}
end:
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc%2 == 1 {
	fmt.Println("e69")
	fmt.Println("e70")
}
goto end
fmt.Println("e71")
end:
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
for x73 := range it1(1) {
	fmt.Println("body", 73, x73)
	fmt.Println("e75", x73)
}
if acc > 1 {
	fmt.Println("e76")
}
fmt.Println("e77")
L78:
switch acc % 3 {
case 0:
	fmt.Println("e79")
	fmt.Println("e80")
case 1:
	break L78
	for range it0(2) {
		fmt.Println("body", 82)
		acc++
		for acc = range it1(2) {
			fmt.Println("body", 85)
			for acc = range it1(2) {
				fmt.Println("body", 88)
				fmt.Println("e90")
			}
		}
	}
default:
	break
	if acc > 0 {
		fmt.Println("e91")
		fmt.Println("e92")
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
// body 4 0
// body 7 0
// e9 0 0
// e11 0 0
// body 7 1
// e9 0 1
// e11 0 1
// body 7 2
// e9 0 2
// e11 0 2
// it1 stopped at 0
// it1 done 2
// e1
// body 4 0
// body 7 0
// e9 0 0
// e11 0 0
// body 7 1
// e9 0 1
// e11 0 1
// body 7 2
// e9 0 2
// e11 0 2
// it1 stopped at 0
// it1 done 2
// e1
// body 4 0
// body 7 0
// e9 0 0
// e11 0 0
// body 7 1
// e9 0 1
// e11 0 1
// body 7 2
// e9 0 2
// e11 0 2
// it1 stopped at 0
// it1 done 2
// f0 = 0
// body 17 0
// e19 0
// e21
// body 23 0
// e25 0
// body 23 1
// e25 1
// it1 done 2
// kept 0
// kept 1
// e33
// e34
// deferred acc 0
// f1 = 0
// e36
// defer 35 0
// f2 = 0
// e37
// body 39 0
// body 51 0
// body 51 s0
// e54 0
// body 51 10
// body 51 s1
// e54 10
// body 51 20
// body 51 s2
// e54 20
// it2 done 3
// kept 0
// kept 10
// kept 20
// body 57 0
// body 63
// it0 stopped at 0
// it0 done 2
// it1 stopped at 0
// it1 done 1
// e37
// body 39 0
// body 51 0
// body 51 s0
// e54 0
// body 51 10
// body 51 s1
// e54 10
// body 51 20
// body 51 s2
// e54 20
// it2 done 3
// kept 0
// kept 10
// kept 20
// body 57 0
// body 63
// it0 stopped at 0
// it0 done 2
// it1 stopped at 0
// it1 done 1
// e37
// body 39 0
// body 51 0
// body 51 s0
// e54 0
// body 51 10
// body 51 s1
// e54 10
// body 51 20
// body 51 s2
// e54 20
// it2 done 3
// kept 0
// kept 10
// kept 20
// body 57 0
// body 63
// it0 stopped at 0
// it0 done 2
// it1 stopped at 0
// it1 done 1
// defer 60 0
// defer 53 0
// defer 53 0
// defer 53 0
// defer 60 0
// defer 53 0
// defer 53 0
// defer 53 0
// defer 60 0
// defer 53 0
// defer 53 0
// defer 53 0
// deferred acc 0
// f3 = 0
// deferred acc 0
// f4 = 0
// body 73 0
// e75 0
// it1 done 1
// e77
// e79
// e80
// deferred acc 0
// f5 = 0
