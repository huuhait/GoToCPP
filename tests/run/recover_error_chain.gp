// phase: 05
// ExitCode: 2
// Stderr: ^panic: myerr 3 \[recovered\]\n\tpanic: again\n\ngoroutine 1 \[running\]:\n

package main

import "fmt"

type MyErr struct{ Code int }

func (e MyErr) Error() string { return fmt.Sprint("myerr ", e.Code) }

func main() {
	defer func() {
		r := recover()
		fmt.Println("recovered:", r.(error).Error())
		panic("again")
	}()
	panic(MyErr{3})
}

// Output:
// recovered: myerr 3
