// phase: 05

// C++ exceptions of every kind are recoverable panics: std::exception subclasses, throws of a
// C string, a std::string, an int and a plain struct, a nested exception. The recovered value is
// an error and prints the same text as an unrecovered one. The frames that the exception unwinds
// run their C++ destructors, also through a Go interface method, a func value, a method value,
// a std::function that calls back into Go, and a template.
package main

import (
	"errors"
	"fmt"

	"./cxxrec.hpp"
	"<string>"
	"<vector>"
)

type Thrower interface{ Throw(n int) }

type appThrower struct{ prefix string }

func (a appThrower) Throw(n int) { cxxrec.throw_app(std.string(fmt.Sprint(a.prefix, n))) }

type intThrower struct{}

func (*intThrower) Throw(n int) { cxxrec.throw_int(C.int(n)) }

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// catch runs f and prints what it panics with. The C++ names of the types that the standard
// library chooses (a std::string, a nested exception) are not the same for every library, so
// a name that starts with "~" prints only the parts that must be there.
func catch(name string, f func()) {
	defer func() {
		r := recover()
		err, isErr := r.(error)
		text := fmt.Sprint(r)
		if name[0] == '~' {
			name = name[1:]
			text = fmt.Sprint(contains(text, "C++ exception"), contains(text, name))
		}
		fmt.Printf("%-12s %v | error=%v | %T\n", name, text, isErr, r)
		if isErr {
			var again error = err
			fmt.Printf("  Error() matches Print: %v\n", again.Error() == fmt.Sprint(r))
		}
	}()
	f()
	fmt.Println(name, "no exception")
}

func viaInterface(t Thrower, n int) (res string) {
	defer func() {
		if r := recover(); r != nil {
			res = fmt.Sprint("recovered ", r)
		}
	}()
	t.Throw(n)
	return "none"
}

func main() {
	catch("app", func() { cxxrec.throw_app(std.string("custom what")) })
	catch("logic", func() { cxxrec.throw_logic(std.string("bad logic")) })
	catch("~char", func() { cxxrec.throw_cstr() })
	catch("~string", func() { cxxrec.throw_string() })
	catch("int", func() { cxxrec.throw_int(7) })
	catch("~Plain", func() { cxxrec.throw_plain(3) })
	catch("~outer", func() { cxxrec.throw_nested() })

	fmt.Println(viaInterface(appThrower{"via-"}, 1))
	fmt.Println(viaInterface(&intThrower{}, 2))
	var th Thrower = appThrower{"m-"}
	f := th.Throw
	g := (*intThrower).Throw
	catch("method value", func() { f(3) })
	catch("method expr", func() { g(&intThrower{}, 4) })

	before := int(cxxrec.destroyed)
	catch("raii", func() { cxxrec.guarded_throw(3) })
	fmt.Println("destructors run:", int(cxxrec.destroyed)-before)

	before = int(cxxrec.destroyed)
	v := std.vector[C.int]{1, 2, 3}
	catch("go panic", func() {
		cxxrec.for_each_sum(v, func(x C.int) {
			if x == 2 {
				panic(fmt.Errorf("go panic at %d", x))
			}
		})
	})
	fmt.Println("destructors run:", int(cxxrec.destroyed)-before)

	before = int(cxxrec.destroyed)
	catch("cxx in cb", func() {
		cxxrec.for_each_sum(v, func(x C.int) {
			if x == 3 {
				cxxrec.throw_logic(std.string("from a callback"))
			}
		})
	})
	fmt.Println("destructors run:", int(cxxrec.destroyed)-before)

	before = int(cxxrec.destroyed)
	catch("template", func() {
		cxxrec.apply_twice(func(x C.int) C.int {
			if x > 2 {
				panic("too big")
			}
			return x * 3
		}, 1)
	})
	fmt.Println("destructors run:", int(cxxrec.destroyed)-before)

	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("wrapped: %w", r.(error))
			}
		}()
		cxxrec.throw_app(std.string("inner app"))
		return nil
	}()
	fmt.Println(err, errors.Unwrap(err) != nil)
	fmt.Println("done")
}

// Output:
// app          C++ exception cxxrec::AppError: custom what | error=true | runtime.CppException
//   Error() matches Print: true
// logic        C++ exception std::logic_error: bad logic | error=true | runtime.CppException
//   Error() matches Print: true
// char         true true | error=true | runtime.CppException
//   Error() matches Print: true
// string       true true | error=true | runtime.CppException
//   Error() matches Print: true
// int          C++ exception of type int | error=true | runtime.CppException
//   Error() matches Print: true
// Plain        true true | error=true | runtime.CppException
//   Error() matches Print: true
// outer        true true | error=true | runtime.CppException
//   Error() matches Print: true
// recovered C++ exception cxxrec::AppError: via-1
// recovered C++ exception of type int
// method value C++ exception cxxrec::AppError: m-3 | error=true | runtime.CppException
//   Error() matches Print: true
// method expr  C++ exception of type int | error=true | runtime.CppException
//   Error() matches Print: true
// raii         C++ exception cxxrec::AppError: deep | error=true | runtime.CppException
//   Error() matches Print: true
// destructors run: 4
// go panic     go panic at 2 | error=true | *errors.errorString
//   Error() matches Print: true
// destructors run: 1
// cxx in cb    C++ exception std::logic_error: from a callback | error=true | runtime.CppException
//   Error() matches Print: true
// destructors run: 1
// template     too big | error=false | string
// destructors run: 1
// wrapped: C++ exception cxxrec::AppError: inner app true
// done
