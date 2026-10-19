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
if acc%2 == 0 {
	goto end
}
for x2 := range it1(2) {
	fmt.Println("body", 2, x2)
	acc++
}
for x5 := range it1(3) {
	fmt.Println("body", 5, x5)
	for range it0(1) {
		fmt.Println("body", 8)
		if x5%2 == 0 {
			for x12 := range it1(1) {
				fmt.Println("body", 12, x12)
				if x12%3 == 2 {
					continue
				}
			}
		} else {
			acc++
		}
		for n16 := range 2 {
			fmt.Println("body", 16, n16)
			if n16 > 2 {
				fmt.Println("e18", x5, n16)
				break
			}
			for range it1(2) {
				fmt.Println("body", 20)
				if x5%2 == 1 {
					fmt.Println("e22", x5, n16)
					goto end
				}
			}
			for range it1(3) {
				fmt.Println("body", 24)
				fmt.Println("e26", x5, n16)
			}
		}
	}
	fmt.Println("e27", x5)
}
if acc == 1 {
	for x30 := range it1(1) {
		fmt.Println("body", 30, x30)
		if x30%2 == 1 {
			continue
		}
	}
} else {
	if acc%2 == 1 {
		fmt.Println("e33")
	}
	fmt.Println("e34")
}
end:
	res = acc
	return
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
for x36 := range it1(3) {
	fmt.Println("body", 36, x36)
	fmt.Println("e38", x36)
	if acc%2 == 1 {
		res = acc + 66
		return
	}
}
if acc == 2 {
	return acc + 66
}
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e39")
return acc + 67
for n41 := range 2 {
	fmt.Println("body", 41, n41)
	fmt.Println("e43", n41)
	fmt.Println("e44", n41)
	fmt.Println("e45", n41)
}
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
return acc + 71
{
	var fs []func() int
	for x47 := range it1(3) {
		fmt.Println("body", 47, x47)
		fs = append(fs, func() int { return x47 })
		for x50, s50 := range it2(3) {
			fmt.Println("body", 50, x50)
			fmt.Println("body", 50, s50)
			for x53 := range it1(2) {
				fmt.Println("body", 53, x53)
				acc += x53 % 5
				fmt.Println("e55", x47, x50, x53)
				fmt.Println("e56", x47, x50, x53)
			}
			for range it0(1) {
				fmt.Println("body", 58)
				for range it1(1) {
					fmt.Println("body", 61)
					fmt.Println("e63", x47, x50)
				}
				fmt.Println("e64", x47, x50)
			}
		}
		if acc == 2 {
			fmt.Println("e65", x47)
			continue
		}
		switch x47 % 3 {
case 0:
			fmt.Println("e67", x47)
case 1:
			for acc = range it1(1) {
				fmt.Println("body", 69)
				fmt.Println("e71", x47)
				for acc = range it1(1) {
					fmt.Println("body", 73)
					if x47%3 == 1 {
						fmt.Println("e75", x47)
						goto end
					}
				}
				goto end
			}
L76:
			for i77 := 0; i77 < 3; i77++ {
				fmt.Println("body", 77, i77)
				for n80 := range 1 {
					fmt.Println("body", 80, n80)
					if x47%3 == 0 {
						fmt.Println("e82", x47, i77, n80)
						return acc + 73
					}
				}
				acc += x47 % 5
				if i77%2 == 1 {
					continue L76
				}
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc > 2 {
	defer fmt.Println("defer", 84, acc)
} else {
	if acc == 0 {
		fmt.Println("e86")
	}
	if acc > 0 {
		fmt.Println("e87")
		fmt.Println("e88")
	}
}
fmt.Println("e89")
end:
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
{
	var fs []func() int
	for x91 := range it1(2) {
		fmt.Println("body", 91, x91)
		fs = append(fs, func() int { return x91 })
L93:
		for x94 := range it1(2) {
			fmt.Println("body", 94, x94)
			for x97, s97 := range it2(2) {
				fmt.Println("body", 97, x97)
				fmt.Println("body", 97, s97)
				for range it0(1) {
					fmt.Println("body", 100)
					if x94%3 == 1 {
						fmt.Println("e102", x91, x94, x97)
						continue
					}
				}
				fmt.Println("e103", x91, x94, x97)
				continue L93
			}
			fmt.Println("e104", x91, x94)
L105:
			for range it0(3) {
				fmt.Println("body", 106)
				return acc + 13
				fmt.Println("e108", x91, x94)
				if x91%3 == 2 {
					break L105
				}
			}
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e109")
	return acc
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e110")
{
	var fs []func() int
	for x112 := range it1(1) {
		fmt.Println("body", 112, x112)
		{
			var fs []func() int
			for i115 := 0; i115 < 2; i115++ {
				fmt.Println("body", 115, i115)
				{
					var fs []func() int
					for i118 := 0; i118 < 3; i118++ {
						fmt.Println("body", 118, i118)
						fs = append(fs, func() int { return i118 })
						for x121 := range it1(3) {
							fmt.Println("body", 121, x121)
							break
						}
						for x124 := range it1(1) {
							fmt.Println("body", 124, x124)
							if i118%3 == 1 {
								continue
							}
						}
						if i118%2 == 0 {
							fmt.Println("e127")
						} else {
							if acc > 1 {
								return acc + 74
							}
						}
					}
					for _, f := range fs {
						fmt.Println("kept", f())
					}
				}
				fmt.Println("e129", x112, i115)
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		for n131 := range 3 {
			fmt.Println("body", 131, n131)
			if n131%2 == 0 {
				goto end
			}
		}
		if x112 == 2 {
			fmt.Println("e133", x112)
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e134")
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
// f0 = 0
// body 36 0
// e38 0
// body 36 1
// e38 1
// body 36 2
// e38 2
// it1 done 3
// f1 = 0
// e39
// f2 = 67
// f3 = 71
// body 91 0
// body 94 0
// body 97 0
// body 97 s0
// body 100
// it0 done 1
// e103 0 0 0
// it2 stopped at 0
// it2 done 2
// body 94 1
// body 97 0
// body 97 s0
// body 100
// e102 0 1 0
// it0 done 1
// e103 0 1 0
// it2 stopped at 0
// it2 done 2
// it1 done 2
// body 91 1
// body 94 0
// body 97 0
// body 97 s0
// body 100
// it0 done 1
// e103 1 0 0
// it2 stopped at 0
// it2 done 2
// body 94 1
// body 97 0
// body 97 s0
// body 100
// e102 1 1 0
// it0 done 1
// e103 1 1 0
// it2 stopped at 0
// it2 done 2
// it1 done 2
// it1 done 2
// kept 0
// kept 1
// e109
// f4 = 0
// e110
// body 112 0
// body 115 0
// body 118 0
// body 121 0
// it1 stopped at 0
// it1 done 3
// body 124 0
// it1 done 1
// e127
// body 118 1
// body 121 0
// it1 stopped at 0
// it1 done 3
// body 124 0
// it1 done 1
// body 118 2
// body 121 0
// it1 stopped at 0
// it1 done 3
// body 124 0
// it1 done 1
// e127
// kept 0
// kept 1
// kept 2
// e129 0 0
// body 115 1
// body 118 0
// body 121 0
// it1 stopped at 0
// it1 done 3
// body 124 0
// it1 done 1
// e127
// body 118 1
// body 121 0
// it1 stopped at 0
// it1 done 3
// body 124 0
// it1 done 1
// body 118 2
// body 121 0
// it1 stopped at 0
// it1 done 3
// body 124 0
// it1 done 1
// e127
// kept 0
// kept 1
// kept 2
// e129 0 1
// body 131 0
// it1 stopped at 0
// it1 done 1
// f5 = 0
