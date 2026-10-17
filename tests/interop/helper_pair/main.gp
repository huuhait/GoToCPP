// phase: 04

// A helper header with a .cpp file next to it, as a package (step 4.16, README §2.10): this
// folder is one package, run as "gocpp run .", so gocpp compiles ledger.cpp with the C++ compiler
// and links it. ledger.hpp is "package ledger" and the C++ namespace ledger, and includes
// "inc/limits.h" from the package folder.
package main

import (
	"fmt"

	"./ledger.hpp"
)

func main() {
	var b ledger.book
	for i, name := range []string{"rent", "food", "fun", "extra"} {
		ok := b.add(name, 1050*(i+1))
		fmt.Println(name, bool(ok))
	}
	fmt.Println(b.size(), b.total(), ledger.ledger_total(b))
	fmt.Println(string(b.report()))
	fmt.Println(string(ledger.dollars(7)), ledger.calls(), ledger.LEDGER_MAX_ENTRIES)
}

// Output:
// rent true
// food true
// fun true
// extra false
// 3 6300 6300
// rent=10.50;food=21.00;fun=31.50;
// 0.07 4 3
