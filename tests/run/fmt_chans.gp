// phase: 08

// Channels print their address (<nil> for a nil channel) and %T names the direction; %d %x work
// on the address; a channel in a struct, slice, map and any; named channel types.
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
type Pipe chan int

type Holder struct {
	C  chan string
	R  <-chan int
	S  chan<- bool
	c  chan int
}

func main() {
	var nc chan int
	c := make(chan int, 2)
	var r <-chan int = c
	var s chan<- int = c
	p := make(Pipe, 1)
	fmt.Println(nc, nc == nil)
	fmt.Printf("%T %T %T %T %T\n", nc, c, r, s, p)
	fmt.Printf("%v %v %v %t\n", fmt.Sprint(c)[:2], fmt.Sprint(r)[:2], fmt.Sprint(s)[:2], fmt.Sprint(c) == fmt.Sprint(r))
	fmt.Println(fmt.Sprintf("%p", c) == fmt.Sprint(c), fmt.Sprintf("%p", nc), fmt.Sprintf("%d", nc))
	fmt.Println(norm(fmt.Sprint(Holder{c: c})), norm(fmt.Sprint(Holder{})))
	fmt.Println(norm(fmt.Sprintf("%+v", Holder{C: make(chan string)})))
	fmt.Println(norm(fmt.Sprint([]chan int{c, nil})), norm(fmt.Sprint(map[string]chan int{"a": c})))
	var a any = c
	fmt.Printf("%T %t\n", a, a == any(c))
	fmt.Printf("%#v %#v\n", nc, [1]chan int{})
	fmt.Println(norm(fmt.Sprintf("%v %s", c, c)))
}

// Output:
// <nil> true
// chan int chan int <-chan int chan<- int main.Pipe
// 0x 0x 0x true
// true 0x0 0
// {<nil> <nil> <nil> 0xP} {<nil> <nil> <nil> <nil>}
// {C:0xP R:<nil> S:<nil> c:<nil>}
// [0xP <nil>] map[a:0xP]
// chan int true
// (chan int)(nil) [1]chan int{(chan int)(nil)}
// 0xP %!s(chan int=0xP)
