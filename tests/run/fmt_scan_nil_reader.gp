// phase: 09

// A nil reader panics only on an actual read, not while constructing a scanner.
package main

import "fmt"

type noRead struct{}

func (noRead) Scan(state fmt.ScanState, verb rune) error { return nil }

func check(name string, scan func() (int, error)) {
	defer func() {
		if value := recover(); value != nil {
			fmt.Println(name, "panic", value)
		}
	}()
	count, err := scan()
	fmt.Println(name, count, err)
}

func main() {
	check("empty Fscan", func() (int, error) { return fmt.Fscan(nil) })
	check("empty Fscanf", func() (int, error) { return fmt.Fscanf(nil, "") })
	check("custom no read", func() (int, error) { return fmt.Fscan(nil, noRead{}) })
	check("extra operand", func() (int, error) {
		var number int
		return fmt.Fscanf(nil, "", &number)
	})
	check("newline needs read", func() (int, error) { return fmt.Fscanln(nil) })
	check("literal needs read", func() (int, error) { return fmt.Fscanf(nil, "x") })
	check("operand needs read", func() (int, error) {
		var number int
		return fmt.Fscan(nil, &number)
	})
}

// Output:
// empty Fscan 0 <nil>
// empty Fscanf 0 <nil>
// custom no read 1 <nil>
// extra operand 0 too many operands
// newline needs read panic runtime error: invalid memory address or nil pointer dereference
// literal needs read panic runtime error: invalid memory address or nil pointer dereference
// operand needs read panic runtime error: invalid memory address or nil pointer dereference
