// phase: 04

package main

import (
	"fmt"

	"./ledger.hpp" // package ledger, and also the C++ namespace ledger
)

func main() {
	fmt.Println(string(ledger.dollars(1050))) // defined in ledger.cpp, built with the package

	acct := ledger.Account(250) // a constructor call
	acct.deposit(75)
	fmt.Println(string(acct.text()), int(acct.cents()))
}

// Output:
// $10.50
// $3.25 325
