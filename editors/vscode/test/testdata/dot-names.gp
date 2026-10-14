// SYNTAX TEST "source.gocpp" "a Go keyword or predeclared name right after a dot is a name"

// Lines that start with "//" and then "^" or "<-" are assertions about the
// line above them (vscode-tmgrammar-test). This file is indented with spaces,
// so that the "^" marks line up with the code.

package main

func keywords() {
    ages := std.map[std.string, int]{}
//          ^^^ entity.name.type.go
//              ^^^ entity.name.type.go
//              ^^^ - keyword.map.go
//                      ^^^^^^ entity.name.type.go
//                      ^^^^^^ - storage.type.string.go
//                              ^^^ storage.type.numeric.go
    kind := node.type()
//               ^^^^ entity.name.function.support.go
//               ^^^^ - keyword.type.go
    n := posix.select(fd+1, &rd, nil, nil, nil)
//             ^^^^^^ entity.name.function.support.go
//             ^^^^^^ - keyword.control.go
    r := x.range
//         ^^^^^ variable.other.go
//         ^^^^^ - keyword.control.go
    d := opts.default
//            ^^^^^^^ variable.other.go
//            ^^^^^^^ - keyword.control.go
    f := lib.func(1)
//           ^^^^ entity.name.function.support.go
//           ^^^^ - keyword.function.go
    lib.go(task)
//      ^^ entity.name.function.support.go
//      ^^ - keyword.control.go
    v := t.var + t.const + t.chan
//         ^^^ variable.other.go
//         ^^^ - keyword.var.go
//                 ^^^^^ variable.other.go
//                 ^^^^^ - keyword.const.go
//                           ^^^^ variable.other.go
//                           ^^^^ - keyword.channel.go
    var m std.map[int, int]
//  ^^^ keyword.var.go
//            ^^^ entity.name.type.go
//            ^^^ - keyword.map.go
}

func predeclared() {
    s := std.string("hello")
//           ^^^^^^ entity.name.function.support.go
//           ^^^^^^ - storage.type.string.go
    if at := s.find("x"); at != std.string.npos {
//                                  ^^^^^^ variable.other.go
//                                  ^^^^^^ - storage.type.string.go
    }
    name := p.filename().string()
//                       ^^^^^^ entity.name.function.support.go
//                       ^^^^^^ - storage.type.string.go
    s.append(", world")
//    ^^^^^^ entity.name.function.support.go
//    ^^^^^^ - entity.name.function.support.builtin.go
    m := std.max(a, b)
//           ^^^ entity.name.function.support.go
//           ^^^ - entity.name.function.support.builtin.go
    v.clear()
//    ^^^^^ entity.name.function.support.go
//    ^^^^^ - entity.name.function.support.builtin.go
    n := x.len()
//         ^^^ - entity.name.function.support.builtin.go
    ok := opts.true
//             ^^^^ - constant.language.boolean.go
    var c C.int = 7
//        ^ entity.name.type.go
//          ^^^ entity.name.type.go
//          ^^^ - storage.type.numeric.go
}

// Go's own keywords and predeclared names stay as they are.
func goNames(args ...string) {
//                ^^^ keyword.operator.ellipsis.go
//                   ^^^^^^ storage.type.string.go
    m := map[string]int{}
//       ^^^ keyword.map.go
//           ^^^^^^ storage.type.string.go
//                  ^^^ storage.type.numeric.go
    xs = append(xs, len(ys), max(a, b))
//       ^^^^^^ entity.name.function.support.builtin.go
//                  ^^^ entity.name.function.support.builtin.go
//                           ^^^ entity.name.function.support.builtin.go
    t := string(b)
//       ^^^^^^ storage.type.string.go
    p := new(T)
//       ^^^ entity.name.function.support.builtin.go
    switch v := x.(type) {
//  ^^^^^^ keyword.control.go
//                 ^^^^ keyword.type.go
    case int, string:
//       ^^^ storage.type.numeric.go
//            ^^^^^^ storage.type.string.go
    }
    y := x.(string)
//          ^^^^^^ storage.type.string.go
    for range n {
//      ^^^^^ keyword.control.go
    }
    // A keyword on the line after a "." stays a keyword (same line only).
    fmt.
    if x {
//  ^^ keyword.control.go
    }
}

func variadic(fs ...func(int), ms ...map[string]int, cs ...chan int) {}
//                  ^^^^ keyword.function.go
//                                   ^^^ keyword.map.go
//                                                         ^^^^ keyword.channel.go
