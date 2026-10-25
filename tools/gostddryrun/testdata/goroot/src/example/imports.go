package example

import "example/dep"

// Dep uses the dependency so that the import is not blank.
func Dep() int { return dep.Value() }
