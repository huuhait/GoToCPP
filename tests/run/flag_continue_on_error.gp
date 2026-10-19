// phase: 09

// FlagSet with ContinueOnError: the value forms (-f, --f, -f=v, -f v), "--" and "-" as arguments, every error text and
// what goes to the output (here os.Stdout), -h and -help, Visit/VisitAll/Lookup/Set/NFlag/NArg/Arg/Args and the usage text.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func newSet() (*flag.FlagSet, *int, *string, *bool, *float64, *time.Duration) {
	fs := flag.NewFlagSet("tool", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	n := fs.Int("n", 1, "the `count`")
	s := fs.String("s", "dflt", "a string")
	b := fs.Bool("b", false, "a bool")
	f := fs.Float64("f", 0.5, "a float")
	d := fs.Duration("d", 90*time.Second, "a duration")
	return fs, n, s, b, f, d
}

func run(args ...string) {
	fs, n, s, b, f, d := newSet()
	err := fs.Parse(args)
	fmt.Printf("%q -> err=%v n=%d s=%q b=%v f=%v d=%v args=%q narg=%d nflag=%d parsed=%v\n",
		args, err, *n, *s, *b, *f, *d, fs.Args(), fs.NArg(), fs.NFlag(), fs.Parsed())
	if errors.Is(err, flag.ErrHelp) {
		fmt.Println("(help)")
	}
}

func main() {
	run()
	run("-n", "5")
	run("--n=7", "-s=")
	run("-s", "a b", "-b", "x")
	run("-b=false", "-b=1", "--b=T")
	run("-b", "false")
	run("-b", "true", "x")
	run("-b=true", "true")
	run("-f", "1e3", "-d", "1h30m")
	run("-n=0x10", "-n")
	run("-n=0b101")
	run("-n=1_000")
	run("-n", "-3")
	run("-s", "-n")
	run("-", "-n=3")
	run("--", "-n=3")
	run("x", "-n=3")
	run("-n=3", "--", "-n=4", "--")
	run("-h")
	run("-help")
	run("--help")
	run("-zz")
	run("-n=3", "-zz=4")
	run("---n")
	run("-=3")
	run("--=3")
	run("-n")
	run("-s")
	run("-n=abc")
	run("-n=99999999999999999999")
	run("-f=x")
	run("-f=1e999")
	run("-d=5")
	run("-d=5x")
	run("-b=maybe")
	run("-b=yes", "-n=1")

	fmt.Println("--- Visit, Lookup, Set")
	fs, _, _, _, _, _ := newSet()
	fs.Parse([]string{"-s", "x", "-n=2"})
	fs.Visit(func(f *flag.Flag) { fmt.Printf("visit %s=%s (default %s)\n", f.Name, f.Value, f.DefValue) })
	var names []string
	fs.VisitAll(func(f *flag.Flag) { names = append(names, f.Name+":"+f.Usage) })
	fmt.Println(strings.Join(names, " | "))
	fmt.Println(fs.Lookup("n").Value, fs.Lookup("zz") == nil)
	fmt.Println(fs.Set("n", "9"), fs.Set("n", "q"), fs.Set("nothing", "1"))
	fmt.Println(fs.Lookup("n").Value, fs.NFlag())
	fmt.Println(fs.Arg(0) == "", fs.Arg(-1) == "", fs.Arg(5) == "")
	fs.Parse([]string{"a", "b", "c"})
	fmt.Println(fs.Arg(0), fs.Arg(2), fs.Arg(3) == "", fs.NArg(), fs.Args())

	fmt.Println("--- usage")
	fs, _, _, _, _, _ = newSet()
	fs.PrintDefaults()
	fs.Usage()
	fs.SetOutput(nil)
	fmt.Println(fs.Output() == os.Stderr, fs.Name(), fs.ErrorHandling() == flag.ContinueOnError)
	fmt.Println(flag.ErrHelp)

	fmt.Println("--- Init, a zero FlagSet, reuse")
	var z flag.FlagSet
	z.SetOutput(os.Stdout)
	z.Int("k", 1, "k flag")
	fmt.Println(z.Parse([]string{"-k", "2", "-j"}))
	z.Init("zero", flag.ContinueOnError)
	z.SetOutput(os.Stdout)
	fmt.Println(z.Parse([]string{"-j"}))
	z.Usage = func() { fmt.Println("my usage") } // (a closure that used z would be a reference cycle)
	fmt.Println(z.Parse([]string{"-j"}))
	fmt.Println(z.Parse([]string{"-k", "3"}), z.NFlag())
}

// Output:
// [] -> err=<nil> n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// ["-n" "5"] -> err=<nil> n=5 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// ["--n=7" "-s="] -> err=<nil> n=7 s="" b=false f=0.5 d=1m30s args=[] narg=0 nflag=2 parsed=true
// ["-s" "a b" "-b" "x"] -> err=<nil> n=1 s="a b" b=true f=0.5 d=1m30s args=["x"] narg=1 nflag=2 parsed=true
// ["-b=false" "-b=1" "--b=T"] -> err=<nil> n=1 s="dflt" b=true f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// ["-b" "false"] -> err=<nil> n=1 s="dflt" b=true f=0.5 d=1m30s args=["false"] narg=1 nflag=1 parsed=true
// ["-b" "true" "x"] -> err=<nil> n=1 s="dflt" b=true f=0.5 d=1m30s args=["true" "x"] narg=2 nflag=1 parsed=true
// ["-b=true" "true"] -> err=<nil> n=1 s="dflt" b=true f=0.5 d=1m30s args=["true"] narg=1 nflag=1 parsed=true
// ["-f" "1e3" "-d" "1h30m"] -> err=<nil> n=1 s="dflt" b=false f=1000 d=1h30m0s args=[] narg=0 nflag=2 parsed=true
// flag needs an argument: -n
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-n=0x10" "-n"] -> err=flag needs an argument: -n n=16 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// ["-n=0b101"] -> err=<nil> n=5 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// ["-n=1_000"] -> err=<nil> n=1000 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// ["-n" "-3"] -> err=<nil> n=-3 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// ["-s" "-n"] -> err=<nil> n=1 s="-n" b=false f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// ["-" "-n=3"] -> err=<nil> n=1 s="dflt" b=false f=0.5 d=1m30s args=["-" "-n=3"] narg=2 nflag=0 parsed=true
// ["--" "-n=3"] -> err=<nil> n=1 s="dflt" b=false f=0.5 d=1m30s args=["-n=3"] narg=1 nflag=0 parsed=true
// ["x" "-n=3"] -> err=<nil> n=1 s="dflt" b=false f=0.5 d=1m30s args=["x" "-n=3"] narg=2 nflag=0 parsed=true
// ["-n=3" "--" "-n=4" "--"] -> err=<nil> n=3 s="dflt" b=false f=0.5 d=1m30s args=["-n=4" "--"] narg=2 nflag=1 parsed=true
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-h"] -> err=flag: help requested n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// (help)
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-help"] -> err=flag: help requested n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// (help)
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["--help"] -> err=flag: help requested n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// (help)
// flag provided but not defined: -zz
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-zz"] -> err=flag provided but not defined: -zz n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// flag provided but not defined: -zz
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-n=3" "-zz=4"] -> err=flag provided but not defined: -zz n=3 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=1 parsed=true
// bad flag syntax: ---n
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["---n"] -> err=bad flag syntax: ---n n=1 s="dflt" b=false f=0.5 d=1m30s args=["---n"] narg=1 nflag=0 parsed=true
// bad flag syntax: -=3
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-=3"] -> err=bad flag syntax: -=3 n=1 s="dflt" b=false f=0.5 d=1m30s args=["-=3"] narg=1 nflag=0 parsed=true
// bad flag syntax: --=3
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["--=3"] -> err=bad flag syntax: --=3 n=1 s="dflt" b=false f=0.5 d=1m30s args=["--=3"] narg=1 nflag=0 parsed=true
// flag needs an argument: -n
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-n"] -> err=flag needs an argument: -n n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// flag needs an argument: -s
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-s"] -> err=flag needs an argument: -s n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// invalid value "abc" for flag -n: parse error
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-n=abc"] -> err=invalid value "abc" for flag -n: parse error n=0 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// invalid value "99999999999999999999" for flag -n: value out of range
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-n=99999999999999999999"] -> err=invalid value "99999999999999999999" for flag -n: value out of range n=9223372036854775807 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// invalid value "x" for flag -f: parse error
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-f=x"] -> err=invalid value "x" for flag -f: parse error n=1 s="dflt" b=false f=0 d=1m30s args=[] narg=0 nflag=0 parsed=true
// invalid value "1e999" for flag -f: value out of range
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-f=1e999"] -> err=invalid value "1e999" for flag -f: value out of range n=1 s="dflt" b=false f=+Inf d=1m30s args=[] narg=0 nflag=0 parsed=true
// invalid value "5" for flag -d: parse error
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-d=5"] -> err=invalid value "5" for flag -d: parse error n=1 s="dflt" b=false f=0.5 d=0s args=[] narg=0 nflag=0 parsed=true
// invalid value "5x" for flag -d: parse error
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-d=5x"] -> err=invalid value "5x" for flag -d: parse error n=1 s="dflt" b=false f=0.5 d=0s args=[] narg=0 nflag=0 parsed=true
// invalid boolean value "maybe" for -b: parse error
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-b=maybe"] -> err=invalid boolean value "maybe" for -b: parse error n=1 s="dflt" b=false f=0.5 d=1m30s args=[] narg=0 nflag=0 parsed=true
// invalid boolean value "yes" for -b: parse error
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// ["-b=yes" "-n=1"] -> err=invalid boolean value "yes" for -b: parse error n=1 s="dflt" b=false f=0.5 d=1m30s args=["-n=1"] narg=1 nflag=0 parsed=true
// --- Visit, Lookup, Set
// visit n=2 (default 1)
// visit s=x (default dflt)
// b:a bool | d:a duration | f:a float | n:the `count` | s:a string
// 2 true
// <nil> parse error no such flag -nothing
// 0 2
// true true true
// a c true 3 [a b c]
// --- usage
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// Usage of tool:
//   -b	a bool
//   -d duration
//     	a duration (default 1m30s)
//   -f float
//     	a float (default 0.5)
//   -n count
//     	the count (default 1)
//   -s string
//     	a string (default "dflt")
// true tool true
// flag: help requested
// --- Init, a zero FlagSet, reuse
// flag provided but not defined: -j
// Usage:
//   -k int
//     	k flag (default 1)
// flag provided but not defined: -j
// flag provided but not defined: -j
// Usage of zero:
//   -k int
//     	k flag (default 1)
// flag provided but not defined: -j
// flag provided but not defined: -j
// my usage
// flag provided but not defined: -j
// <nil> 1
