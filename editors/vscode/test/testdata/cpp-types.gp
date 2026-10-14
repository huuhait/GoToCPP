// SYNTAX TEST "source.gocpp" "C++ forms: constant type arguments, dotted types, C types and helpers"

// Lines that start with "//" and then "^" or "<-" are assertions about the
// line above them (vscode-tmgrammar-test). This file is indented with spaces,
// so that the "^" marks line up with the code.

package main

type Timer struct {
    Every std.chrono.seconds       // std::chrono::seconds
//  ^^^^^ variable.other.property.go
//        ^^^ entity.name.type.go
//            ^^^^^^ entity.name.type.go
//                   ^^^^^^^ entity.name.type.go
//                                 ^^ comment.line.double-slash.go
    Pos   std.vector[int].iterator // std::vector<int64_t>::iterator
//  ^^^ variable.other.property.go
//            ^^^^^^ entity.name.type.go
//                   ^^^ storage.type.numeric.go
//                        ^^^^^^^^ entity.name.type.go
    Buf   std.array[C.int, 4]
//            ^^^^^ entity.name.type.go
//                  ^ entity.name.type.go
//                    ^^^ entity.name.type.go
//                         ^ constant.numeric.decimal.go
    Win   *SDL.SDL_Window
//        ^ keyword.operator.address.go
//             ^^^^^^^^^^ entity.name.type.go
}

func base(p std.filesystem.path) std.filesystem.path {
//        ^ variable.parameter.go
//                         ^^^^ entity.name.type.go
//                                              ^^^^ entity.name.type.go
    return p.filename()
}

func get(it std.vector[int].iterator, n C.size_t) std.array[int, 5] {
//       ^^ variable.parameter.go
//                          ^^^^^^^^ entity.name.type.go
//                                    ^ variable.parameter.go
//                                        ^^^^^^ entity.name.type.go
//                                                    ^^^^^ entity.name.type.go
//                                                               ^ constant.numeric.decimal.go
    var buf std.array[byte, 256]
//      ^^^ variable.other.assignment.go
//              ^^^^^ entity.name.type.go
//                    ^^^^ storage.type.byte.go
//                          ^^^ constant.numeric.decimal.go
    a := std.array[int, 5]{1, 2, 3, 4, 5}
//           ^^^^^ entity.name.type.go
//                      ^ constant.numeric.decimal.go
    bits := std.bitset[64]{}
//              ^^^^^^ entity.name.type.go
//                     ^^ constant.numeric.decimal.go
    var d std.chrono.seconds
//      ^ variable.other.assignment.go
//                   ^^^^^^^ entity.name.type.go
    var ds []std.chrono.seconds
//                      ^^^^^^^ entity.name.type.go
    v := std.vector[int](10, 0)
//           ^^^^^^ entity.name.function.support.go
//                  ^^^ storage.type.numeric.go
    var c C.int = 7
//        ^ entity.name.type.go
//          ^^^ entity.name.type.go
    x := C.int(42)
//       ^ variable.other.go
//         ^^^ entity.name.function.support.go
    var p **C.char
//        ^^ keyword.operator.address.go
//            ^^^^ entity.name.type.go
    cstdio.printf("%s %d\n", C.cstr(name), C.int(len(name)))
//         ^^^^^^ entity.name.function.support.go
//                             ^^^^ entity.name.function.support.go
//                                  ^^^^ variable.other.go
//                                               ^^^ entity.name.function.support.builtin.go
    s := C.GoString(sqlite3.sqlite3_libversion())
//         ^^^^^^^^ entity.name.function.support.go
//                          ^^^^^^^^^^^^^^^^^^ entity.name.function.support.go
    rl.ClearBackground(rl.RAYWHITE)
//     ^^^^^^^^^^^^^^^ entity.name.function.support.go
//                        ^^^^^^^^ variable.other.go
    return a
}

// After a function whose result is a C++ type with constant arguments, the
// grammar is back at the top level.
var after = g(arg)
//  ^^^^^ variable.other.assignment.go
//            ^^^ variable.other.go
//            ^^^ - entity.name.type.go
