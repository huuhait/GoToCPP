// phase: 06

// Uses of variables and functions of other packages as places and values: writes, op-assign, ++, swap, address-of, ranges, switch cases with constants of another package, closures and defers, calls of func variables, maps and slices of types of other packages.

package main

import (
	"fmt"

	"example.com/vars/cfg"
)

type local struct{ o cfg.Opt }

func touch() { cfg.Default.Level = 40 }

func main() {
	cfg.Default.Level = 3
	cfg.Default.Name += "!"
	cfg.Table[1]++
	cfg.Table[0], cfg.Table[3] = cfg.Table[3], cfg.Table[0]
	for i, n := range cfg.Names {
		fmt.Print(i, n, ";")
	}
	fmt.Println()
	for k, v := range cfg.Lookup {
		if k == "x" {
			fmt.Println(k, v)
		}
	}
	cfg.Names = append(cfg.Names, "delta")
	fmt.Println(len(cfg.Names), cap(cfg.Names) >= 4, cfg.Names[3])
	sw := func(n int) string {
		switch n {
		case cfg.Max:
			return "max"
		case cfg.Max - 1:
			return "max-1"
		}
		return "other"
	}
	fmt.Println(sw(3), sw(2), sw(1))
	defer fmt.Println("deferred", cfg.Size)
	defer func() { fmt.Println("closure", cfg.Default.Level) }()
	touch()
	p := &cfg.Table
	p[2] = 99
	q := &cfg.Table[1]
	*q = 77
	fmt.Println(cfg.Table, *p)
	l := local{cfg.Default}
	l.o.Bump()
	fmt.Println(l.o.Level, cfg.Default.Level)
	cfg.Hooks = append(cfg.Hooks, func() int { cfg.Size++; return cfg.Size })
	for _, h := range cfg.Hooks {
		fmt.Print(h(), ";")
	}
	fmt.Println()
	var h cfg.Handler = cfg.Upper
	fmt.Println(h("z"), cfg.Upper("y"))
	m := map[string]cfg.Opt{"a": cfg.Default}
	o := m["a"]
	o.Level = 0
	fmt.Println(m["a"].Level, o.Level)
	s := []cfg.Opt{cfg.Default, {"b", 2}}
	s[1].Bump()
	fmt.Println(s)
	x, y := cfg.Size, cfg.Default
	fmt.Println(x, y)
	cfg.Greeting = "bye"
	fmt.Println(cfg.Greeting)
}

// Output:
// cfg.init 30 hello alpha
// 0alpha;1beta;2gamma;
// x 1
// 4 true delta
// max max-1 other
// [4 77 99 1] [4 77 99 1]
// 41 40
// 30;31;
// <z> <y>
// 40 0
// [{def! 40} {b 3}]
// 31 {def! 40}
// bye
// closure 40
// deferred 30
