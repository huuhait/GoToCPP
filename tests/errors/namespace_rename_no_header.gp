// phase: 01

// A namespace rename in a file without header imports can't name a
// namespace.
package main

import fs "::std::filesystem" // ERROR "cannot rename ::std::filesystem: the file imports no header that could declare it" "C\+\+ namespaces are not supported yet \(Phase 04\)"

func main() {
	_ = fs.path("/")
}
