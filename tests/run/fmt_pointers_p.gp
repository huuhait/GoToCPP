// phase: 05

// %p and %v of pointers print addresses (replaced by 0xP here); %d %x %o %b format the address as
// a number; slices, maps, funcs and unsafe pointers with %p; nil pointers.
package main

import "fmt"

// norm replaces every hexadecimal address (0x...) with 0xP: addresses differ from run to run.
func norm(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '0' && i+1 < len(s) && s[i+1] == 'x' {
			j := i + 2
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] >= 'a' && s[j] <= 'f') {
				j++
			}
			if j > i+2 {
				out = append(out, "0xP"...)
				i = j - 1
				continue
			}
		}
		out = append(out, s[i])
	}
	return string(out)
}
type T struct{ n int }

func main() {
	x := 1
	t := &T{1}
	sl := []int{1}
	m := map[string]int{}
	f := func() {}
	var np *int
	var ns []int
	var nm map[int]int
	var nf func()
	for _, f := range []string{"%p", "%v", "%+v", "%#v", "%s", "%t", "%T", "%10p|", "%-10p|", "%#x", "%#v"} {
		fmt.Printf("%-5s ", f)
		fmt.Println(norm(fmt.Sprintf(f, &x)), norm(fmt.Sprintf(f, t)), norm(fmt.Sprintf(f, np)))
	}
	for _, f := range []string{"%d", "%x", "%X", "%o", "%b", "%#p", "%08x", "%+d"} {
		fmt.Printf("%-5s %v %v\n", f, len(fmt.Sprintf(f, &x)) > 3, fmt.Sprintf(f, np))
	}
	fmt.Println(norm(fmt.Sprintf("%p %p %p", sl, m, f)), norm(fmt.Sprintf("%p %p %p", ns, nm, nf)))
	fmt.Println(norm(fmt.Sprintf("%v %v %v", &sl, &m, &f)))
	fmt.Println(fmt.Sprintf("%p", sl)[:2], fmt.Sprintf("%p", t)[:2], fmt.Sprintf("%p", &x) == fmt.Sprintf("%v", &x))
	fmt.Println(fmt.Sprintf("%d", &x) != "", fmt.Sprintf("%v", f)[:2], fmt.Sprintf("%v", nf), fmt.Sprintf("%v", ns))
	var a, b any = &x, &x
	fmt.Println(a == b, fmt.Sprintf("%p", a) == fmt.Sprintf("%p", b))
	fmt.Printf("%p %p\n", nil, 5)
}

// Output:
// %p    0xP 0xP 0xP
// %v    0xP &{1} <nil>
// %+v   0xP &{n:1} <nil>
// %#v   (*int)(0xP) &main.T{n:1} (*int)(nil)
// %s    %!s(*int=0xP) &{%!s(int=1)} %!s(*int=<nil>)
// %t    %!t(*int=0xP) &{%!t(int=1)} %!t(*int=<nil>)
// %T    *int *main.T *int
// %10p| 0xP| 0xP|        0xP|
// %-10p| 0xP| 0xP| 0xP       |
// %#x   0xP &{0xP} 0xP
// %#v   (*int)(0xP) &main.T{n:1} (*int)(nil)
// %d    true 0
// %x    true 0
// %X    true 0
// %o    true 0
// %b    true 0
// %#p   true 0
// %08x  true 00000000
// %+d   true +0
// 0xP 0xP 0xP 0xP 0xP 0xP
// &[1] &map[] 0xP
// 0x 0x true
// true 0x <nil> []
// true true
// %!p(<nil>) %!p(int=5)
