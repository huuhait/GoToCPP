package main

import "fmt"

type Mode uint8

const (
	Execute   Mode = 4
	Extra     Mode = 8
	ReadAlias      = Read
	Both           = Read | Write
)

func main() {
	fmt.Println(Read, Write, Execute, ReadAlias, Both, Everything)
	fmt.Println(Mode(0), Mode(6), Mode(255))
	fmt.Println(Late, AliasOfWrite, Mode(3).String())
	fmt.Printf("%v %d %s|%6s|\n", Execute, Execute, Both, Write)
}

func (m Mode) String() string {
	switch m {
	case Read:
		return "Read"
	case Write:
		return "Write"
	case Execute:
		return "Execute"
	case Both:
		return "Both"
	case Everything:
		return "Everything"
	case Extra:
		return "Extra"
	}
	n := uint64(m)
	digits := string(rune('0' + n%10))
	for n /= 10; n > 0; n /= 10 {
		digits = string(rune('0'+n%10)) + digits
	}
	return "Mode(" + digits + ")"
}
