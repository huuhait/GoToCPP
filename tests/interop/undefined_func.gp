// phase: 04
// ExitCode: 1
// Stderr: (?m)^\./undefined_func\.gp:11:9: undefined: cstdio\.printff \(the imported headers declare no ::printff\)$

// A C-level name of a header package that the headers don't declare (a typo).
package main

import "<cstdio>"

func main() {
	cstdio.printff("x\n")
}

// Output:
