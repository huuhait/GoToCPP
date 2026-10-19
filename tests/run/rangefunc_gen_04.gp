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
fmt.Println("e1")
L2:
for i3 := 0; i3 < 2; i3++ {
	fmt.Println("body", 3, i3)
	if acc > 0 {
		fmt.Println("e5", i3)
		goto end
	}
	if i3%3 == 2 {
		fmt.Println("e7")
	} else {
		if acc%2 == 0 {
			fmt.Println("e9")
			goto end
		}
L10:
		for x11 := range it1(1) {
			fmt.Println("body", 11, x11)
			switch x11 % 3 {
case 0:
				if x11%3 == 2 {
					fmt.Println("e14", x11)
					continue L10
				}
				if x11 > 2 {
					acc++
				} else {
					if acc > 2 {
						fmt.Println("e17")
					}
				}
case 1:
				fmt.Println("e18", x11)
				if x11%3 == 0 {
					goto end
				}
			}
			if x11 == 2 {
				fmt.Println("e19", x11)
				break L10
			}
			for x21 := range it1(2) {
				fmt.Println("body", 21, x21)
				fmt.Println("e23", x11, x21)
			}
		}
		if acc > 3 {
			fmt.Println("e24")
			break L2
		}
	}
}
fmt.Println("e25")
end:
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e26")
fmt.Println("e27")
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
{
	var fs []func() int
	for x29 := range it1(1) {
		fmt.Println("body", 29, x29)
		fs = append(fs, func() int { return x29 })
		for range it1(3) {
			fmt.Println("body", 32)
			goto end
			fmt.Println("e34", x29)
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
goto end
end:
	return acc
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
switch acc % 3 {
case 0:
	if acc%2 == 1 {
		fmt.Println("e36")
		goto end
	}
	fmt.Println("e37")
case 1:
	goto top
	fmt.Println("e38")
default:
	fmt.Println("e39")
	if acc%2 == 1 {
		fmt.Println("e40")
		break
	}
}
defer fmt.Println("defer", 41, acc)
fmt.Println("e42")
{
	var fs []func() int
	for n44 := range 2 {
		fmt.Println("body", 44, n44)
		if n44%2 == 1 {
			fmt.Println("e46", n44)
			return acc + 93
		}
		fmt.Println("e47", n44)
		{
			var fs []func() int
			for x49 := range it1(3) {
				fmt.Println("body", 49, x49)
				acc++
				{
					var fs []func() int
					for x52, s52 := range it2(1) {
						fmt.Println("body", 52, x52)
						fmt.Println("body", 52, s52)
						fs = append(fs, func() int { return x52 })
						{
							var fs []func() int
							for x55, s55 := range it2(1) {
								fmt.Println("body", 55, x55)
								fmt.Println("body", 55, s55)
								fs = append(fs, func() int { return x55 })
								if acc > 3 {
									fmt.Println("e57", n44, x49, x52, x55)
								}
							}
							for _, f := range fs {
								fmt.Println("kept", f())
							}
						}
						fmt.Println("e58", n44, x49, x52)
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
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
end:
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
for range it1(3) {
	fmt.Println("body", 60)
	for x63 := range it1(3) {
		fmt.Println("body", 63, x63)
		acc++
	}
	switch acc % 3 {
case 0:
		acc++
		fmt.Println("e66")
case 1:
		for i68 := 0; i68 < 2; i68++ {
			fmt.Println("body", 68, i68)
			fmt.Println("e70", i68)
		}
default:
		fmt.Println("e71")
		for n73 := range 2 {
			fmt.Println("body", 73, n73)
			fmt.Println("e75", n73)
		}
	}
}
return acc + 97
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e76")
for i78 := 0; i78 < 1; i78++ {
	fmt.Println("body", 78, i78)
	defer fmt.Println("defer", 80, acc)
	for acc = range it1(1) {
		fmt.Println("body", 82)
		return acc + 8
		for range it1(3) {
			fmt.Println("body", 85)
			fmt.Println("e87", i78)
		}
		acc++
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
// body 3 0
// e9
// f0 = 0
// e26
// e27
// deferred acc 0
// f1 = 0
// body 29 0
// body 32
// it1 stopped at 0
// it1 done 3
// it1 stopped at 0
// it1 done 1
// f2 = 0
// e37
// e42
// body 44 0
// e47 0
// body 49 0
// body 52 0
// body 52 s0
// body 55 0
// body 55 s0
// it2 done 1
// kept 0
// e58 0 0 0
// it2 done 1
// kept 0
// body 49 1
// body 52 0
// body 52 s0
// body 55 0
// body 55 s0
// it2 done 1
// kept 0
// e58 0 1 0
// it2 done 1
// kept 0
// body 49 2
// body 52 0
// body 52 s0
// body 55 0
// body 55 s0
// it2 done 1
// kept 0
// e58 0 2 0
// it2 done 1
// kept 0
// it1 done 3
// body 44 1
// e46 1
// defer 41 0
// deferred acc 3
// f3 = 96
// body 60
// body 63 0
// body 63 1
// body 63 2
// it1 done 3
// e66
// body 60
// body 63 0
// body 63 1
// body 63 2
// it1 done 3
// body 68 0
// e70 0
// body 68 1
// e70 1
// body 60
// body 63 0
// body 63 1
// body 63 2
// it1 done 3
// body 68 0
// e70 0
// body 68 1
// e70 1
// it1 done 3
// f4 = 107
// e76
// body 78 0
// body 82
// it1 stopped at 0
// it1 done 1
// defer 80 0
// deferred acc 0
// f5 = 8
