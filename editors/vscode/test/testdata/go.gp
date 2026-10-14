// SYNTAX TEST "source.gocpp" "plain Go: what the gocpp changes must not break"

// Lines that start with "//" and then "^" or "<-" are assertions about the
// line above them (vscode-tmgrammar-test). This file is indented with spaces,
// so that the "^" marks line up with the code.

package main
// <------- keyword.package.go
//      ^^^^ entity.name.type.package.go

import "fmt"
// <------ keyword.control.import.go
//      ^^^ string.quoted.double.go entity.name.import.go

func main() {
// <---- keyword.function.go
//   ^^^^ entity.name.function.go
    if x != y && !ok {
//       ^^ keyword.operator.comparison.go
//               ^ keyword.operator.logical.go
    }
    switch v := x.(type) {
//                 ^^^^ keyword.type.go
    case int:
//       ^^^ storage.type.numeric.go
    }
    fmt.Println("#", '#', `#`) // # here is not a macro
//      ^^^^^^^ entity.name.function.support.go
//               ^ string.quoted.double.go
//                    ^ string.quoted.rune.go
//                         ^ string.quoted.raw.go
//                                ^ comment.line.double-slash.go
}

// A func declaration ends at the "{" of its body, also after a result type
// that has a func type or a type with more than one argument. Before the fix
// the lines after such a function were still part of its declaration, and
// "arg" below was highlighted as a type.
func counter(step int) func() int {
//   ^^^^^^^ entity.name.function.go
//           ^^^^ variable.parameter.go
//                     ^^^^ keyword.function.go
    return nil
}

var a1 = g(arg)
//         ^^^ variable.other.go
//         ^^^ - entity.name.type.go

func trace(name string) func() {
//   ^^^^^ entity.name.function.go
    return nil
}

var a2 = g(arg)
//         ^^^ variable.other.go
//         ^^^ - entity.name.type.go

func (s *Stack[T]) All() iter.Seq2[int, T] {
//                 ^^^ entity.name.function.go
//                            ^^^^ entity.name.type.go
    return nil
}

var a3 = g(arg)
//         ^^^ variable.other.go
//         ^^^ - entity.name.type.go

func pairs[K cmp.Ordered, V any](m map[K]V) []Pair[K, V] {
//   ^^^^^ entity.name.function.go
//                                            ^^^^ entity.name.type.go
    return nil
}

var a4 = g(arg)
//         ^^^ variable.other.go
//         ^^^ - entity.name.type.go

func shape() interface{ Area() int } {
//   ^^^^^ entity.name.function.go
//           ^^^^^^^^^ keyword.interface.go
    return nil
}

var a5 = g(arg)
//         ^^^ variable.other.go
//         ^^^ - entity.name.type.go

func plain(a, b int) (int, error) {
//   ^^^^^ entity.name.function.go
//         ^ variable.parameter.go
//                         ^^^^^ storage.type.error.go
    return a + b, nil
}

var a6 = g(arg)
//         ^^^ variable.other.go
//         ^^^ - entity.name.type.go
