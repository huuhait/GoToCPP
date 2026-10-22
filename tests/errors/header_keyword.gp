// phase: 01

// <sys/select.h> would be named select, a Go keyword, so the parser stops at
// select.fd_set before gocpp can look at the import. The error asks for a
// name.
package main

import "<sys/select.h>"

var set select.fd_set // ERROR "<sys/select.h> has no package name, because select is a Go keyword: give it a name: import cselect \x22<sys/select.h>\x22"

func main() {
	_ = set
}
