// SYNTAX TEST "source.gocpp" "macros: macro declarations, #Name(...) uses, built-in macros, lazy"

// Lines that start with "//" and then "^" or "<-" are assertions about the
// line above them (vscode-tmgrammar-test). This file is indented with spaces,
// so that the "^" marks line up with the code.

package util

// Declarations, at package level.
macro Assert(cond bool, msg lazy string) {
// <----- keyword.macro.gocpp
//    ^^^^^^ entity.name.function.macro.gocpp
//           ^^^^ variable.parameter.go
//                ^^^^ storage.type.boolean.go
//                      ^^^ variable.parameter.go
//                          ^^^^ storage.modifier.lazy.gocpp
//                               ^^^^^^ storage.type.string.go
    if !cond {
        panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
//                                         ^ entity.name.function.macro.builtin.gocpp punctuation.definition.macro.gocpp
//                                          ^^^^ entity.name.function.macro.builtin.gocpp
//                                                  ^^^^^ entity.name.function.macro.builtin.gocpp
//                                                           ^^^^^ entity.name.function.macro.builtin.gocpp
//                                                                 ^^^^ variable.other.go
    }
}

macro Dbg[T any](v T) T {
// <----- keyword.macro.gocpp
//    ^^^ entity.name.function.macro.gocpp
//        ^ variable.parameter.go
//          ^^^ entity.name.type.any.go
//               ^ variable.parameter.go
//                 ^ entity.name.type.go
//                    ^ entity.name.type.go
    fmt.Fprintf(os.Stderr, "%s = %v\n", #text(v), v)
//                                      ^^^^^ entity.name.function.macro.builtin.gocpp
    return v
}

macro Trace() func() {
// <----- keyword.macro.gocpp
//    ^^^^^ entity.name.function.macro.gocpp
//            ^^^^ keyword.function.go
    fmt.Println("enter", #func())
//                       ^ entity.name.function.macro.builtin.gocpp punctuation.definition.macro.gocpp
//                        ^^^^ entity.name.function.macro.builtin.gocpp
//                        ^^^^ - keyword.function.go
    return func() { fmt.Println("exit ", #func()) }
//         ^^^^ keyword.function.go
//                                       ^^^^^ entity.name.function.macro.builtin.gocpp
}

macro Locked(mu *sync.Mutex) func() {
//    ^^^^^^ entity.name.function.macro.gocpp
//           ^^ variable.parameter.go
//                    ^^^^^ entity.name.type.go
    mu.Lock()
    return mu.Unlock
}

macro Recover() {
//    ^^^^^^^ entity.name.function.macro.gocpp
    defer func() {
        if r := recover(); r != nil {
            log.Println("recovered in", #func()+":", r)
        }
    }()
    #body()
//  ^ entity.name.function.macro.builtin.gocpp punctuation.definition.macro.gocpp
//   ^^^^ entity.name.function.macro.builtin.gocpp
}

macro Once(o *sync.Once) {
    o.Do(func() { #body() })
//                ^^^^^ entity.name.function.macro.builtin.gocpp
}

macro Logo() string {
//    ^^^^ entity.name.function.macro.gocpp
//           ^^^^^^ storage.type.string.go
    return #embed("logo.png")
//         ^^^^^^ entity.name.function.macro.builtin.gocpp
//                 ^^^^^^^^ string.quoted.double.go
}

macro Mixed(a, b lazy int, msg lazy string, n int) {}
//          ^ variable.parameter.go
//             ^ variable.parameter.go
//               ^^^^ storage.modifier.lazy.gocpp
//                    ^^^ storage.type.numeric.go
//                         ^^^ variable.parameter.go
//                             ^^^^ storage.modifier.lazy.gocpp
//                                          ^ variable.parameter.go

macro Lines(
    cond bool,
//  ^^^^ variable.parameter.go
    msg lazy string,
//  ^^^ variable.parameter.go
//      ^^^^ storage.modifier.lazy.gocpp
//           ^^^^^^ storage.type.string.go
) {
}

// "lazy" can't be the name of a macro parameter; the compiler says so, and
// the grammar doesn't call it a modifier.
macro Bad(lazy string) {}
//        ^^^^ variable.parameter.go
//        ^^^^ - storage.modifier.lazy.gocpp

// Uses on top of a func or a type: start style, wrap style and #Derive.
// A doc comment goes above the # lines.
#Trace()
// <- entity.name.function.macro.gocpp punctuation.definition.macro.gocpp
// <~----- entity.name.function.macro.gocpp
#util.VM()
// <- entity.name.function.macro.gocpp punctuation.definition.macro.gocpp
// <~---- variable.other.go
// <~~~~~- punctuation.other.period.go
// <~~~~~~-- entity.name.function.macro.gocpp
func secret(key string) (int, error) {
// <---- keyword.function.go
//   ^^^^^^ entity.name.function.go
    #Assert(key != "", "empty key")
//  ^ entity.name.function.macro.gocpp punctuation.definition.macro.gocpp
//   ^^^^^^ entity.name.function.macro.gocpp
//              ^^ keyword.operator.comparison.go
    n := #Dbg(len(key) * 42)
//       ^^^^ entity.name.function.macro.gocpp
//            ^^^ entity.name.function.support.builtin.go
    port := #util.Dbg(cfg.Port)
//          ^ entity.name.function.macro.gocpp punctuation.definition.macro.gocpp
//           ^^^^ variable.other.go
//                ^^^ entity.name.function.macro.gocpp
//                        ^^^^ variable.other.go
    return n, nil
}

#Locked(&s.mu)
// <- entity.name.function.macro.gocpp punctuation.definition.macro.gocpp
// <~------ entity.name.function.macro.gocpp
//      ^ keyword.operator.address.go
//       ^ variable.other.go
//         ^^ variable.other.go
//         ^^ - entity.name.type.go
func (s *Store) Put(k string, v int) {
//              ^^^ entity.name.function.go
    s.m[k] = v
}

#Derive(String)
// <- entity.name.function.macro.builtin.gocpp punctuation.definition.macro.gocpp
// <~------ entity.name.function.macro.builtin.gocpp
//      ^^^^^^ variable.other.go
type Color int
// <---- keyword.type.go

#Derive(JSON)
// <~------ entity.name.function.macro.builtin.gocpp
type Config struct {
    Port int `json:"port"`
}

// A macro in an index before a call. The Go grammar reads "x[...](" as a
// call with type arguments; a macro there is still a macro.
func indexed() {
    handlers[#Dbg(i)](x)
//           ^ entity.name.function.macro.gocpp punctuation.definition.macro.gocpp
//            ^^^ entity.name.function.macro.gocpp
//            ^^^ - entity.name.type.go
}

// Not macros: # in strings, raw strings, runes and comments.
func notMacros() {
    a := "#32770"
//        ^^^^^^ string.quoted.double.go
//        ^ - entity.name.function.macro.gocpp punctuation.definition.macro.gocpp
    b := `#Dbg(x)`
//        ^^^^^^^ string.quoted.raw.go
//        ^ - entity.name.function.macro.gocpp
    c := '#'
//        ^ string.quoted.rune.go constant.other.rune.go
    d := strings.HasPrefix(line, "#") // #Dbg(x) in a comment
//                                ^ string.quoted.double.go
//                                       ^^^^^^^ comment.line.double-slash.go
//                                       ^^^^^^^ - entity.name.function.macro.gocpp
    /* #body() */
//     ^^^^^^^ comment.block.go
//     ^^^^^^^ - entity.name.function.macro.builtin.gocpp
}

// D18: outside a macro declaration, macro and lazy are ordinary names.
func count() int {
    macro, lazy := 1, 2
//  ^^^^^ variable.other.assignment.go
//  ^^^^^ - keyword.macro.gocpp
//         ^^^^ variable.other.assignment.go
//         ^^^^ - storage.modifier.lazy.gocpp
    return macro + lazy
//         ^^^^^ variable.other.go
//                 ^^^^ variable.other.go
}

func f(lazy int) {}
//     ^^^^ variable.parameter.go
//     ^^^^ - storage.modifier.lazy.gocpp

var macro = 1
//  ^^^^^ variable.other.assignment.go

func macro() {}
//   ^^^^^ entity.name.function.go
//   ^^^^^ - keyword.macro.gocpp

// In the body of a macro, lazy is a name too.
macro Body(x int) {
    lazy := x
//  ^^^^ variable.other.assignment.go
//  ^^^^ - storage.modifier.lazy.gocpp
}

func g() {
    macro(lazy(x))
//  ^^^^^ entity.name.function.support.go
//  ^^^^^ - keyword.macro.gocpp
//        ^^^^ entity.name.function.support.go
}
