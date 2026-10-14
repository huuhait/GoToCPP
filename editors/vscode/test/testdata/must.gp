// SYNTAX TEST "source.gocpp" "must: name!(...) calls and func name!(...) declarations"

// Lines that start with "//" and then "^" or "<-" are assertions about the
// line above them (vscode-tmgrammar-test). This file is indented with spaces,
// so that the "^" marks line up with the code.

package main

// Must functions: the name keeps its function scope, the ! is an operator.
func loadConfig!(path string) Config {
//   ^^^^^^^^^^ entity.name.function.go
//             ^ keyword.operator.must.gocpp
//               ^^^^ variable.parameter.go
//                            ^^^^^^ entity.name.type.go
}

func main!() {
//   ^^^^ entity.name.function.go
//       ^ keyword.operator.must.gocpp
}

func (s *Server) Start!() {
//       ^^^^^^ entity.name.type.go
//               ^^^^^ entity.name.function.go
//                    ^ keyword.operator.must.gocpp
}

func parse![T any](s string) T {
//   ^^^^^ entity.name.function.go
//        ^ keyword.operator.must.gocpp
//          ^ variable.parameter.go
}

func loadConfig2!(path string) (cfg Config, err error) {
//   ^^^^^^^^^^^ entity.name.function.go
//              ^ keyword.operator.must.gocpp
//                              ^^^ variable.parameter.go
//                                              ^^^^^ storage.type.error.go
}

// Must calls.
func calls() {
    data := os.ReadFile!(path)
//             ^^^^^^^^ entity.name.function.support.go
//                     ^ keyword.operator.must.gocpp
//                       ^^^^ variable.other.go
    n := strconv.Atoi!(s) * 2
//               ^^^^ entity.name.function.support.go
//                   ^ keyword.operator.must.gocpp
//                        ^ keyword.operator.arithmetic.go
    a, b := split!(s)
//          ^^^^^ entity.name.function.support.go
//               ^ keyword.operator.must.gocpp
    f.Close!()
//    ^^^^^ entity.name.function.support.go
//         ^ keyword.operator.must.gocpp
    defer f.Close!()
//  ^^^^^ keyword.control.go
//          ^^^^^ entity.name.function.support.go
//               ^ keyword.operator.must.gocpp
    v := parse![int](s)
//       ^^^^^ entity.name.function.support.go
//            ^ keyword.operator.must.gocpp
//              ^^^ storage.type.numeric.go
    w := parse![Config](s)
//            ^ keyword.operator.must.gocpp
//              ^^^^^^ entity.name.type.go
    g(h(), F!(a))
//         ^ entity.name.function.support.go
//          ^ keyword.operator.must.gocpp
    port := loadConfig!(path).Workers
//          ^^^^^^^^^^ entity.name.function.support.go
//                    ^ keyword.operator.must.gocpp
//                            ^^^^^^^ variable.other.go
    switch v := parse!(x).(type) {
//                   ^ keyword.operator.must.gocpp
//                         ^^^^ keyword.type.go
    case int:
    }
    switch {
    case F!(x):
//       ^ entity.name.function.support.go
//        ^ keyword.operator.must.gocpp
    }
}

// Plain Go: != and the prefix "not" stay as they are.
func plain() {
    if x != y && !ok {
//       ^^ keyword.operator.comparison.go
//               ^ keyword.operator.logical.go
//               ^ - keyword.operator.must.gocpp
    }
    ok = x!=(y)
//        ^^ keyword.operator.comparison.go
//        ^^ - keyword.operator.must.gocpp
    if !rl.IsWindowReady() {
//     ^ keyword.operator.logical.go
//         ^^^^^^^^^^^^^ entity.name.function.support.go
    }
    for !done(x) {
//      ^ keyword.operator.logical.go
    }
    // After a Go keyword, ! is "not": if!(a) is if !(a).
    if!(a && b) {
//  ^^ keyword.control.go
//    ^ keyword.operator.logical.go
//    ^ - keyword.operator.must.gocpp
        return!(x)
//      ^^^^^^ keyword.control.go
//            ^ keyword.operator.logical.go
//            ^ - keyword.operator.must.gocpp
    }
    for!(a) {
//  ^^^ keyword.control.go
//     ^ keyword.operator.logical.go
    }
    switch {
    case!(b):
//  ^^^^ keyword.control.go
//      ^ keyword.operator.logical.go
//      ^ - keyword.operator.must.gocpp
    }
    // In strings and comments, nothing changes: f!(x)
//                                               ^^^^^ comment.line.double-slash.go
//                                                ^ - keyword.operator.must.gocpp
    s := "f!(x)"
//         ^ string.quoted.double.go
//         ^ - keyword.operator.must.gocpp
}
