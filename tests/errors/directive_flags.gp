// phase: 01
//gocpp:cflags -DOK -fplugin=./evil.so // ERROR "invalid flag in //gocpp:cflags: -fplugin=\./evil\.so"
//gocpp:cflags -I // ERROR "invalid flag in //gocpp:cflags: -I without argument"
//gocpp:ldflags -Wl,-plugin,./evil.so // ERROR "invalid flag in //gocpp:ldflags: -Wl,-plugin,\./evil\.so"
//gocpp:link to_library // ERROR "invalid flag in //gocpp:link: -lto_library"
//gocpp:cflags -m32 // ERROR "invalid flag in //gocpp:cflags: -m32 \(it changes the ABI.*GOCPP_CFLAGS_ALLOW can allow it\)"
//gocpp:ldflags --sysroot=/opt/root // ERROR "invalid flag in //gocpp:ldflags: --sysroot=/opt/root \(a package can't choose the system root or the toolchain: they are choices for the whole program; set them in GOCPP_CFLAGS or GOCPP_LDFLAGS"
//gocpp:cflags -D _GLIBCXX_DEBUG // ERROR "invalid flag in //gocpp:cflags: -D_GLIBCXX_DEBUG \(it changes the ABI"
//gocpp:cflags -DMSG=\"hi\" -Iinclude -std=c++23 -fno-rtti -D_REENTRANT -D_FILE_OFFSET_BITS=64 -D_GLIBCXX_ASSERTIONS

// The flag allow-list (Phase 04 README §2.9, step 4.15): a flag of
// //gocpp:cflags, ldflags or link that can run code at build time, or a
// choice for the whole program (the target, the ABI, the system root), is an
// error at its line, before any compiler or probe runs. The last line has
// only allowed flags, among them a string macro that cgo can't define. The
// gate error of the directives comes once per file.
package main

func main() {}
