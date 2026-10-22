// phase: 04

// main.h would be named main, which the package declares: go/types says main.mn is undefined,
// and gocpp adds how to name the header.
package main

import "./main.h"

func main() {
	println(main.mn()) // ERROR "main.mn undefined \(type func\(\) has no field or method mn\); \x22./main.h\x22 has no package name, because main is declared by the package: give it a name: import cmain \x22./main.h\x22"
}
