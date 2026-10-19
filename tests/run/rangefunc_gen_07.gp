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
for range it1(2) {
	fmt.Println("body", 2)
	for range it1(1) {
		fmt.Println("body", 5)
		if acc%2 == 1 {
			return acc + 59
		}
	}
}
{
	var fs []func() int
	for x8, s8 := range it2(2) {
		fmt.Println("body", 8, x8)
		fmt.Println("body", 8, s8)
		fs = append(fs, func() int { return x8 })
		acc++
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
{
	var fs []func() int
	for x11 := range it1(1) {
		fmt.Println("body", 11, x11)
		for x14 := range it1(3) {
			fmt.Println("body", 14, x14)
			for x17 := range it1(1) {
				fmt.Println("body", 17, x17)
				defer fmt.Println("defer", 19, acc)
				for n21 := range 3 {
					fmt.Println("body", 21, n21)
					if x14 > 2 {
						if acc == 0 {
							return acc + 76
						}
					} else {
						if acc%2 == 0 {
							fmt.Println("e25")
						}
					}
				}
			}
			for range it0(3) {
				fmt.Println("body", 27)
				fmt.Println("e29", x11, x14)
				for n31 := range 1 {
					fmt.Println("body", 31, n31)
					fmt.Println("e33", x11, x14, n31)
				}
				fmt.Println("e34", x11, x14)
			}
		}
		{
			var fs []func() int
			for x36 := range it1(3) {
				fmt.Println("body", 36, x36)
				fs = append(fs, func() int { return x36 })
L38:
				for range it1(3) {
					fmt.Println("body", 39)
					for x42 := range it1(2) {
						fmt.Println("body", 42, x42)
						fmt.Println("e44", x11, x36, x42)
					}
					fmt.Println("e45", x11, x36)
					switch x36 % 3 {
case 0:
						defer fmt.Println("defer", 47, acc)
						acc += x36 % 5
case 1:
						acc++
						acc += x36 % 5
default:
						break L38
						fmt.Println("e48", x11, x36)
					}
				}
			}
			for _, f := range fs {
				fmt.Println("kept", f())
			}
		}
		acc += x11 % 5
	}
	for _, f := range fs {
		fmt.Println("kept", f())
	}
}
L49:
switch acc % 3 {
case 0:
	for x51 := range it1(2) {
		fmt.Println("body", 51, x51)
		for x54 := range it1(3) {
			fmt.Println("body", 54, x54)
			if x54 > 1 {
				acc++
			} else {
				fmt.Println("e58")
			}
			if x54 > 1 {
				if acc%2 == 0 {
					return acc + 91
				}
			} else {
				goto end
			}
		}
		fmt.Println("e61", x51)
		fmt.Println("e62", x51)
	}
case 1:
	for n64 := range 2 {
		fmt.Println("body", 64, n64)
		for acc = range it1(3) {
			fmt.Println("body", 67)
			fmt.Println("e69", n64)
			switch n64 % 3 {
case 0:
				acc += n64 % 5
				if acc > 2 {
					if acc%3 == 1 {
						if acc > 1 {
							break
						}
					} else {
						if acc%3 == 0 {
							fmt.Println("e74")
							return acc + 13
						}
					}
				} else {
					if acc > 2 {
						goto top
					}
				}
case 1:
				if acc%3 == 0 {
					break
				}
			}
		}
		fmt.Println("e76", n64)
	}
default:
L77:
	switch acc % 3 {
case 0:
		switch acc % 3 {
case 0:
			switch acc % 3 {
case 0:
				if acc%3 == 1 {
					break
				}
				if acc%2 == 0 {
					break L77
				}
case 1:
				if acc > 0 {
					break L49
				}
default:
				if acc%2 == 0 {
					fmt.Println("e81")
				} else {
					if acc%2 == 0 {
						break L77
					}
				}
			}
case 1:
			for x84 := range it1(1) {
				fmt.Println("body", 84, x84)
				if x84%3 == 0 {
					fmt.Println("e87")
				} else {
					if acc%3 == 0 {
						goto top
					}
				}
			}
		}
		for range it1(2) {
			fmt.Println("body", 90)
			if acc > 0 {
				fmt.Println("e92")
			}
			fmt.Println("e93")
		}
case 1:
		if acc > 2 {
			if acc%2 == 1 {
				return acc + 30
			}
		} else {
			fmt.Println("e96")
		}
		if acc%2 == 1 {
			fmt.Println("e97")
			return acc + 88
		}
	}
	{
		var fs []func() int
		for n99 := range 2 {
			fmt.Println("body", 99, n99)
			fs = append(fs, func() int { return n99 })
			if n99 > 2 {
				for acc = range it1(3) {
					fmt.Println("body", 103)
					fmt.Println("e105")
				}
			} else {
				switch acc % 3 {
case 0:
					if acc%3 == 2 {
						break L49
					}
case 1:
					if acc > 3 {
						break L49
					}
					fmt.Println("e108")
default:
					fmt.Println("e109")
					return acc + 73
				}
			}
		}
		for _, f := range fs {
			fmt.Println("kept", f())
		}
	}
}
end:
	return acc
}

func f1() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e110")
for i112 := 0; i112 < 3; i112++ {
	fmt.Println("body", 112, i112)
	fmt.Println("e114", i112)
	fmt.Println("e115", i112)
	switch i112 % 3 {
case 0:
		for x118 := range it1(3) {
			fmt.Println("body", 118, x118)
			if x118 > 0 {
				break
			}
			if acc%2 == 0 {
				fmt.Println("e120", i112, x118)
				break
			}
		}
case 1:
		if acc == 0 {
			fmt.Println("e121", i112)
		}
		if i112 > 1 {
			for i124 := 0; i124 < 2; i124++ {
				fmt.Println("body", 124, i124)
				defer fmt.Println("defer", 126, acc)
			}
		} else {
			for i129 := 0; i129 < 2; i129++ {
				fmt.Println("body", 129, i129)
				fmt.Println("e131", i129)
			}
		}
	}
}
	return acc
}

func f2() int {
	acc, steps := 0, 0
	_ = steps
fmt.Println("e132")
if acc > 3 {
	fmt.Println("e133")
}
	return acc
}

func f3() (res int) {
	acc, steps := 0, 0
	_ = steps
for x135 := range it1(1) {
	fmt.Println("body", 135, x135)
	acc += x135 % 5
	if x135 == 1 {
		fmt.Println("e137", x135)
		fmt.Println("e138", x135)
	}
}
{
	var fs []func() int
	for i140 := 0; i140 < 1; i140++ {
		fmt.Println("body", 140, i140)
		fmt.Println("e142", i140)
		fmt.Println("e143", i140)
		{
			var fs []func() int
			for x145 := range it1(1) {
				fmt.Println("body", 145, x145)
				fs = append(fs, func() int { return x145 })
				if x145%3 == 2 {
					continue
				}
				break
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
fmt.Println("e147")
if acc > 1 {
	fmt.Println("e148")
	fmt.Println("e149")
}
	res = acc
	return
}

func f4() (res int) {
	acc, steps := 0, 0
	_ = steps
for range it1(1) {
	fmt.Println("body", 151)
	defer fmt.Println("defer", 153, acc)
	if acc%3 == 2 {
		break
	}
}
for i155 := 0; i155 < 1; i155++ {
	fmt.Println("body", 155, i155)
	acc++
	if i155%3 == 1 {
		fmt.Println("e157", i155)
		continue
	}
}
	res = acc
	return
}

func f5() (res int) {
	acc, steps := 0, 0
	_ = steps
top:
	steps++
	if steps > 3 {
		goto end
	}
fmt.Println("e158")
goto end
switch acc % 3 {
case 0:
	fmt.Println("e160")
	if acc > 2 {
		fmt.Println("e161")
	}
case 1:
	fmt.Println("e162")
	for n164 := range 2 {
		fmt.Println("body", 164, n164)
		switch n164 % 3 {
case 0:
			fmt.Println("e167", n164)
case 1:
			if n164 == 3 {
				continue
			}
			fmt.Println("e168", n164)
		}
	}
default:
	fmt.Println("e169")
}
{
	var fs []func() int
L170:
	for acc = range it1(1) {
		fmt.Println("body", 171)
		break L170
		{
			var fs []func() int
			for acc = range it1(2) {
				fmt.Println("body", 174)
				switch acc % 3 {
case 0:
					{
						var fs []func() int
						for n178 := range 1 {
							fmt.Println("body", 178, n178)
							fs = append(fs, func() int { return n178 })
							if acc%3 == 1 {
								fmt.Println("e180", n178)
								goto top
							}
						}
						for _, f := range fs {
							fmt.Println("kept", f())
						}
					}
					fmt.Println("e181")
case 1:
					if acc%2 == 1 {
						fmt.Println("e182")
						goto top
					}
					for range it0(1) {
						fmt.Println("body", 184)
						fmt.Println("e186")
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
// body 5
// it1 done 1
// body 2
// body 5
// it1 done 1
// it1 done 2
// body 8 0
// body 8 s0
// body 8 10
// body 8 s1
// it2 done 2
// kept 0
// kept 10
// body 11 0
// body 14 0
// body 17 0
// body 21 0
// e25
// body 21 1
// e25
// body 21 2
// e25
// it1 done 1
// body 27
// e29 0 0
// body 31 0
// e33 0 0 0
// e34 0 0
// body 27
// e29 0 0
// body 31 0
// e33 0 0 0
// e34 0 0
// body 27
// e29 0 0
// body 31 0
// e33 0 0 0
// e34 0 0
// it0 done 3
// body 14 1
// body 17 0
// body 21 0
// e25
// body 21 1
// e25
// body 21 2
// e25
// it1 done 1
// body 27
// e29 0 1
// body 31 0
// e33 0 1 0
// e34 0 1
// body 27
// e29 0 1
// body 31 0
// e33 0 1 0
// e34 0 1
// body 27
// e29 0 1
// body 31 0
// e33 0 1 0
// e34 0 1
// it0 done 3
// body 14 2
// body 17 0
// body 21 0
// e25
// body 21 1
// e25
// body 21 2
// e25
// it1 done 1
// body 27
// e29 0 2
// body 31 0
// e33 0 2 0
// e34 0 2
// body 27
// e29 0 2
// body 31 0
// e33 0 2 0
// e34 0 2
// body 27
// e29 0 2
// body 31 0
// e33 0 2 0
// e34 0 2
// it0 done 3
// it1 done 3
// body 36 0
// body 39
// body 42 0
// e44 0 0 0
// body 42 1
// e44 0 0 1
// it1 done 2
// e45 0 0
// body 39
// body 42 0
// e44 0 0 0
// body 42 1
// e44 0 0 1
// it1 done 2
// e45 0 0
// body 39
// body 42 0
// e44 0 0 0
// body 42 1
// e44 0 0 1
// it1 done 2
// e45 0 0
// it1 done 3
// body 36 1
// body 39
// body 42 0
// e44 0 1 0
// body 42 1
// e44 0 1 1
// it1 done 2
// e45 0 1
// body 39
// body 42 0
// e44 0 1 0
// body 42 1
// e44 0 1 1
// it1 done 2
// e45 0 1
// body 39
// body 42 0
// e44 0 1 0
// body 42 1
// e44 0 1 1
// it1 done 2
// e45 0 1
// it1 done 3
// body 36 2
// body 39
// body 42 0
// e44 0 2 0
// body 42 1
// e44 0 2 1
// it1 done 2
// e45 0 2
// it1 stopped at 0
// it1 done 3
// it1 done 3
// kept 0
// kept 1
// kept 2
// it1 done 1
// body 99 0
// e109
// defer 47 2
// defer 47 2
// defer 47 2
// defer 19 2
// defer 19 2
// defer 19 2
// deferred acc 8
// f0 = 81
// e110
// body 112 0
// e114 0
// e115 0
// body 118 0
// e120 0 0
// it1 stopped at 0
// it1 done 3
// body 112 1
// e114 1
// e115 1
// e121 1
// body 129 0
// e131 0
// body 129 1
// e131 1
// body 112 2
// e114 2
// e115 2
// f1 = 0
// e132
// f2 = 0
// body 135 0
// it1 done 1
// body 140 0
// e142 0
// e143 0
// body 145 0
// it1 stopped at 0
// it1 done 1
// kept 0
// e147
// f3 = 0
// body 151
// it1 done 1
// body 155 0
// defer 153 0
// f4 = 1
// e158
// f5 = 0
