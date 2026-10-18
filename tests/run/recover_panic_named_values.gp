// phase: 05
// ExitCode: 2
// Stderr: ^panic: \(main\.T\) 0x[0-9a-f]+\n\tpanic: \(\*main\.T\) 0x[0-9a-f]+\n\tpanic: main\.MyS\("hi"\)\n\tpanic: main\.MyInt\(5\)\n\tpanic: main\.MyF\(\+2\.500000e\+000\)\n\tpanic: main\.MyB\(true\)\n\ngoroutine 1 \[running\]:\n

// Panic values of named types print with their type: main.MyInt(5), main.MyS("hi") (printanycustomtype);
// a struct or a pointer prints its type and an address.
package main

type T struct{ A, B int }
type MyInt int
type MyS string
type MyF float64
type MyB bool

func main() {
	defer func() { panic(MyB(true)) }()
	defer func() { panic(MyF(2.5)) }()
	defer func() { panic(MyInt(5)) }()
	defer func() { panic(MyS("hi")) }()
	defer func() { panic(&T{1, 2}) }()
	panic(T{1, 2})
}

// Output:
