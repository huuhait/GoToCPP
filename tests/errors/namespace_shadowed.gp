// phase: 01

// A local variable hides a namespace of the same name, as Go scoping says:
// std.vector is then a field or method of the int std.
package main

import "<vector>" // ERROR "header imports are not supported yet \(Phase 04\)"

func main() {
	{
		std := 3
		_ = std.vector // ERROR "std.vector undefined \(type int has no field or method vector\)"
	}
	_ = std.vector[int]{} // ERROR "composite types are not supported yet \(Phase 03\)"
}
