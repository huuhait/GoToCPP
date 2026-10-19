// phase: 09

// The flag types: every Var function of FlagSet and of the command line, Value and Getter, Func, BoolFunc, TextVar,
// UnquoteUsage, the defaults printed by PrintDefaults for each kind, Var panics and the zero value check of String.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type color int

func (c *color) String() string {
	if c == nil {
		return "nil"
	}
	return [...]string{"red", "green", "blue"}[*c]
}

func (c *color) Set(s string) error {
	switch s {
	case "red":
		*c = 0
	case "green":
		*c = 1
	case "blue":
		*c = 2
	default:
		return errors.New("not a color")
	}
	return nil
}

type temp float64

func (t *temp) UnmarshalText(b []byte) error {
	if string(b) == "hot" {
		*t = 100
		return nil
	}
	if string(b) == "cold" {
		*t = -10
		return nil
	}
	return fmt.Errorf("bad temp %q", b)
}

func (t temp) MarshalText() ([]byte, error) {
	if t == 0 {
		return []byte("zero"), nil
	}
	return []byte(fmt.Sprintf("%gC", float64(t))), nil
}

func main() {
	fs := flag.NewFlagSet("values", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)

	var (
		bv  bool
		iv  int
		i64 int64
		uv  uint
		u64 uint64
		sv  string
		fv  float64
		dv  time.Duration
		col color
		tmp temp
	)
	fs.BoolVar(&bv, "bool", true, "a bool")
	fs.IntVar(&iv, "int", -4, "an int")
	fs.Int64Var(&i64, "int64", 1<<40, "an int64")
	fs.UintVar(&uv, "uint", 7, "a uint")
	fs.Uint64Var(&u64, "uint64", 1<<63, "a uint64")
	fs.StringVar(&sv, "string", "hi there", "a `name` for the thing")
	fs.Float64Var(&fv, "float", 1.5e-7, "a float")
	fs.DurationVar(&dv, "dur", 1500*time.Millisecond, "a duration")
	fs.Var(&col, "color", "one of `red`, `green` or `blue`")
	fs.TextVar(&tmp, "temp", temp(21.5), "the temperature")
	fs.TextVar(&tmp, "temp0", temp(0), "zero temperature")
	var got []string
	fs.Func("fn", "calls a function with the `word`", func(s string) error {
		got = append(got, "fn:"+s)
		if s == "bad" {
			return errors.New("bad word")
		}
		return nil
	})
	fs.BoolFunc("bfn", "calls a function without a value", func(s string) error {
		got = append(got, "bfn:"+s)
		return nil
	})
	fs.PrintDefaults()

	fmt.Println("--- parse")
	err := fs.Parse([]string{"-bool=false", "-int", "-0x1f", "-int64=-9", "-uint", "0b11", "-uint64", "18446744073709551615",
		"-string", "s s", "-float=.25", "-dur", "-1m2.5s", "-color", "blue", "-temp", "hot", "-fn", "a", "-bfn", "-bfn=x", "-fn=b", "tail"})
	fmt.Println(err, bv, iv, i64, uv, u64, sv, fv, dv, col.String(), tmp, got, fs.Args())

	fmt.Println("--- errors")
	fs.Usage = func() { fmt.Println("(usage)") }
	for _, a := range []string{"-color=pink", "-temp=warm", "-fn=bad", "-uint=-1", "-int64=", "-dur=", "-bool=", "-string"} {
		fmt.Println(a, "=>", fs.Parse([]string{a}))
	}

	fmt.Println("--- Getter")
	fs.VisitAll(func(f *flag.Flag) {
		g, ok := f.Value.(flag.Getter)
		if ok && strings.HasPrefix(f.Name, "temp") {
			fmt.Printf("%s: %T\n", f.Name, g.Get()) // a pointer: no address in the output
		} else if ok {
			fmt.Printf("%s: %T %v\n", f.Name, g.Get(), g.Get())
		} else {
			fmt.Printf("%s: not a Getter (%T) %q\n", f.Name, f.Value, f.Value)
		}
	})

	fmt.Println("--- UnquoteUsage")
	fs.VisitAll(func(f *flag.Flag) {
		name, usage := flag.UnquoteUsage(f)
		fmt.Printf("%s: %q %q\n", f.Name, name, usage)
	})

	fmt.Println("--- pointer forms")
	p := flag.NewFlagSet("ptr", flag.ContinueOnError)
	pb, pi, pi64 := p.Bool("b", false, ""), p.Int("i", 1, ""), p.Int64("i64", 2, "")
	pu, pu64, ps := p.Uint("u", 3, ""), p.Uint64("u64", 4, ""), p.String("s", "five", "")
	pf, pd := p.Float64("f", 6, ""), p.Duration("d", 7, "")
	fmt.Println(*pb, *pi, *pi64, *pu, *pu64, *ps, *pf, *pd)
	p.Parse([]string{"-b", "-i=-1", "-i64=-2", "-u=3", "-u64=4", "-s=x", "-f=5", "-d=6ns"})
	fmt.Println(*pb, *pi, *pi64, *pu, *pu64, *ps, *pf, *pd)

	fmt.Println("--- panics")
	for _, name := range []string{"", "-x", "a=b", "col"} {
		func() {
			defer func() { fmt.Printf("%q: recovered %v\n", name, recover()) }()
			q := flag.NewFlagSet("q", flag.ContinueOnError)
			q.SetOutput(os.Stdout)
			var c color
			q.Var(&c, "col", "")
			q.Var(&c, name, "")
		}()
	}
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		q := flag.NewFlagSet("", flag.ContinueOnError)
		q.SetOutput(os.Stdout)
		q.Int("x", 0, "")
		q.Int("x", 0, "")
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		q := flag.NewFlagSet("q", flag.ContinueOnError)
		q.TextVar(new(temp), "t", other{}, "")
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		q := flag.NewFlagSet("q", flag.ContinueOnError)
		q.TextVar(valText{}, "t", valText{}, "")
	}()

	fmt.Println("--- a String method that panics on the zero value")
	q := flag.NewFlagSet("z", flag.ContinueOnError)
	q.SetOutput(os.Stdout)
	q.Var(&panicky{v: "x"}, "pk", "panics when zero")
	q.PrintDefaults()
	fmt.Println(strings.Count("done", "o"))
}

type other struct{}

func (other) MarshalText() ([]byte, error) { return []byte("other"), nil }

type valText struct{}

func (valText) MarshalText() ([]byte, error)  { return []byte("v"), nil }
func (valText) UnmarshalText(b []byte) error { return nil }

type panicky struct{ v string }

func (p *panicky) Set(s string) error { p.v = s; return nil }
func (p *panicky) String() string {
	if p == nil || p.v == "" {
		panic("no value")
	}
	return p.v
}

// Output:
//   -bfn
//     	calls a function without a value
//   -bool
//     	a bool (default true)
//   -color red
//     	one of red, `green` or `blue`
//   -dur duration
//     	a duration (default 1.5s)
//   -float float
//     	a float (default 1.5e-07)
//   -fn word
//     	calls a function with the word
//   -int int
//     	an int (default -4)
//   -int64 int
//     	an int64 (default 1099511627776)
//   -string name
//     	a name for the thing (default "hi there")
//   -temp value
//     	the temperature (default 21.5C)
//   -temp0 value
//     	zero temperature (default zero)
//   -uint uint
//     	a uint (default 7)
//   -uint64 uint
//     	a uint64 (default 9223372036854775808)
// --- parse
// <nil> false -31 -9 3 18446744073709551615 s s 0.25 -1m2.5s blue 100 [fn:a bfn:true bfn:x fn:b] [tail]
// --- errors
// invalid value "pink" for flag -color: not a color
// (usage)
// -color=pink => invalid value "pink" for flag -color: not a color
// invalid value "warm" for flag -temp: bad temp "warm"
// (usage)
// -temp=warm => invalid value "warm" for flag -temp: bad temp "warm"
// invalid value "bad" for flag -fn: bad word
// (usage)
// -fn=bad => invalid value "bad" for flag -fn: bad word
// invalid value "-1" for flag -uint: parse error
// (usage)
// -uint=-1 => invalid value "-1" for flag -uint: parse error
// invalid value "" for flag -int64: parse error
// (usage)
// -int64= => invalid value "" for flag -int64: parse error
// invalid value "" for flag -dur: parse error
// (usage)
// -dur= => invalid value "" for flag -dur: parse error
// invalid boolean value "" for -bool: parse error
// (usage)
// -bool= => invalid boolean value "" for -bool: parse error
// flag needs an argument: -string
// (usage)
// -string => flag needs an argument: -string
// --- Getter
// bfn: not a Getter (flag.boolFuncValue) ""
// bool: bool false
// color: not a Getter (*main.color) "blue"
// dur: time.Duration 0s
// float: float64 0.25
// fn: not a Getter (flag.funcValue) ""
// int: int -31
// int64: int64 0
// string: string s s
// temp: *main.temp
// temp0: *main.temp
// uint: uint 0
// uint64: uint64 18446744073709551615
// --- UnquoteUsage
// bfn: "" "calls a function without a value"
// bool: "" "a bool"
// color: "red" "one of red, `green` or `blue`"
// dur: "duration" "a duration"
// float: "float" "a float"
// fn: "word" "calls a function with the word"
// int: "int" "an int"
// int64: "int" "an int64"
// string: "name" "a name for the thing"
// temp: "value" "the temperature"
// temp0: "value" "zero temperature"
// uint: "uint" "a uint"
// uint64: "uint" "a uint64"
// --- pointer forms
// false 1 2 3 4 five 6 7ns
// true -1 -2 3 4 x 5 6ns
// --- panics
// "": recovered <nil>
// flag "-x" begins with -
// "-x": recovered flag "-x" begins with -
// flag "a=b" contains =
// "a=b": recovered flag "a=b" contains =
// q flag redefined: col
// "col": recovered q flag redefined: col
// flag redefined: x
// recovered: flag redefined: x
// recovered: default type does not match variable type: main.other != main.temp
// recovered: variable value type must be a pointer
// --- a String method that panics on the zero value
//   -pk value
//     	panics when zero
//
// panic calling String method on zero main.panicky for flag pk: no value
// 1
