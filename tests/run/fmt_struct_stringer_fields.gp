// phase: 05

// fmt calls String and Error on fields it can reach with reflection: exported fields yes,
// unexported fields no (CanInterface is false), also inside slices, maps and pointers; an
// exported field of an unexported embedded struct is open again.
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
type Money int

func (m Money) String() string { return fmt.Sprintf("$%d", int(m)) }

type Err struct{ msg string }

func (e Err) Error() string { return "E:" + e.msg }

type Acct struct {
	Pub   Money
	priv  Money
	Pubs  []Money
	privs []Money
	M     map[string]Money
	m     map[string]Money
	E     error
	e     error
	A     any
	a     any
	P     *Money
	p     *Money
	S     fmt.Stringer
	s     fmt.Stringer
}

type hidden struct {
	Shown  Money
	hidden Money
}

type Outer struct {
	hidden
	Top Money
}

type Outer2 struct {
	Hidden hidden
	hid    hidden
}

func main() {
	m := Money(5)
	a := Acct{1, 2, []Money{3}, []Money{4}, map[string]Money{"k": 5}, map[string]Money{"k": 6}, Err{"a"}, Err{"b"}, Money(7), Money(8), &m, &m, Money(9), Money(10)}
	fmt.Println(norm(fmt.Sprintf("%v", a)))
	fmt.Println(norm(fmt.Sprintf("%+v", a)))
	o := Outer{hidden{1, 2}, 3}
	fmt.Printf("%v %+v\n", o, o)
	o2 := Outer2{hidden{1, 2}, hidden{3, 4}}
	fmt.Printf("%v %+v\n", o2, o2)
	fmt.Printf("%v %v\n", []hidden{{1, 2}}, map[string]hidden{"k": {3, 4}})
	fmt.Printf("%s %d\n", Acct{Pub: 1}.Pub, Acct{Pub: 1}.Pub)
}

// Output:
// {$1 2 [$3] [4] map[k:$5] map[k:6] E:a {b} $7 8 $5 0xP $9 10}
// {Pub:$1 priv:2 Pubs:[$3] privs:[4] M:map[k:$5] m:map[k:6] E:E:a e:{msg:b} A:$7 a:8 P:$5 p:0xP S:$9 s:10}
// {{$1 2} $3} {hidden:{Shown:$1 hidden:2} Top:$3}
// {{$1 2} {3 4}} {Hidden:{Shown:$1 hidden:2} hid:{Shown:3 hidden:4}}
// [{$1 2}] map[k:{$3 4}]
// $1 1
