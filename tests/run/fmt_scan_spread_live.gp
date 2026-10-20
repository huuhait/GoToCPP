// phase: 09

// A custom Scanner changes the next operand of a live ...any slice before it is scanned.
package main

import (
	"fmt"
	"os"
	"strings"
)

var arguments []any
var original int
var replacement int

type replaceNext struct{ drop bool }

func (scanner replaceNext) Scan(state fmt.ScanState, verb rune) error {
	arguments[1] = &replacement
	if scanner.drop {
		arguments = nil
	}
	return nil
}

func reset(drop bool) {
	original = 0
	replacement = 0
	arguments = []any{replaceNext{drop}, &original}
}

func report(name string, count int, err error) {
	fmt.Println(name, count, err, original, replacement)
}

func scanInput(name string, scan func(...any) (int, error)) {
	reader, writer, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	writer.WriteString("7\n")
	writer.Close()
	previous := os.Stdin
	os.Stdin = reader
	defer func() {
		os.Stdin = previous
		reader.Close()
	}()

	reset(false)
	count, err := scan(arguments...)
	report(name, count, err)
}

func main() {
	reset(false)
	count, err := fmt.Sscan("7", arguments...)
	report("Sscan", count, err)

	reset(false)
	count, err = fmt.Sscanln("7\n", arguments...)
	report("Sscanln", count, err)

	reset(false)
	count, err = fmt.Sscanf("7", "%v%v", arguments...)
	report("Sscanf", count, err)

	reset(false)
	count, err = fmt.Fscan(strings.NewReader("7"), arguments...)
	report("Fscan", count, err)

	reset(false)
	count, err = fmt.Fscanln(strings.NewReader("7\n"), arguments...)
	report("Fscanln", count, err)

	reset(false)
	count, err = fmt.Fscanf(strings.NewReader("7"), "%v%v", arguments...)
	report("Fscanf", count, err)

	scanInput("Scan", fmt.Scan)
	scanInput("Scanln", fmt.Scanln)
	scanInput("Scanf", func(values ...any) (int, error) {
		return fmt.Scanf("%v%v", values...)
	})

	reset(true)
	count, err = fmt.Sscan("7", arguments...)
	report("dropped slice", count, err)
}

// Output:
// Sscan 2 <nil> 0 7
// Sscanln 2 <nil> 0 7
// Sscanf 2 <nil> 0 7
// Fscan 2 <nil> 0 7
// Fscanln 2 <nil> 0 7
// Fscanf 2 <nil> 0 7
// Scan 2 <nil> 0 7
// Scanln 2 <nil> 0 7
// Scanf 2 <nil> 0 7
// dropped slice 2 <nil> 0 7
