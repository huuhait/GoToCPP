// phase: 01

// A //gocpp: directive after the package clause is an error.
package main

//gocpp:link m // ERROR "//gocpp: directives must come before the package clause"

func main() {}
