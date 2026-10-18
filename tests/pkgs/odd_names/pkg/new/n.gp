package new

import "fmt"

// package names that are C++ keywords or Go builtins

type class struct{ int_ int }

var namespace = 3

func Class() class { return class{namespace} }

func (c class) Delete() { fmt.Println("delete", c.int_) }

const template = "tmpl"

func Template() string { return template }
