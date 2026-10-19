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
	return acc + 78
}
fmt.Println("e1")
for x3 := range it1(1) {
	fmt.Println("body", 3, x3)
	fmt.Println("e5", x3)
}
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e6")
for range it1(3) {
	fmt.Println("body", 8)
	for acc = range it1(2) {
		fmt.Println("body", 11)
		fmt.Println("e13")
		for x15 := range it1(1) {
			fmt.Println("body", 15, x15)
			fmt.Println("e17", x15)
		}
	}
}
	return acc
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
{
	var fs []func() int
L18:
	for x19 := range it1(2) {
		fmt.Println("body", 19, x19)
		if x19 > 0 {
			fmt.Println("e22")
		} else {
			break L18
		}
		for x25 := range it1(1) {
			fmt.Println("body", 25, x25)
			if x19%2 == 1 {
				goto end
			}
		}
		if acc > 0 {
L28:
			for x29 := range it1(3) {
				fmt.Println("body", 29, x29)
				if acc == 3 {
					fmt.Println("e32")
				} else {
					if acc%3 == 0 {
						continue L28
					}
				}
				defer fmt.Println("defer", 34, acc)
			}
			acc++
		} else {
			{
				var fs []func() int
				for x37 := range it1(2) {
					fmt.Println("body", 37, x37)
					{
						var fs []func() int
						for x40 := range it1(1) {
							fmt.Println("body", 40, x40)
							fs = append(fs, func() int { return x40 })
							if acc%3 == 0 {
								return acc + 58
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
L42:
			for range it0(1) {
				fmt.Println("body", 43)
				acc++
L45:
				switch acc % 3 {
case 0:
					if acc > 0 {
						break L45
					}
					fmt.Println("e46")
case 1:
					if acc == 1 {
						goto b35
					}
					if acc%2 == 0 {
						return acc + 95
					}
default:
					if acc%2 == 0 {
						continue L42
					}
					if acc == 0 {
						break L45
					}
				}
			}
b35:
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e47")
if acc > 1 {
	for x50 := range it1(3) {
		fmt.Println("body", 50, x50)
		acc++
		acc += x50 % 5
	}
	for n53 := range 2 {
		fmt.Println("body", 53, n53)
		if n53 == 1 {
			if acc > 1 {
				return acc + 46
			}
		} else {
L57:
			for x58, s58 := range it2(3) {
				fmt.Println("body", 58, x58)
				fmt.Println("body", 58, s58)
				if acc%2 == 0 {
					break L57
				}
			}
		}
		fmt.Println("e60", n53)
		for acc = range it1(1) {
			fmt.Println("body", 62)
			for i65 := 0; i65 < 1; i65++ {
				fmt.Println("body", 65, i65)
				if acc == 3 {
					goto top
				}
			}
			fmt.Println("e67", n53)
			if n53%2 == 0 {
				acc++
			} else {
				if acc == 0 {
					fmt.Println("e70")
				}
			}
		}
	}
	{
		var fs []func() int
L71:
		for x72 := range it1(1) {
			fmt.Println("body", 72, x72)
			{
				var fs []func() int
				for x75 := range it1(3) {
					fmt.Println("body", 75, x75)
					fs = append(fs, func() int { return x75 })
					acc++
					acc++
					for range it1(1) {
						fmt.Println("body", 78)
						if x75%3 == 2 {
							continue
						}
					}
				}
				for _, f := range fs {
					fmt.Println("kept", f())
				}
			}
			if acc > 2 {
				continue L71
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
} else {
	goto end
	defer fmt.Println("defer", 81, acc)
}
end:
	return acc
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
L82:
for x83 := range it1(1) {
	fmt.Println("body", 83, x83)
	break L82
	for x86, s86 := range it2(2) {
		fmt.Println("body", 86, x86)
		fmt.Println("body", 86, s86)
		acc++
		for n89 := range 2 {
			fmt.Println("body", 89, n89)
			fmt.Println("e91", x83, x86, n89)
			for x93 := range it1(3) {
				fmt.Println("body", 93, x93)
				if x83%3 == 0 {
					fmt.Println("e96")
				} else {
					if acc == 0 {
						if acc > 0 {
							continue
						}
					} else {
						if acc%2 == 1 {
							return acc + 33
						}
					}
				}
			}
		}
		break L82
	}
	if x83%3 == 2 {
		fmt.Println("e100", x83)
		break L82
	}
}
for x102 := range it1(2) {
	fmt.Println("body", 102, x102)
	return acc + 83
	if x102%2 == 1 {
		fmt.Println("e104", x102)
	}
}
	res = acc
	return
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
{
	var fs []func() int
	for i106 := 0; i106 < 1; i106++ {
		fmt.Println("body", 106, i106)
		fs = append(fs, func() int { return i106 })
		if i106%3 == 0 {
			goto top
		}
		fmt.Println("e108", i106)
		if acc%3 == 1 {
			res = acc + 76
			return
		}
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
if acc == 0 {
L110:
	for n111 := range 2 {
		fmt.Println("body", 111, n111)
		for x114 := range it1(2) {
			fmt.Println("body", 114, x114)
			for x117, s117 := range it2(2) {
				fmt.Println("body", 117, x117)
				fmt.Println("body", 117, s117)
				if x117%2 == 0 {
					continue L110
				}
			}
			fmt.Println("e119", n111, x114)
		}
	}
	{
		var fs []func() int
		for x121 := range it1(2) {
			fmt.Println("body", 121, x121)
			switch x121 % 3 {
case 0:
				{
					var fs []func() int
					for x125 := range it1(1) {
						fmt.Println("body", 125, x125)
						fs = append(fs, func() int { return x125 })
						if x121 > 2 {
							fmt.Println("e127", x121, x125)
							res = acc + 70
							return
						}
					}
					for _, f := range fs {
						fmt.Println("kept", f())
					}
				}
				fmt.Println("e128", x121)
case 1:
				fmt.Println("e129", x121)
				fmt.Println("e130", x121)
default:
				return acc + 38
			}
			if x121%3 == 0 {
				fmt.Println("e132")
			} else {
				for acc = range it1(1) {
					fmt.Println("body", 135)
					acc++
				}
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
	{
		var fs []func() int
		for range it1(2) {
			fmt.Println("body", 138)
			{
				var fs []func() int
				for i141 := 0; i141 < 2; i141++ {
					fmt.Println("body", 141, i141)
					fs = append(fs, func() int { return i141 })
					acc++
					fmt.Println("e143", i141)
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
} else {
	if acc%3 == 2 {
		fmt.Println("e145")
		fmt.Println("e146")
	}
	fmt.Println("e147")
}
if acc > 0 {
	goto end
}
if acc%2 == 0 {
	fmt.Println("e148")
	res = acc + 88
	return
}
end:
	res = acc
	return
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
top:
	steps++
	if steps > 3 {
		goto end
	}
if acc == 1 {
	fmt.Println("e149")
}
fmt.Println("e150")
L151:
for x152, s152 := range it2(3) {
	fmt.Println("body", 152, x152)
	fmt.Println("body", 152, s152)
	if acc == 1 {
		fmt.Println("e154", x152)
		goto top
	}
	if x152 == 2 {
		for range it1(2) {
			fmt.Println("body", 157)
			for acc = range it1(3) {
				fmt.Println("body", 160)
				if acc > 0 {
					return acc + 55
				}
			}
			continue L151
		}
		for n163 := range 1 {
			fmt.Println("body", 163, n163)
			for x166 := range it1(2) {
				fmt.Println("body", 166, x166)
				if acc%2 == 0 {
					fmt.Println("e169")
				} else {
					if acc == 2 {
						goto b167
					}
				}
b167:
			}
		}
		continue L151
	} else {
		acc++
		break L151
		for x173 := range it1(3) {
			fmt.Println("body", 173, x173)
			if x173 == 2 {
				fmt.Println("e175", x173)
				fmt.Println("e176", x173)
			}
			for x178, s178 := range it2(3) {
				fmt.Println("body", 178, x178)
				fmt.Println("body", 178, s178)
				if x178%3 == 0 {
					fmt.Println("e180", x173, x178)
					return acc + 95
				}
			}
		}
	}
	if x152 > 0 {
		fmt.Println("e181", x152)
		break L151
	}
}
fmt.Println("e182")
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
// e1
// body 3 0
// e5 0
// it1 done 1
// deferred acc 0
// f0 = 0
// e6
// body 8
// body 11
// e13
// body 15 0
// e17 0
// it1 done 1
// body 11
// e13
// body 15 0
// e17 0
// it1 done 1
// it1 done 2
// body 8
// body 11
// e13
// body 15 0
// e17 0
// it1 done 1
// body 11
// e13
// body 15 0
// e17 0
// it1 done 1
// it1 done 2
// body 8
// body 11
// e13
// body 15 0
// e17 0
// it1 done 1
// body 11
// e13
// body 15 0
// e17 0
// it1 done 1
// it1 done 2
// it1 done 3
// f1 = 1
// body 19 0
// it1 stopped at 0
// it1 done 2
// e47
// f2 = 0
// body 83 0
// it1 stopped at 0
// it1 done 1
// body 102 0
// it1 stopped at 0
// it1 done 2
// f3 = 83
// body 106 0
// body 106 0
// body 106 0
// deferred acc 0
// f4 = 0
// e150
// body 152 0
// body 152 s0
// it2 stopped at 0
// it2 done 3
// e182
// deferred acc 1
// f5 = 1
