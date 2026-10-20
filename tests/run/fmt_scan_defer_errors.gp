// phase: 09

// ScanState errors must remain Go scanError panics through custom Scanner/defer frames.
package main

import "fmt"

var cleaned int

type deferredSpace struct{}

func (deferredSpace) Scan(state fmt.ScanState, verb rune) error {
	defer func() { cleaned++ }()
	state.SkipSpace()
	return nil
}

type repanicSpace struct{}

func (repanicSpace) Scan(state fmt.ScanState, verb rune) error {
	defer func() {
		cleaned++
		if value := recover(); value != nil {
			fmt.Printf("recovered %T\n", value)
			panic(value)
		}
	}()
	state.SkipSpace()
	return nil
}

type recoverSpace struct{}

func (recoverSpace) Scan(state fmt.ScanState, verb rune) error {
	defer func() {
		cleaned++
		if value := recover(); value != nil {
			fmt.Printf("handled %T\n", value)
		}
	}()
	state.SkipSpace()
	return nil
}

type nestedToken struct{}

func (nestedToken) Scan(state fmt.ScanState, verb rune) error {
	defer func() { cleaned++ }()
	_, err := state.Token(false, func(character rune) bool {
		defer func() { cleaned++ }()
		state.SkipSpace()
		return true
	})
	return err
}

func check(name string, input string, scanner fmt.Scanner) {
	cleaned = 0
	count, err := fmt.Sscanln(input, scanner)
	fmt.Println(name, count, err, cleaned)
}

func main() {
	check("deferred", "\n", deferredSpace{})
	check("repanicked", "\n", repanicSpace{})
	check("handled", "\n", recoverSpace{})
	check("predicate", "x\n", nestedToken{})
}

// Output:
// deferred 0 unexpected newline 1
// recovered fmt.scanError
// repanicked 0 unexpected newline 1
// handled fmt.scanError
// handled 1 <nil> 1
// predicate 0 unexpected newline 2
