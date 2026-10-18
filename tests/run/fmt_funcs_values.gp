// phase: 05

// Func values print their code address (nil funcs print <nil>); %T names the func type; method
// values and closures; here only the shape of the output is checked.
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
type T struct{}

func (T) M(int) int { return 1 }

func f(a int, b string) bool { return true }

func main() {
	var nf func()
	g := func(x int) int { return x }
	fmt.Println(nf, norm(fmt.Sprint(f)) , norm(fmt.Sprint(g)), norm(fmt.Sprint(T{}.M)))
	fmt.Printf("%T %T %T %T\n", nf, f, g, T{}.M)
	fmt.Printf("%v %t\n", nf == nil, fmt.Sprint(f)[:2] == "0x")
	fmt.Println(norm(fmt.Sprintf("%v %+v %#v", f, f, f)))
	fmt.Println(norm(fmt.Sprint([]func(){nil, func() {}})), norm(fmt.Sprint(map[string]func(){"a": nil})))
	fmt.Println(norm(fmt.Sprint(struct{ F func() }{func() {}})))
	fmt.Printf("%d %s %x\n", nf, nf, nf)
	fmt.Println(fmt.Sprintf("%p", f) == fmt.Sprintf("%v", f), fmt.Sprintf("%p", nf))
}

// Output:
// <nil> 0xP 0xP 0xP
// func() func(int, string) bool func(int) int func(int) int
// true true
// 0xP 0xP (func(int, string) bool)(0xP)
// [<nil> 0xP] map[a:<nil>]
// {0xP}
// 0 %!s(func()=<nil>) 0
// true 0x0
