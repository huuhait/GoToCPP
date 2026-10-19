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
{
	var fs []func() int
	for range it0(3) {
		fmt.Println("body", 2)
		{
			var fs []func() int
L4:
			for x5 := range it1(3) {
				fmt.Println("body", 5, x5)
				fmt.Println("e7", x5)
				{
					var fs []func() int
					for i9 := 0; i9 < 2; i9++ {
						fmt.Println("body", 9, i9)
						fs = append(fs, func() int { return i9 })
						goto b6
						goto end
						for x12 := range it1(3) {
							fmt.Println("body", 12, x12)
							if x5 == 1 {
								break L4
							}
						}
					}
					for _, f := range fs {
						fmt.Println("kept", f())
					}
				}
				acc++
b6:
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		for range it1(1) {
			fmt.Println("body", 15)
			fmt.Println("e17")
		}
		goto b3
b3:
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc%3 == 1 {
	for x20, s20 := range it2(1) {
		fmt.Println("body", 20, x20)
		fmt.Println("body", 20, s20)
		res = acc + 83
		return
	}
	acc++
} else {
	if acc > 2 {
		res = acc + 44
		return
	}
L23:
	for n24 := range 1 {
		fmt.Println("body", 24, n24)
		if n24 > 2 {
			continue L23
		} else {
			acc++
		}
		if acc%2 == 0 {
			if acc%2 == 0 {
				if acc%2 == 0 {
					fmt.Println("e30")
				}
			} else {
				acc++
			}
		} else {
			if acc%2 == 0 {
				res = acc + 56
				return
			}
		}
		fmt.Println("e33", n24)
	}
}
end:
	res = acc
	return
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e34")
if acc%2 == 1 {
	for x37 := range it1(3) {
		fmt.Println("body", 37, x37)
		if x37 > 1 {
			fmt.Println("e39", x37)
		}
		for x41 := range it1(2) {
			fmt.Println("body", 41, x41)
			for i44 := 0; i44 < 2; i44++ {
				fmt.Println("body", 44, i44)
				if x37 > 0 {
					fmt.Println("e46", x37, x41, i44)
				}
			}
			if x41%3 == 0 {
				goto top
			}
		}
		for i48 := 0; i48 < 3; i48++ {
			fmt.Println("body", 48, i48)
			if acc == 2 {
				fmt.Println("e50", x37, i48)
				goto b38
			}
		}
b38:
	}
	if acc%2 == 0 {
		fmt.Println("e52")
L53:
		for range it0(2) {
			fmt.Println("body", 54)
L56:
			for acc = range it1(2) {
				fmt.Println("body", 57)
				if acc%3 == 0 {
					break L56
				}
			}
			if acc%3 == 2 {
				fmt.Println("e60")
			} else {
				if acc > 0 {
					fmt.Println("e62")
					continue L53
				}
			}
			fmt.Println("e63")
		}
		if acc%2 == 0 {
			goto top
		}
	} else {
		if acc > 3 {
			if acc == 2 {
				fmt.Println("e66")
			}
		} else {
			for range it0(2) {
				fmt.Println("body", 69)
				if acc == 3 {
					goto end
				}
			}
		}
		for x72 := range it1(1) {
			fmt.Println("body", 72, x72)
			acc++
		}
		switch acc % 3 {
case 0:
			acc++
case 1:
			fmt.Println("e75")
			fmt.Println("e76")
		}
	}
} else {
	fmt.Println("e78")
	acc++
}
if acc%2 == 1 {
	goto top
}
if acc == 0 {
	for n81 := range 1 {
		fmt.Println("body", 81, n81)
		for n84 := range 2 {
			fmt.Println("body", 84, n84)
			fmt.Println("e86", n81, n84)
			for acc = range it1(2) {
				fmt.Println("body", 88)
				fmt.Println("e90", n81, n84)
			}
		}
		for x92 := range it1(2) {
			fmt.Println("body", 92, x92)
			acc++
			fmt.Println("e94", n81, x92)
		}
	}
	fmt.Println("e95")
	if acc%3 == 1 {
		goto top
	}
} else {
	if acc%3 == 0 {
		goto end
	}
	fmt.Println("e97")
}
end:
	return acc
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
for x99 := range it1(1) {
	fmt.Println("body", 99, x99)
	fmt.Println("e101", x99)
}
for x103, s103 := range it2(2) {
	fmt.Println("body", 103, x103)
	fmt.Println("body", 103, s103)
	for acc = range it1(2) {
		fmt.Println("body", 106)
		defer fmt.Println("defer", 108, acc)
		fmt.Println("e109", x103)
		for x111 := range it1(1) {
			fmt.Println("body", 111, x111)
			if x103 == 0 {
				return acc + 48
			}
			for x114 := range it1(2) {
				fmt.Println("body", 114, x114)
				fmt.Println("e116", x103, x111, x114)
			}
L117:
			for x118, s118 := range it2(1) {
				fmt.Println("body", 118, x118)
				fmt.Println("body", 118, s118)
				if x118 == 3 {
					fmt.Println("e121")
				} else {
					if acc%2 == 0 {
						fmt.Println("e123")
						continue L117
					}
				}
			}
		}
	}
}
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
fmt.Println("e124")
goto end
fmt.Println("e125")
end:
	return acc
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
if acc == 3 {
	fmt.Println("e127")
	for n129 := range 2 {
		fmt.Println("body", 129, n129)
L131:
		switch n129 % 3 {
case 0:
			if n129 == 3 {
				goto top
			}
			for x133 := range it1(3) {
				fmt.Println("body", 133, x133)
				if n129%2 == 1 {
					break L131
				}
			}
case 1:
			defer fmt.Println("defer", 135, acc)
			if n129%3 == 1 {
				return acc + 33
			}
		}
	}
} else {
	if acc == 0 {
		fmt.Println("e137")
		goto end
	}
}
for range it1(3) {
	fmt.Println("body", 139)
	fmt.Println("e141")
	if acc > 2 {
		goto end
	}
}
fmt.Println("e142")
fmt.Println("e143")
end:
	res = acc
	return
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
for x145, s145 := range it2(1) {
	fmt.Println("body", 145, x145)
	fmt.Println("body", 145, s145)
	if x145 > 2 {
		fmt.Println("e147", x145)
		fmt.Println("e148", x145)
	}
}
{
	var fs []func() int
	for x150, s150 := range it2(1) {
		fmt.Println("body", 150, x150)
		fmt.Println("body", 150, s150)
		fs = append(fs, func() int { return x150 })
		fmt.Println("e152", x150)
		acc++
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
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
// body 2
// body 5 0
// e7 0
// body 9 0
// body 5 1
// e7 1
// body 9 0
// body 5 2
// e7 2
// body 9 0
// it1 done 3
// body 15
// e17
// it1 done 1
// body 2
// body 5 0
// e7 0
// body 9 0
// body 5 1
// e7 1
// body 9 0
// body 5 2
// e7 2
// body 9 0
// it1 done 3
// body 15
// e17
// it1 done 1
// body 2
// body 5 0
// e7 0
// body 9 0
// body 5 1
// e7 1
// body 9 0
// body 5 2
// e7 2
// body 9 0
// it1 done 3
// body 15
// e17
// it1 done 1
// it0 done 3
// body 24 0
// e33 0
// deferred acc 1
// f0 = 1
// e34
// e78
// e34
// body 37 0
// body 41 0
// body 44 0
// body 44 1
// it1 stopped at 0
// it1 done 2
// it1 stopped at 0
// it1 done 3
// e34
// body 37 0
// body 41 0
// body 44 0
// body 44 1
// it1 stopped at 0
// it1 done 2
// it1 stopped at 0
// it1 done 3
// f1 = 1
// body 99 0
// e101 0
// it1 done 1
// body 103 0
// body 103 s0
// body 106
// e109 0
// body 111 0
// it1 stopped at 0
// it1 done 1
// it1 stopped at 0
// it1 done 2
// it2 stopped at 0
// it2 done 2
// defer 108 0
// f2 = 48
// e124
// deferred acc 0
// f3 = 0
// e137
// deferred acc 0
// f4 = 0
// body 145 0
// body 145 s0
// it2 done 1
// body 150 0
// body 150 s0
// e152 0
// it2 done 1
// kept 0
// f5 = 1
