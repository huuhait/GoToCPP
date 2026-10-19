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
fmt.Println("e1")
{
	var fs []func() int
L2:
	for n3 := range 1 {
		fmt.Println("body", 3, n3)
		if n3%3 == 1 {
			if acc%3 == 1 {
				if acc == 2 {
					fmt.Println("e7")
				}
			} else {
				continue
			}
			if acc%2 == 1 {
				break
			}
		} else {
L10:
			for n11 := range 3 {
				fmt.Println("body", 11, n11)
				if n11%2 == 1 {
					break
				}
				if n11%2 == 1 {
					break L10
				}
				if acc%3 == 2 {
					fmt.Println("e13", n11)
					return acc + 82
				}
			}
			{
				var fs []func() int
				for x15 := range it1(1) {
					fmt.Println("body", 15, x15)
					{
						var fs []func() int
L17:
						for x18 := range it1(1) {
							fmt.Println("body", 18, x18)
							fs = append(fs, func() int { return x18 })
							if x15 > 1 {
								continue L17
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
			for acc = range it1(1) {
				fmt.Println("body", 21)
				fmt.Println("e23")
			}
		}
		if n3%3 == 2 {
			fmt.Println("e25")
		} else {
			break L2
			goto b4
		}
		for i28 := 0; i28 < 1; i28++ {
			fmt.Println("body", 28, i28)
			fmt.Println("e30", n3, i28)
L31:
			for n32 := range 2 {
				fmt.Println("body", 32, n32)
				for x35 := range it1(1) {
					fmt.Println("body", 35, x35)
					if i28%3 == 2 {
						fmt.Println("e37", n3, i28, n32, x35)
						break
					}
				}
				continue L31
			}
			return acc + 8
		}
b4:
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
fmt.Println("e38")
	return acc
}

func f1() (res int) {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e39")
fmt.Println("e40")
if acc%2 == 0 {
	fmt.Println("e42")
} else {
	acc++
	fmt.Println("e44")
}
	res = acc
	return
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
if acc == 3 {
	fmt.Println("e45")
}
L46:
switch acc % 3 {
case 0:
	goto top
	for x48 := range it1(2) {
		fmt.Println("body", 48, x48)
		defer fmt.Println("defer", 50, acc)
	}
case 1:
	if acc%3 == 0 {
		fmt.Println("e52")
		fmt.Println("e53")
	} else {
		fmt.Println("e55")
		switch acc % 3 {
case 0:
			break
L57:
			for range it1(2) {
				fmt.Println("body", 58)
				if acc%3 == 0 {
					continue L57
				}
			}
case 1:
			fmt.Println("e60")
			defer fmt.Println("defer", 61, acc)
default:
			fmt.Println("e62")
		}
	}
	if acc%3 == 1 {
		goto end
		if acc%2 == 0 {
			goto end
		}
		for acc = range it1(1) {
			fmt.Println("body", 65)
			for x68 := range it1(3) {
				fmt.Println("body", 68, x68)
				acc += x68 % 5
			}
			if acc == 1 {
				return acc + 47
			}
			if acc%2 == 0 {
				if acc%3 == 2 {
					return acc + 5
				}
			} else {
				fmt.Println("e72")
			}
		}
	} else {
		if acc > 2 {
			goto top
		}
		for x75 := range it1(3) {
			fmt.Println("body", 75, x75)
			fmt.Println("e77", x75)
			if x75%2 == 1 {
				goto b76
			}
			for n79 := range 3 {
				fmt.Println("body", 79, n79)
				fmt.Println("e81", x75, n79)
			}
b76:
		}
		for x83, s83 := range it2(2) {
			fmt.Println("body", 83, x83)
			fmt.Println("body", 83, s83)
			fmt.Println("e85", x83)
			if x83%3 == 2 {
				break L46
			}
		}
	}
}
end:
	return acc
}

func f3() int {
	acc, steps := 0, 0
	_ = steps
if acc > 0 {
	fmt.Println("e86")
}
fmt.Println("e87")
for x89 := range it1(1) {
	fmt.Println("body", 89, x89)
	for n92 := range 2 {
		fmt.Println("body", 92, n92)
		return acc + 72
	}
}
fmt.Println("e94")
	return acc
}

func f4() int {
	acc, steps := 0, 0
	_ = steps
	defer func() { fmt.Println("deferred acc", acc) }()
acc++
fmt.Println("e95")
	return acc
}

func f5() int {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
for n97 := range 1 {
	fmt.Println("body", 97, n97)
	acc += n97 % 5
	fmt.Println("e99", n97)
}
{
	var fs []func() int
L100:
	for range it0(3) {
		fmt.Println("body", 101)
		if acc > 1 {
			return acc + 87
		}
		{
			var fs []func() int
L103:
			for range it0(3) {
				fmt.Println("body", 104)
				continue L100
				{
					var fs []func() int
					for x107 := range it1(3) {
						fmt.Println("body", 107, x107)
						{
							var fs []func() int
							for x110 := range it1(1) {
								fmt.Println("body", 110, x110)
								fs = append(fs, func() int { return x110 })
								fmt.Println("e112", x107, x110)
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
				{
					var fs []func() int
					for n114 := range 3 {
						fmt.Println("body", 114, n114)
						fs = append(fs, func() int { return n114 })
						if acc%3 == 2 {
							goto top
						}
						if n114%3 == 2 {
							continue L103
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
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
for x117 := range it1(1) {
	fmt.Println("body", 117, x117)
	if x117 > 3 {
		continue
	}
}
if acc > 2 {
	goto top
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
// e1
// body 3 0
// body 11 0
// body 11 1
// body 15 0
// body 18 0
// it1 done 1
// kept 0
// it1 done 1
// body 21
// e23
// it1 done 1
// e38
// deferred acc 0
// f0 = 0
// e39
// e40
// e42
// f1 = 0
// f2 = 0
// e87
// body 89 0
// body 92 0
// it1 stopped at 0
// it1 done 1
// f3 = 72
// e95
// deferred acc 1
// f4 = 1
// body 97 0
// e99 0
// body 101
// body 104
// it0 stopped at 0
// it0 done 3
// body 101
// body 104
// it0 stopped at 0
// it0 done 3
// body 101
// body 104
// it0 stopped at 0
// it0 done 3
// it0 done 3
// body 117 0
// it1 done 1
// f5 = 0
