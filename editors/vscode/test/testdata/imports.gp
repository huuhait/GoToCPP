// SYNTAX TEST "source.gocpp" "imports: gocpp packages, C/C++ headers and C++ namespaces"

// Lines that start with "//" and then "^" or "<-" are assertions about the
// line above them (vscode-tmgrammar-test). This file is indented with spaces,
// so that the "^" marks line up with the code.

package main

import (
    "fmt"
//   ^^^ string.quoted.double.go entity.name.import.go
//   ^^^ - meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "example.com/multi-pkg/geom"
//   ^^^^^^^^^^^^^^^^^^^^^^^^^^ string.quoted.double.go entity.name.import.go

    "<raylib.h>" // a C header: package raylib
//  ^ string.quoted.double.go punctuation.definition.string.begin.go
//   ^ string.quoted.double.go meta.preprocessor.include.gocpp punctuation.definition.header.begin.gocpp
//    ^^^^^^^^ string.quoted.double.go meta.preprocessor.include.gocpp entity.name.import.header.gocpp
//    ^^^^^^^^ - entity.name.import.go
//            ^ string.quoted.double.go meta.preprocessor.include.gocpp punctuation.definition.header.end.gocpp
//             ^ string.quoted.double.go punctuation.definition.string.end.go
//               ^^ comment.line.double-slash.go
    "<SDL2/SDL.h>"
//    ^^^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "<vector>"
//    ^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "./physics.h"
//  ^ string.quoted.double.go punctuation.definition.string.begin.go
//   ^^^^^^^^^^^ string.quoted.double.go meta.preprocessor.include.gocpp entity.name.import.header.gocpp
//   ^^^^^^^^^^^ - entity.name.import.go
//              ^ string.quoted.double.go punctuation.definition.string.end.go
    "native/x.hpp"
//   ^^^^^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "./my-lib.h"
//   ^^^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "lib/a.hh"
//   ^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "lib/b.hxx"
//   ^^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "lib/c.h++"
//   ^^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "lib/d.inl"
//   ^^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
    "lib/hpp"
//   ^^^^^^^ entity.name.import.go
//   ^^^^^^^ - entity.name.import.header.gocpp

    rl "<raylib.h>"
//  ^^ variable.other.import.go
//       ^^^^^^^^ entity.name.import.header.gocpp
    _ "<GL/gl.h>"
//  ^ variable.other.import.go
//      ^^^^^^^ entity.name.import.header.gocpp
    cstring "<string.h>"
//  ^^^^^^^ variable.other.import.go
//            ^^^^^^^^ entity.name.import.header.gocpp
    phys "./physics.h"
//  ^^^^ variable.other.import.go
//        ^^^^^^^^^^^ entity.name.import.header.gocpp

    fs "::std::filesystem"
//  ^^ variable.other.import.go
//     ^ string.quoted.double.go punctuation.definition.string.begin.go
//      ^^ string.quoted.double.go entity.name.namespace.gocpp punctuation.separator.namespace.gocpp
//        ^^^ string.quoted.double.go entity.name.namespace.gocpp
//           ^^ string.quoted.double.go entity.name.namespace.gocpp punctuation.separator.namespace.gocpp
//             ^^^^^^^^^^ string.quoted.double.go entity.name.namespace.gocpp
//             ^^^^^^^^^^ - entity.name.import.go
//                       ^ string.quoted.double.go punctuation.definition.string.end.go
    cfmt "::fmt"
//  ^^^^ variable.other.import.go
//        ^^ punctuation.separator.namespace.gocpp
//          ^^^ entity.name.namespace.gocpp
)

import "<cstdio>"
// <------ keyword.control.import.go
//       ^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
import posix "<sys/select.h>"
//     ^^^^^ variable.other.import.go
//             ^^^^^^^^^^^^ meta.preprocessor.include.gocpp entity.name.import.header.gocpp
import fs2 "::std::filesystem"
//          ^^ punctuation.separator.namespace.gocpp
//            ^^^ entity.name.namespace.gocpp
import "strings"
//      ^^^^^^^ string.quoted.double.go entity.name.import.go

// A string with "<" that is not an import stays a string.
var s = "<raylib.h>"
//       ^^^^^^^^^^ string.quoted.double.go
//       ^^^^^^^^^^ - meta.preprocessor.include.gocpp entity.name.import.header.gocpp
