// phase: 05

// A value with an Error method prints through it (error wins over String); %v %s %q %x %X, Print
// and Println; pointer and value receivers; nil errors.
package main

import "fmt"

type E1 struct{ code int }

func (e E1) Error() string { return fmt.Sprintf("E1:%d", e.code) }

type EP struct{ msg string }

func (e *EP) Error() string { return "EP:" + e.msg }

type Both struct{}

func (Both) Error() string  { return "the error" }
func (Both) String() string { return "the string" }

type Code int

func (c Code) Error() string { return fmt.Sprintf("code %d", int(c)) }

type Chain struct {
	msg  string
	next error
}

func (c *Chain) Error() string {
	if c.next == nil {
		return c.msg
	}
	return c.msg + ": " + c.next.Error()
}

func (c *Chain) Unwrap() error { return c.next }

func main() {
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%10v|", "%-12s|", "%.3s"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, E1{7})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%10v|", "%-12s|", "%.3s"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, &EP{"m"})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%10v|", "%-12s|", "%.3s"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, Both{})
		fmt.Printf("]\n")
	}
	for _, f := range []string{"%v", "%s", "%d", "%x", "%q", "%#v", "%+v", "%10v|", "%-12s|", "%.3s"} {
		fmt.Printf("%-6s[", f)
		fmt.Printf(f, Code(4))
		fmt.Printf("]\n")
	}
	var err error = E1{1}
	fmt.Println(err, &EP{"x"}, Both{}, Code(2))
	var nilerr error
	fmt.Println(nilerr)
	fmt.Printf("%v %s %d %q|\n", nilerr, nilerr, nilerr, nilerr)
	c := &Chain{"outer", &Chain{"inner", E1{3}}}
	fmt.Println(c)
	fmt.Printf("%v|%s|%q\n", c, c, c)
	var ep *EP
	fmt.Println(ep == nil)
	fmt.Printf("%v\n", []error{E1{1}, &EP{"p"}, nil, Code(3)})
	fmt.Printf("%v\n", map[string]error{"a": E1{1}, "b": nil})
	fmt.Print(E1{1}, E1{2}, "s", Code(3), "\n")
}

// Output:
// %v    [E1:7]
// %s    [E1:7]
// %d    [{7}]
// %x    [45313a37]
// %q    ["E1:7"]
// %#v   [main.E1{code:7}]
// %+v   [E1:7]
// %10v| [      E1:7|]
// %-12s|[E1:7        |]
// %.3s  [E1:]
// %v    [EP:m]
// %s    [EP:m]
// %d    [&{%!d(string=m)}]
// %x    [45503a6d]
// %q    ["EP:m"]
// %#v   [&main.EP{msg:"m"}]
// %+v   [EP:m]
// %10v| [      EP:m|]
// %-12s|[EP:m        |]
// %.3s  [EP:]
// %v    [the error]
// %s    [the error]
// %d    [{}]
// %x    [746865206572726f72]
// %q    ["the error"]
// %#v   [main.Both{}]
// %+v   [the error]
// %10v| [ the error|]
// %-12s|[the error   |]
// %.3s  [the]
// %v    [code 4]
// %s    [code 4]
// %d    [4]
// %x    [636f64652034]
// %q    ["code 4"]
// %#v   [4]
// %+v   [code 4]
// %10v| [    code 4|]
// %-12s|[code 4      |]
// %.3s  [cod]
// E1:1 EP:x the error code 2
// <nil>
// <nil> %!s(<nil>) %!d(<nil>) %!q(<nil>)|
// outer: inner: E1:3
// outer: inner: E1:3|outer: inner: E1:3|"outer: inner: E1:3"
// true
// [E1:1 EP:p <nil> code 3]
// map[a:E1:1 b:<nil>]
// E1:1 E1:2scode 3
