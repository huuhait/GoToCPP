// phase: 01

// headers/main.h would be named main, which this package declares. Using it
// by that name is go/types' error for func main, with the hint.
package main

import "./headers/main.h"

func main() {
	println(main.mn()) // ERROR "main.mn undefined \(type func\(\) has no field or method mn\); \x22./headers/main.h\x22 has no package name, because main is declared by the package: give it a name: import cmain \x22./headers/main.h\x22"
}
