// phase: 06

// Package variables (extern in pkg.hpp, defined in pkg_init.cpp), their initialization across packages and files, func values and method values of other packages, aliases and defined types over types of other packages, and the writes, address-of and ranges that name a variable of another package.

package main

import (
	"fmt"

	"example.com/vars/cfg"
	"example.com/vars/db"
	"example.com/vars/util"
)

var X = db.Z + 1

func init() { fmt.Println("main.init", X) }

func main() {
	fmt.Println(cfg.Names, cfg.Size, cfg.Table, cfg.Lookup["y"], len(cfg.Buf))
	fmt.Println(util.Apply(cfg.Upper, "go"), util.Apply(func(s string) string { return s + s }, "ab"))
	f := util.NewCounter
	c := f()
	c.Inc()
	inc := c.Inc
	fmt.Println(inc(), c.Get(), util.Counter.Get(*c), (*util.Counter).Inc(c))
	get := c.Get
	c.Inc()
	fmt.Println(get(), c.Get())
	for _, h := range cfg.Hooks {
		fmt.Println(h())
	}
	cfg.Table[2] = 30
	cfg.Lookup["z"] = 3
	fmt.Println(cfg.Table, len(cfg.Lookup))
	var cf db.Config = cfg.Default
	cf.Bump()
	fmt.Println(cf, cf.With(9), cfg.Default)
	l := db.Level(cf)
	fmt.Println(l.Level, db.Meters(4).Double(), db.Use(), db.Use())
	p := &cfg.Default
	p.Bump()
	fmt.Println(cfg.Default.Level)
	fn := cfg.Opt.With
	fmt.Println(fn(cfg.Default, 5).Level)
	arr := [cfg.Max]string{"a", "b", "c"}
	fmt.Println(arr, len(arr))
	cfg.Size += 5
	cfg.Size++
	fmt.Println(cfg.Size)
	var q *cfg.Opt = &cfg.Opt{"q", 1}
	q.Bump()
	fmt.Println(*q, q.Name)
}

// Output:
// cfg.init 30 hello alpha
// util.f
// util.init a 35 5 70
// util.init b
// main.init 39
// [alpha beta gamma] 30 [1 2 3 4] 2 6
// <go> abab
// 2 2 2 3
// 3 4
// 30
// [1 2 30 4] 3
// {def 3} {def 9} {def 2}
// 3 8 71 72
// 3
// 5
// [a b c] 3
// 36
// {q 2} q
