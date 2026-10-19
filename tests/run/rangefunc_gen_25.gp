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
top:
	steps++
	if steps > 3 {
		goto end
	}
for x2 := range it1(1) {
	fmt.Println("body", 2, x2)
	for n5 := range 1 {
		fmt.Println("body", 5, n5)
		for acc = range it1(3) {
			fmt.Println("body", 8)
			for x11 := range it1(2) {
				fmt.Println("body", 11, x11)
				if n5 > 1 {
					goto top
				}
			}
			fmt.Println("e13", x2, n5)
		}
L14:
		for x15 := range it1(3) {
			fmt.Println("body", 15, x15)
			switch x15 % 3 {
case 0:
				if x2 == 0 {
					break
				}
				if x15%2 == 1 {
					goto end
				}
case 1:
				if n5 == 1 {
					fmt.Println("e18", x2, n5, x15)
					break
				}
				if acc == 3 {
					fmt.Println("e19", x2, n5, x15)
				}
default:
				if x2%3 == 0 {
					break L14
				}
			}
			if x2 == 3 {
				if acc == 0 {
					fmt.Println("e21")
					fmt.Println("e22")
				}
			} else {
				if acc == 2 {
					if acc == 3 {
						continue L14
					}
				} else {
					defer fmt.Println("defer", 26, acc)
				}
			}
			acc += x15 % 5
		}
	}
}
for x28 := range it1(1) {
	fmt.Println("body", 28, x28)
	for i31 := 0; i31 < 3; i31++ {
		fmt.Println("body", 31, i31)
		for x34 := range it1(3) {
			fmt.Println("body", 34, x34)
			goto end
		}
	}
}
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
defer fmt.Println("defer", 36, acc)
{
	var fs []func() int
	for x38 := range it1(1) {
		fmt.Println("body", 38, x38)
		switch x38 % 3 {
case 0:
			if acc%2 == 0 {
				return acc + 31
			}
case 1:
			fmt.Println("e41", x38)
			{
				var fs []func() int
				for n43 := range 2 {
					fmt.Println("body", 43, n43)
					fs = append(fs, func() int { return n43 })
					return acc + 19
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
default:
			acc += x38 % 5
		}
		for range it1(3) {
			fmt.Println("body", 46)
			continue
			acc++
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc > 3 {
	fmt.Println("e49")
	fmt.Println("e50")
} else {
	if acc%2 == 0 {
		fmt.Println("e52")
	}
}
{
	var fs []func() int
	for acc = range it1(3) {
		fmt.Println("body", 54)
		{
			var fs []func() int
			for n57 := range 1 {
				fmt.Println("body", 57, n57)
				fs = append(fs, func() int { return n57 })
				for range it1(3) {
					fmt.Println("body", 60)
					for n63 := range 3 {
						fmt.Println("body", 63, n63)
						if n57%2 == 0 {
							fmt.Println("e65", n57, n63)
							return acc + 9
						}
					}
					if n57 > 2 {
						fmt.Println("e66", n57)
						goto top
					}
				}
				defer fmt.Println("defer", 67, acc)
				if n57 > 0 {
					return acc + 4
				}
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		for n69 := range 2 {
			fmt.Println("body", 69, n69)
			fmt.Println("e71", n69)
			if n69 > 1 {
				goto b55
			}
			if acc > 0 {
				fmt.Println("e72", n69)
				return acc + 10
			}
		}
		fmt.Println("e73")
b55:
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
end:
	return acc
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
{
	var fs []func() int
	for acc = range it1(1) {
		fmt.Println("body", 75)
		{
			var fs []func() int
			for range it1(2) {
				fmt.Println("body", 78)
				if acc > 2 {
					defer fmt.Println("defer", 81, acc)
				} else {
					fmt.Println("e83")
				}
				for x85 := range it1(3) {
					fmt.Println("body", 85, x85)
					for range it1(2) {
						fmt.Println("body", 88)
						if x85%2 == 0 {
							if acc%3 == 0 {
								continue
							}
						} else {
							if acc == 0 {
								goto b86
							}
						}
					}
					if x85%3 == 2 {
						fmt.Println("e93")
					} else {
						if acc%3 == 0 {
							break
						}
					}
b86:
				}
				{
					var fs []func() int
					for x96 := range it1(2) {
						fmt.Println("body", 96, x96)
						{
							var fs []func() int
							for x99 := range it1(3) {
								fmt.Println("body", 99, x99)
								fs = append(fs, func() int { return x99 })
								continue
							}
							for _, f := range fs {
								fmt.Println("kept", f())
							}
						}
						goto b79
					}
					for _, f := range fs {
						fmt.Println("kept", f())
					}
				}
b79:
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		acc++
		switch acc % 3 {
case 0:
			fmt.Println("e102")
case 1:
			for x104 := range it1(1) {
				fmt.Println("body", 104, x104)
				acc += x104 % 5
L106:
				for range it0(3) {
					fmt.Println("body", 107)
					if x104 == 1 {
						continue L106
					}
				}
				fmt.Println("e109", x104)
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc%3 == 1 {
	fmt.Println("e110")
	return acc + 96
}
if acc%3 == 2 {
	fmt.Println("e111")
}
if acc == 2 {
	fmt.Println("e112")
	return acc + 14
}
	return acc
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e113")
L114:
switch acc % 3 {
case 0:
	fmt.Println("e115")
	break L114
case 1:
	break L114
	for x117 := range it1(3) {
		fmt.Println("body", 117, x117)
		return acc + 76
	}
}
fmt.Println("e119")
fmt.Println("e120")
	res = acc
	return
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
L121:
switch acc % 3 {
case 0:
	defer fmt.Println("defer", 122, acc)
	if acc%3 == 1 {
		return acc + 63
	}
case 1:
	fmt.Println("e123")
	break L121
default:
	acc++
	for acc = range it1(2) {
		fmt.Println("body", 125)
		continue
		defer fmt.Println("defer", 127, acc)
	}
}
fmt.Println("e128")
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e129")
goto end
for x131 := range it1(2) {
	fmt.Println("body", 131, x131)
	for i134 := 0; i134 < 1; i134++ {
		fmt.Println("body", 134, i134)
		for n137 := range 1 {
			fmt.Println("body", 137, n137)
			fmt.Println("e139", x131, i134, n137)
			for x141, s141 := range it2(3) {
				fmt.Println("body", 141, x141)
				fmt.Println("body", 141, s141)
				fmt.Println("e143", x131, i134, n137, x141)
			}
		}
		if i134 == 3 {
			goto b135
		} else {
			fmt.Println("e146")
		}
b135:
	}
}
if acc > 0 {
	fmt.Println("e148")
} else {
	fmt.Println("e150")
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
// body 2 0
// body 5 0
// body 8
// body 11 0
// body 11 1
// it1 done 2
// e13 0 0
// body 8
// body 11 0
// body 11 1
// it1 done 2
// e13 0 0
// body 8
// body 11 0
// body 11 1
// it1 done 2
// e13 0 0
// it1 done 3
// body 15 0
// body 15 1
// body 15 2
// it1 stopped at 2
// it1 done 3
// it1 done 1
// body 28 0
// body 31 0
// body 34 0
// it1 stopped at 0
// it1 done 3
// it1 stopped at 0
// it1 done 1
// deferred acc 3
// f0 = 3
// body 38 0
// it1 stopped at 0
// it1 done 1
// defer 36 0
// deferred acc 0
// f1 = 31
// body 75
// body 78
// e83
// body 85 0
// body 88
// body 88
// it1 done 2
// it1 stopped at 0
// it1 done 3
// body 96 0
// body 99 0
// body 99 1
// body 99 2
// it1 done 3
// kept 0
// kept 1
// kept 2
// it1 stopped at 0
// it1 done 2
// body 78
// e83
// body 85 0
// body 88
// body 88
// it1 done 2
// it1 stopped at 0
// it1 done 3
// body 96 0
// body 99 0
// body 99 1
// body 99 2
// it1 done 3
// kept 0
// kept 1
// kept 2
// it1 stopped at 0
// it1 done 2
// it1 done 2
// body 104 0
// body 107
// body 107
// body 107
// it0 done 3
// e109 0
// it1 done 1
// it1 done 1
// e110
// f2 = 97
// e113
// e115
// e119
// e120
// f3 = 0
// e128
// defer 122 0
// f4 = 0
// e129
// f5 = 0
