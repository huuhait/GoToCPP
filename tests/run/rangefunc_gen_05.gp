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
acc++
defer fmt.Println("defer", 1, acc)
for x3 := range it1(3) {
	fmt.Println("body", 3, x3)
	for x6, s6 := range it2(1) {
		fmt.Println("body", 6, x6)
		fmt.Println("body", 6, s6)
		goto end
	}
	acc += x3 % 5
	for n9 := range 1 {
		fmt.Println("body", 9, n9)
		fmt.Println("e11", x3, n9)
	}
}
end:
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
acc++
{
	var fs []func() int
	for x13 := range it1(2) {
		fmt.Println("body", 13, x13)
		fs = append(fs, func() int { return x13 })
		break
		continue
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc > 2 {
	fmt.Println("e15")
}
	res = acc
	return
}

func f2() (res int) {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e16")
L17:
switch acc % 3 {
case 0:
	if acc%2 == 1 {
		break
	}
case 1:
	if acc == 3 {
		break
	}
	for x19 := range it1(1) {
		fmt.Println("body", 19, x19)
		if acc > 3 {
			continue
		} else {
			goto top
		}
	}
default:
	if acc%2 == 0 {
		fmt.Println("e23")
		break L17
	}
	break L17
}
for x25 := range it1(2) {
	fmt.Println("body", 25, x25)
	acc++
	for x28 := range it1(3) {
		fmt.Println("body", 28, x28)
		fmt.Println("e30", x25, x28)
	}
	switch x25 % 3 {
case 0:
		for x33 := range it1(3) {
			fmt.Println("body", 33, x33)
			fmt.Println("e35", x25, x33)
			fmt.Println("e36", x25, x33)
			fmt.Println("e37", x25, x33)
		}
case 1:
L38:
		for x39, s39 := range it2(1) {
			fmt.Println("body", 39, x39)
			fmt.Println("body", 39, s39)
			for x42, s42 := range it2(2) {
				fmt.Println("body", 42, x42)
				fmt.Println("body", 42, s42)
				if x39%3 == 0 {
					res = acc + 27
					return
				}
			}
			fmt.Println("e44", x25, x39)
			for range it1(3) {
				fmt.Println("body", 46)
				if x25%2 == 0 {
					break L38
				} else {
					goto b47
				}
b47:
			}
		}
		if x25 == 0 {
			defer fmt.Println("defer", 51, acc)
		} else {
			for acc = range it1(3) {
				fmt.Println("body", 54)
				fmt.Println("e56")
			}
		}
default:
		goto b26
	}
b26:
}
goto end
end:
	res = acc
	return
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
if acc == 0 {
	goto end
}
fmt.Println("e57")
{
	var fs []func() int
	for range it0(3) {
		fmt.Println("body", 59)
		{
			var fs []func() int
			for x62 := range it1(2) {
				fmt.Println("body", 62, x62)
				{
					var fs []func() int
L64:
					for x65 := range it1(3) {
						fmt.Println("body", 65, x65)
						fs = append(fs, func() int { return x65 })
						for x68 := range it1(2) {
							fmt.Println("body", 68, x68)
							if x62%2 == 1 {
								fmt.Println("e70", x62, x65, x68)
								continue L64
							}
						}
						acc++
						if x65%2 == 1 {
							goto b66
						}
b66:
					}
					for _, f := range fs {
						fmt.Println("kept", f())
					}
				}
				switch x62 % 3 {
case 0:
					{
						var fs []func() int
L72:
						for x73 := range it1(3) {
							fmt.Println("body", 73, x73)
							fs = append(fs, func() int { return x73 })
							if acc%3 == 1 {
								break L72
							}
						}
						for _, f := range fs {
							fmt.Println("kept", f())
						}
					}
case 1:
					for range it0(2) {
						fmt.Println("body", 76)
						fmt.Println("e78", x62)
					}
					{
						var fs []func() int
						for x80 := range it1(3) {
							fmt.Println("body", 80, x80)
							fs = append(fs, func() int { return x80 })
							if x80 == 1 {
								res = acc + 9
								return
							}
						}
						for _, f := range fs {
							fmt.Println("kept", f())
						}
					}
default:
					for range it1(2) {
						fmt.Println("body", 83)
						fmt.Println("e85", x62)
					}
					acc += x62 % 5
				}
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
goto end
end:
	res = acc
	return
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
defer fmt.Println("defer", 86, acc)
fmt.Println("e87")
goto end
end:
	return acc
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e88")
if acc == 0 {
	goto end
}
for n90 := range 1 {
	fmt.Println("body", 90, n90)
	for n93 := range 3 {
		fmt.Println("body", 93, n93)
		fmt.Println("e95", n90, n93)
	}
	fmt.Println("e96", n90)
}
for range it0(1) {
	fmt.Println("body", 98)
	for acc = range it1(2) {
		fmt.Println("body", 101)
		acc++
	}
}
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
// body 6 0
// body 6 s0
// it2 stopped at 0
// it2 done 1
// it1 stopped at 0
// it1 done 3
// defer 1 1
// deferred acc 1
// f0 = 1
// body 13 0
// it1 stopped at 0
// it1 done 2
// kept 0
// deferred acc 1
// f1 = 1
// e16
// body 25 0
// body 28 0
// e30 0 0
// body 28 1
// e30 0 1
// body 28 2
// e30 0 2
// it1 done 3
// body 33 0
// e35 0 0
// e36 0 0
// e37 0 0
// body 33 1
// e35 0 1
// e36 0 1
// e37 0 1
// body 33 2
// e35 0 2
// e36 0 2
// e37 0 2
// it1 done 3
// body 25 1
// body 28 0
// e30 1 0
// body 28 1
// e30 1 1
// body 28 2
// e30 1 2
// it1 done 3
// body 39 0
// body 39 s0
// body 42 0
// body 42 s0
// it2 stopped at 0
// it2 done 2
// it2 stopped at 0
// it2 done 1
// it1 stopped at 1
// it1 done 2
// f2 = 29
// deferred acc 0
// f3 = 0
// e87
// defer 86 0
// deferred acc 0
// f4 = 0
// e88
// f5 = 0
